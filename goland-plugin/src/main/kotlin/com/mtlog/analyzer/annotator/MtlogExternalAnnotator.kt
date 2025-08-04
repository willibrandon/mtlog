package com.mtlog.analyzer.annotator

import com.goide.psi.GoFile
import com.goide.psi.GoStringLiteral
import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.ExternalAnnotator
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.application.readAction
import com.intellij.openapi.components.service
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.editor.Document
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.util.TextRange
import com.intellij.openapi.vfs.VfsUtilCore
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiFile
import com.intellij.psi.util.parentOfType
import com.intellij.util.IncorrectOperationException
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.notification.MtlogNotificationService
import kotlinx.coroutines.*
import org.jetbrains.annotations.Blocking
import java.security.MessageDigest
import java.util.concurrent.ConcurrentHashMap
import kotlin.coroutines.cancellation.CancellationException

/**
 * Analysis input data.
 */
data class MtlogInfo(
    val file: PsiFile,
    val document: Document,
    val text: String,
    val goModPath: String
)

/**
 * Analysis results.
 */
data class MtlogResult(
    val diagnostics: List<MtlogDiagnostic>
)

/**
 * Editor diagnostic with highlighting info.
 */
data class MtlogDiagnostic(
    val range: TextRange,
    val message: String,
    val severity: DiagnosticSeverity,
    val propertyName: String? = null,
    val isTemplateError: Boolean = false,
    val diagnosticId: String? = null
)

/**
 * Diagnostic severity levels.
 */
enum class DiagnosticSeverity {
    ERROR, WARNING, SUGGESTION
}

/**
 * External annotator running mtlog-analyzer asynchronously.
 */
class MtlogExternalAnnotator : ExternalAnnotator<MtlogInfo, MtlogResult>() {
    companion object {
        private val LOG = logger<MtlogExternalAnnotator>()
        private val fileHashCache = ConcurrentHashMap<String, String>()
        private val resultCache = ConcurrentHashMap<String, CachedResult>()
        private const val CACHE_TTL_MS = 60_000L // 1 minute
        
        data class CachedResult(
            val hash: String,
            val result: MtlogResult,
            val timestamp: Long
        )
        
        /**
         * Clears the result cache. Call this when settings change.
         */
        fun clearCache() {
            resultCache.clear()
        }
        
        private fun getFileHash(text: String): String {
            val digest = MessageDigest.getInstance("SHA-256")
            val hashBytes = digest.digest(text.toByteArray())
            return hashBytes.joinToString("") { "%02x".format(it) }
        }
        
        private fun getDebounceDelay(fileSize: Int): Long {
            val fileSizeMb = fileSize / (1024 * 1024)
            return when {
                fileSizeMb > 1 -> 600L
                else -> 300L // Default from advanced settings
            }
        }
    }
    
    override fun collectInformation(file: PsiFile): MtlogInfo? {
        LOG.debug("MtlogExternalAnnotator.collectInformation called for file: ${file.name}")
        
        if (file !is GoFile) {
            LOG.debug("File is not a GoFile: ${file.javaClass.name}")
            return null
        }
        
        val project = file.project
        val service = project.service<MtlogProjectService>()
        if (!service.state.enabled) {
            LOG.debug("Mtlog analyzer is disabled")
            return null
        }
        
        val document = PsiDocumentManager.getInstance(project).getDocument(file) ?: return null
        val text = document.text
        
        // Find go.mod path
        val goModPath = findGoModPath(file)
        if (goModPath == null) {
            LOG.debug("Could not find go.mod for file: ${file.name}")
            return null
        }
        
        LOG.debug("Collected info for file: ${file.name}, go.mod at: $goModPath")
        return MtlogInfo(file, document, text, goModPath)
    }
    
    @Blocking
    override fun doAnnotate(info: MtlogInfo): MtlogResult? {
        LOG.debug("MtlogExternalAnnotator.doAnnotate called for file: ${info.file.name}")
        
        val fileHash = getFileHash(info.text)
        val filePath = info.file.virtualFile.path
        
        // Check cache
        val cached = resultCache[filePath]
        if (cached != null && 
            cached.hash == fileHash && 
            System.currentTimeMillis() - cached.timestamp < CACHE_TTL_MS) {
            return cached.result
        }
        
        return try {
            // doAnnotate already runs on a background thread, no need for runBlocking
            val result = analyzeFileBlocking(info)
            if (result != null) {
                // Cache the result
                resultCache[filePath] = CachedResult(fileHash, result, System.currentTimeMillis())
            }
            result
        } catch (e: Exception) {
            LOG.error("Failed to analyze ${info.file.name}", e)
            null
        }
    }
    
    override fun apply(file: PsiFile, result: MtlogResult?, holder: AnnotationHolder) {
        LOG.debug("MtlogExternalAnnotator.apply called for file: ${file.name}, result is null: ${result == null}")
        if (result == null) return
        
        LOG.debug("Applying ${result.diagnostics.size} diagnostics to file: ${file.name}")
        
        val project = file.project
        val service = project.service<MtlogProjectService>()
        val state = service.state
        val document = PsiDocumentManager.getInstance(project).getDocument(file) ?: return
        
        // Removed - notification is now only shown on startup via MtlogStartupActivity
        
        for (diagnostic in result.diagnostics) {
            val severity = when (diagnostic.severity) {
                DiagnosticSeverity.ERROR -> getSeverity(state.errorSeverity ?: "ERROR")
                DiagnosticSeverity.WARNING -> getSeverity(state.warningSeverity ?: "WARNING")
                DiagnosticSeverity.SUGGESTION -> getSeverity(state.suggestionSeverity ?: "WEAK_WARNING")
            }
            
            // Find the PSI element at the diagnostic range
            val leaf = file.findElementAt(diagnostic.range.startOffset) ?: continue
            val literal = leaf.parentOfType<GoStringLiteral>() ?: leaf  // fallback
            val anchor = literal  // underline sticks to literal
            
            val builder = holder.newAnnotation(severity, diagnostic.message)
                .range(anchor)  // Anchor to whole literal, not leaf
                .needsUpdateOnTyping(true)
            
            // Add quick fixes based on diagnostic type
            when {
                diagnostic.message.contains("PascalCase") && diagnostic.propertyName != null -> {
                    builder.withFix(com.mtlog.analyzer.quickfix.PascalCaseQuickFix(anchor, diagnostic.propertyName))
                }
                diagnostic.message.contains("arguments") -> {
                    builder.withFix(com.mtlog.analyzer.quickfix.TemplateArgumentQuickFix(anchor))
                }
            }
            
            // Add suppression quick fix if diagnostic ID is available
            if (diagnostic.diagnosticId != null) {
                builder.withFix(com.mtlog.analyzer.quickfix.SuppressDiagnosticQuickFix(diagnostic.diagnosticId))
            }
            
            builder.create()
            
            // For template errors, also highlight the arguments
            if (diagnostic.isTemplateError) {
                try {
                    val lineNumber = document.getLineNumber(diagnostic.range.startOffset)
                    val lineStart = document.getLineStartOffset(lineNumber)
                    val lineEnd = document.getLineEndOffset(lineNumber)
                    val lineText = document.text.substring(lineStart, lineEnd)
                    
                    // Find the arguments after the template string
                    val templateEnd = lineText.lastIndexOf("\"")
                    if (templateEnd != -1) {
                        // Look for args after the template - mtlog uses positional args, not named
                        val argsPattern = Regex(",\\s*([^,)]+)")
                        val matchResult = argsPattern.find(lineText, templateEnd)
                        if (matchResult != null) {
                            val argsStart = lineStart + matchResult.groups[1]!!.range.first
                            val argsEnd = lineStart + matchResult.groups[1]!!.range.last + 1
                            
                            holder.newAnnotation(severity, "Incorrect number of arguments")
                                .range(TextRange(argsStart, argsEnd))
                                .needsUpdateOnTyping(true)
                                .create()
                        }
                    }
                } catch (e: Exception) {
                    LOG.warn("Failed to highlight arguments", e)
                }
            }
        }
    }
    
    private fun analyzeFileBlocking(info: MtlogInfo): MtlogResult? {
        val project = info.file.project
        val service = project.service<MtlogProjectService>()
        
        LOG.debug("Running mtlog-analyzer on file: ${info.file.virtualFile.path}")
        
        // Convert to a real file path for the external analyzer
        val ioFile = VfsUtilCore.virtualToIoFile(info.file.virtualFile)
            ?: throw IllegalStateException("Not a real file: ${info.file.virtualFile.path}")
        
        val result = service.runAnalyzer(ioFile.absolutePath, info.goModPath)
        if (result == null) {
            LOG.warn("Analyzer returned no result for ${info.file.name}")
            return null
        }
        
        LOG.debug("Analyzer returned ${result.size} diagnostics")
        
        val diagnostics = result.mapNotNull { diagnostic ->
            // Convert line/column to text offset
            val lineStartOffset = try {
                info.document.getLineStartOffset(diagnostic.lineNumber - 1)
            } catch (e: Exception) {
                LOG.warn("Invalid line number ${diagnostic.lineNumber} for file ${info.file.name}")
                return@mapNotNull null
            }
            
            val lineEndOffset = try {
                info.document.getLineEndOffset(diagnostic.lineNumber - 1)
            } catch (e: Exception) {
                LOG.warn("Invalid line number ${diagnostic.lineNumber} for file ${info.file.name}")
                return@mapNotNull null
            }
            
            // Get the line text to find the property name
            val lineText = info.text.substring(lineStartOffset, lineEndOffset)
            
            // Determine what to highlight based on the diagnostic type
            val (startOffset, endOffset) = when {
                // PascalCase warnings - highlight just the property name inside braces
                diagnostic.message.contains("PascalCase") && diagnostic.propertyName != null -> {
                    val propertyIndex = lineText.indexOf("{${diagnostic.propertyName}}")
                    if (propertyIndex != -1) {
                        // Found the property - highlight just the property name inside the braces
                        val start = lineStartOffset + propertyIndex + 1 // skip the opening brace
                        val end = start + diagnostic.propertyName.length
                        start to end
                    } else {
                        // Fallback to column-based calculation
                        val start = lineStartOffset + (diagnostic.columnNumber - 1)
                        val end = start + diagnostic.propertyName.length
                        start to end
                    }
                }
                
                // Template/argument mismatch - highlight the entire message template string
                diagnostic.message.contains("arguments") || diagnostic.message.contains("properties") -> {
                    // Find the message template string literal
                    val templateStart = lineText.indexOf("\"")
                    val templateEnd = lineText.lastIndexOf("\"")
                    if (templateStart != -1 && templateEnd != -1 && templateStart < templateEnd) {
                        // Highlight from opening quote to closing quote (inclusive of quotes)
                        val start = lineStartOffset + templateStart
                        val end = lineStartOffset + templateEnd + 1
                        start to end
                    } else {
                        // Fallback
                        val start = lineStartOffset + (diagnostic.columnNumber - 1)
                        val end = minOf(start + 40, lineEndOffset)
                        start to end
                    }
                }
                
                // Default case
                else -> {
                    val start = lineStartOffset + (diagnostic.columnNumber - 1)
                    val end = minOf(start + 20, lineEndOffset)
                    start to end
                }
            }
            
            // Ensure range is within document bounds
            val docLength = info.document.textLength
            if (startOffset >= docLength || endOffset > docLength) {
                LOG.warn("Invalid offset $startOffset-$endOffset for document length $docLength")
                return@mapNotNull null
            }
            
            // Extract diagnostic ID from message
            val diagnosticId = extractDiagnosticId(diagnostic.message)
            
            MtlogDiagnostic(
                range = TextRange(startOffset, endOffset),
                message = diagnostic.message,
                severity = when (diagnostic.severity) {
                    "error" -> DiagnosticSeverity.ERROR
                    "warning" -> DiagnosticSeverity.WARNING
                    else -> DiagnosticSeverity.SUGGESTION
                },
                propertyName = diagnostic.propertyName,
                isTemplateError = diagnostic.message.contains("arguments") || diagnostic.message.contains("properties"),
                diagnosticId = diagnosticId
            )
        }
        
        return if (diagnostics.isNotEmpty()) {
            MtlogResult(diagnostics)
        } else {
            null
        }
    }
    
    private fun findGoModPath(file: PsiFile): String? {
        var dir = file.virtualFile.parent
        while (dir != null) {
            if (dir.findChild("go.mod") != null) {
                return dir.path
            }
            dir = dir.parent
        }
        return null
    }
    
    private fun getSeverity(severityName: String): HighlightSeverity {
        return when (severityName) {
            "ERROR" -> HighlightSeverity.ERROR
            "WARNING" -> HighlightSeverity.WARNING
            "WEAK_WARNING" -> HighlightSeverity.WEAK_WARNING
            "INFO" -> HighlightSeverity.INFORMATION
            else -> HighlightSeverity.WARNING
        }
    }
    
    private fun extractDiagnosticId(message: String): String? {
        // First try to extract from [MTLOG00X] format
        val idMatch = Regex("\\[(MTLOG\\d{3})\\]").find(message)
        if (idMatch != null) {
            return idMatch.groupValues[1]
        }
        
        // Otherwise, determine from message content
        val msgLower = message.lowercase()
        return when {
            msgLower.contains("template has") && msgLower.contains("properties") && msgLower.contains("arguments") -> "MTLOG001"
            msgLower.contains("invalid format specifier") -> "MTLOG002"
            msgLower.contains("duplicate property") -> "MTLOG003"
            msgLower.contains("pascalcase") -> "MTLOG004"
            msgLower.contains("capturing") || msgLower.contains("@ prefix") || msgLower.contains("$ prefix") -> "MTLOG005"
            msgLower.contains("error level log without error") || msgLower.contains("error logging without error") -> "MTLOG006"
            msgLower.contains("context key") -> "MTLOG007"
            msgLower.contains("dynamic template") -> "MTLOG008"
            else -> null
        }
    }
}
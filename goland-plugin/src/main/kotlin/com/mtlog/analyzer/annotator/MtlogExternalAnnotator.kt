package com.mtlog.analyzer.annotator

import com.goide.psi.GoFile
import com.goide.psi.GoStringLiteral
import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.ExternalAnnotator
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.application.readAction
import com.intellij.openapi.components.service
import com.intellij.openapi.editor.Document
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.util.TextRange
import com.intellij.openapi.util.Key
import com.intellij.openapi.vfs.VfsUtilCore
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiFile
import com.intellij.psi.util.parentOfType
import com.intellij.util.IncorrectOperationException
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.notification.MtlogNotificationService
import com.mtlog.analyzer.logging.MtlogLogger
import kotlinx.coroutines.*
import org.jetbrains.annotations.Blocking
import kotlin.coroutines.cancellation.CancellationException

/**
 * Analysis input data.
 */
class MtlogInfo(
    val file: PsiFile,
    val document: Document,
    val text: String,
    val goModPath: String,
    val timestamp: Long = System.currentTimeMillis()
) {
    // Don't use data class to avoid automatic equals/hashCode
    override fun equals(other: Any?): Boolean = false
    override fun hashCode(): Int = System.identityHashCode(this)
}

/**
 * Analysis results.
 */
class MtlogResult(
    val diagnostics: List<MtlogDiagnostic>,
    val timestamp: Long = System.currentTimeMillis(),
    val nanoTime: Long = System.nanoTime()  // Additional uniqueness
) {
    // Don't use data class to avoid automatic equals/hashCode
    // This ensures IntelliJ always treats results as "new"
    // Added timestamp and nanoTime to make each result unique
    
    override fun equals(other: Any?): Boolean = false
    override fun hashCode(): Int = System.identityHashCode(this)
}

/**
 * Editor diagnostic with highlighting info.
 */
class MtlogDiagnostic(
    val range: TextRange,
    val message: String,
    val severity: DiagnosticSeverity,
    val propertyName: String? = null,
    val isTemplateError: Boolean = false,
    val diagnosticId: String? = null,
    val uniqueId: Long = System.nanoTime()
) {
    // Not a data class to prevent caching based on equals/hashCode
    override fun equals(other: Any?): Boolean = false
    override fun hashCode(): Int = System.identityHashCode(this)
}

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
        private val FORCE_REFRESH_KEY = Key.create<Long>("MTLOG_FORCE_REFRESH")
        
        private fun getDebounceDelay(fileSize: Int): Long {
            val fileSizeMb = fileSize / (1024 * 1024)
            return when {
                fileSizeMb > 1 -> 600L
                else -> 300L // Default from advanced settings
            }
        }
    }
    
    override fun collectInformation(file: PsiFile): MtlogInfo? {
        MtlogLogger.info("collectInformation START for file: ${file.name}", file.project)
        MtlogLogger.debug("MtlogExternalAnnotator.collectInformation called for file: ${file.name}", file.project)
        MtlogLogger.info("File virtualFile.modificationStamp: ${file.virtualFile?.modificationStamp}", file.project)
        
        // Force a new PSI view to ensure apply() is called
        file.putUserData(FORCE_REFRESH_KEY, System.currentTimeMillis())
        
        if (file !is GoFile) {
            MtlogLogger.debug("File is not a GoFile: ${file.javaClass.name}", file.project)
            return null
        }
        
        val project = file.project
        val service = project.service<MtlogProjectService>()
        if (!service.state.enabled) {
            MtlogLogger.debug("Mtlog analyzer is disabled", project)
            return null
        }
        
        val document = PsiDocumentManager.getInstance(project).getDocument(file) ?: return null
        val text = document.text
        
        // Find go.mod path
        val goModPath = findGoModPath(file)
        if (goModPath == null) {
            MtlogLogger.debug("Could not find go.mod for file: ${file.name}", project)
            return null
        }
        
        MtlogLogger.debug("Collected info for file: ${file.name}, go.mod at: $goModPath", project)
        val info = MtlogInfo(file, document, text, goModPath)
        MtlogLogger.info("collectInformation returning MtlogInfo (non-null)", project)
        return info
    }
    
    @Blocking
    override fun doAnnotate(info: MtlogInfo): MtlogResult? {
        MtlogLogger.debug("MtlogExternalAnnotator.doAnnotate called for file: ${info.file.name}", info.file.project)
        MtlogLogger.info("doAnnotate: file=${info.file.name}, text length=${info.text.length}", info.file.project)
        
        return try {
            // doAnnotate already runs on a background thread, no need for runBlocking
            MtlogLogger.info("Running analyzeFileBlocking...", info.file.project)
            val result = analyzeFileBlocking(info)
            MtlogLogger.info("analyzeFileBlocking returned: ${result?.diagnostics?.size ?: "null"} diagnostics", info.file.project)
            MtlogLogger.info("doAnnotate returning ${result?.diagnostics?.size ?: "null"} diagnostics", info.file.project)
            
            // Store result as pending annotations in case apply() isn't called
            if (result != null && info.file is GoFile) {
                MtlogForceAnnotator.storePendingAnnotations(info.file, result)
            }
            
            result
        } catch (e: Exception) {
            MtlogLogger.error("Failed to analyze ${info.file.name}", info.file.project, e)
            null
        }
    }
    
    override fun apply(file: PsiFile, result: MtlogResult?, holder: AnnotationHolder) {
        MtlogLogger.info("=== MtlogExternalAnnotator.apply START for file: ${file.name} ===", file.project)
        MtlogLogger.info("Result is null: ${result == null}", file.project)
        MtlogLogger.info("File text length: ${file.textLength}", file.project)
        MtlogLogger.info("File modificationStamp: ${file.modificationStamp}", file.project)
        MtlogLogger.info("File virtualFile.modificationStamp: ${file.virtualFile?.modificationStamp}", file.project)
        MtlogLogger.info("PSI userdata FORCE_REFRESH: ${file.getUserData(FORCE_REFRESH_KEY)}", file.project)
        
        // Clear the force refresh key
        file.putUserData(FORCE_REFRESH_KEY, null)
        
        // Clear pending annotations since apply() was called
        if (file is GoFile) {
            file.putUserData(MtlogForceAnnotator.PENDING_ANNOTATIONS_KEY, null)
            MtlogLogger.info("Cleared pending annotations since apply() was called", file.project)
        }
        
        if (result == null) {
            MtlogLogger.warn("Result is null, returning early", file.project)
            return
        }
        
        MtlogLogger.info("Applying ${result.diagnostics.size} diagnostics to file: ${file.name}", file.project)
        
        // Log all diagnostics with their ranges
        result.diagnostics.forEachIndexed { index, diag ->
            MtlogLogger.info("Diagnostic $index: range=${diag.range}, message='${diag.message}'", file.project)
        }
        
        val project = file.project
        val service = project.service<MtlogProjectService>()
        val state = service.state
        val document = PsiDocumentManager.getInstance(project).getDocument(file) ?: return
        
        // Group diagnostics by range to avoid duplicate quick fixes
        val diagnosticsByRange = result.diagnostics.groupBy { it.range }
        
        for ((range, diagnosticsAtRange) in diagnosticsByRange) {
            // Use the first diagnostic for the main annotation
            val diagnostic = diagnosticsAtRange.first()
            val severity = when (diagnostic.severity) {
                DiagnosticSeverity.ERROR -> getSeverity(state.errorSeverity ?: "ERROR")
                DiagnosticSeverity.WARNING -> getSeverity(state.warningSeverity ?: "WARNING")
                DiagnosticSeverity.SUGGESTION -> getSeverity(state.suggestionSeverity ?: "WEAK_WARNING")
            }
            
            // Find the PSI element at the diagnostic range
            MtlogLogger.info("Looking for PSI element at offset ${diagnostic.range.startOffset} (range: ${diagnostic.range})", project)
            MtlogLogger.info("Diagnostic message: ${diagnostic.message}", project)
            val leaf = file.findElementAt(diagnostic.range.startOffset)
            if (leaf == null) {
                MtlogLogger.warn("No PSI element found at offset ${diagnostic.range.startOffset}, file length: ${file.textLength}", project)
                // Log some context around the offset
                val start = maxOf(0, diagnostic.range.startOffset - 20)
                val end = minOf(file.textLength, diagnostic.range.startOffset + 20)
                if (start < file.textLength) {
                    MtlogLogger.warn("Context around offset: '${file.text.substring(start, end)}'", project)
                }
                continue
            }
            MtlogLogger.info("Found leaf element: ${leaf.javaClass.simpleName}, text: '${leaf.text}'", project)
            
            // For better highlighting, try to find the containing call expression
            val anchor = when {
                // For MTLOG006 specifically, highlight the whole call expression
                diagnostic.diagnosticId == "MTLOG006" -> {
                    val callExpr = leaf.parentOfType<com.goide.psi.GoCallExpr>()
                    val stringLiteral = leaf.parentOfType<GoStringLiteral>()
                    MtlogLogger.info("MTLOG006: callExpr=${callExpr?.javaClass?.simpleName}, stringLiteral=${stringLiteral?.javaClass?.simpleName}", project)
                    callExpr ?: stringLiteral ?: leaf
                }
                // For other error-level diagnostics, also highlight the whole call
                diagnostic.severity == DiagnosticSeverity.ERROR -> {
                    leaf.parentOfType<com.goide.psi.GoCallExpr>() ?: leaf.parentOfType<GoStringLiteral>() ?: leaf
                }
                // For other diagnostics, highlight just the string literal
                else -> {
                    leaf.parentOfType<GoStringLiteral>() ?: leaf
                }
            }
            
            // Combine messages if there are multiple diagnostics at the same location
            val combinedMessage = if (diagnosticsAtRange.size > 1) {
                diagnosticsAtRange.joinToString("; ") { it.message }
            } else {
                diagnostic.message
            }
            
            MtlogLogger.debug("Creating annotation for anchor: ${anchor.javaClass.simpleName}, text: '${anchor.text.take(50)}...', range: ${anchor.textRange}", project)
            
            val builder = holder.newAnnotation(severity, combinedMessage)
                .range(anchor)  // Anchor to whole literal, not leaf
                .needsUpdateOnTyping(true)
            
            // Add quick fixes from all diagnostics at this range, but avoid duplicates
            val addedFixes = mutableSetOf<String>()
            
            for (diag in diagnosticsAtRange) {
                MtlogLogger.debug("Processing diagnostic for quick fixes: ${diag.message}, diagnosticId: ${diag.diagnosticId}", project)
                when {
                    diag.message.contains("PascalCase") && diag.propertyName != null -> {
                        val fixKey = "PascalCase:${diag.propertyName}"
                        if (!addedFixes.contains(fixKey)) {
                            MtlogLogger.debug("Adding PascalCase quick fix for property: ${diag.propertyName}", project)
                            builder.withFix(com.mtlog.analyzer.quickfix.PascalCaseQuickFix(anchor, diag.propertyName))
                            addedFixes.add(fixKey)
                        }
                    }
                    diag.message.contains("arguments") -> {
                        val fixKey = "TemplateArgument"
                        if (!addedFixes.contains(fixKey)) {
                            MtlogLogger.debug("Adding TemplateArgument quick fix", project)
                            builder.withFix(com.mtlog.analyzer.quickfix.TemplateArgumentQuickFix(anchor))
                            addedFixes.add(fixKey)
                        }
                    }
                    diag.diagnosticId == "MTLOG006" || (diag.message.contains("error") && diag.message.contains("without error")) -> {
                        val fixKey = "MissingError"
                        if (!addedFixes.contains(fixKey)) {
                            MtlogLogger.info("ATTEMPTING to add MissingError quick fix for message: ${diag.message}, diagnosticId: ${diag.diagnosticId}", project)
                            MtlogLogger.info("Anchor element: ${anchor.javaClass.simpleName}, text: '${anchor.text.take(30)}'", project)
                            try {
                                val quickFix = com.mtlog.analyzer.quickfix.MissingErrorQuickFix(anchor)
                                MtlogLogger.info("Quick fix created successfully", project)
                                builder.withFix(quickFix)
                                MtlogLogger.info("Quick fix added to builder", project)
                                addedFixes.add(fixKey)
                            } catch (e: Exception) {
                                MtlogLogger.error("Failed to create/add quick fix", project, e)
                            }
                        }
                    }
                }
                
                // Add suppression quick fix if diagnostic ID is available
                if (diag.diagnosticId != null) {
                    val fixKey = "Suppress:${diag.diagnosticId}"
                    if (!addedFixes.contains(fixKey)) {
                        builder.withFix(com.mtlog.analyzer.quickfix.SuppressDiagnosticQuickFix(diag.diagnosticId))
                        addedFixes.add(fixKey)
                    }
                }
            }
            
            MtlogLogger.info("Creating annotation with ${addedFixes.size} quick fixes: $addedFixes", project)
            val annotation = builder.create()
            MtlogLogger.info("Annotation created successfully", project)
        }
        
        MtlogLogger.info("=== MtlogExternalAnnotator.apply COMPLETED ===", project)
    }
    
    private fun analyzeFileBlocking(info: MtlogInfo): MtlogResult? {
        val project = info.file.project
        val service = project.service<MtlogProjectService>()
        
        MtlogLogger.debug("Running mtlog-analyzer on file: ${info.file.virtualFile.path}", project)
        
        // Convert to a real file path for the external analyzer (still needed for filename)
        val ioFile = VfsUtilCore.virtualToIoFile(info.file.virtualFile)
            ?: throw IllegalStateException("Not a real file: ${info.file.virtualFile.path}")
        
        MtlogLogger.info("Analyzing file with stdin mode, editor content length: ${info.text.length}", project)
        
        // Pass editor content directly to analyzer via stdin mode
        val result = service.runAnalyzer(ioFile.absolutePath, info.goModPath, info.text)
        MtlogLogger.info("runAnalyzer returned: ${result?.size ?: "null"} raw diagnostics", project)
        if (result == null) {
            MtlogLogger.warn("Analyzer returned no result for ${info.file.name}", project)
            return null
        }
        
        MtlogLogger.info("Analyzer returned ${result.size} diagnostics", project)
        
        val diagnostics = result.mapNotNull { diagnostic ->
            // Convert line/column to text offset
            MtlogLogger.debug("Converting diagnostic at line ${diagnostic.lineNumber}, col ${diagnostic.columnNumber}", project)
            val lineStartOffset = try {
                info.document.getLineStartOffset(diagnostic.lineNumber - 1)
            } catch (e: Exception) {
                MtlogLogger.warn("Invalid line number ${diagnostic.lineNumber} for file ${info.file.name}", project)
                return@mapNotNull null
            }
            
            val lineEndOffset = try {
                info.document.getLineEndOffset(diagnostic.lineNumber - 1)
            } catch (e: Exception) {
                MtlogLogger.warn("Invalid line number ${diagnostic.lineNumber} for file ${info.file.name}", project)
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
                MtlogLogger.warn("Invalid offset $startOffset-$endOffset for document length $docLength", project)
                return@mapNotNull null
            }
            
            // Extract diagnostic ID from message
            val diagnosticId = extractDiagnosticId(diagnostic.message)
            
            val textRange = TextRange(startOffset, endOffset)
            MtlogLogger.debug("Created TextRange($startOffset, $endOffset) for diagnostic: ${diagnostic.message.take(50)}...", project)
            
            MtlogDiagnostic(
                range = textRange,
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
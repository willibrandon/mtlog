package com.mtlog.analyzer.annotator

import com.goide.psi.GoFile
import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.Annotator
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.components.service
import com.intellij.openapi.util.Key
import com.intellij.psi.PsiElement
import com.mtlog.analyzer.logging.MtlogLogger
import com.mtlog.analyzer.service.MtlogProjectService

/**
 * Force annotator that applies cached results when ExternalAnnotator fails to call apply()
 */
class MtlogForceAnnotator : Annotator {
    
    companion object {
        val PENDING_ANNOTATIONS_KEY = Key.create<MtlogResult>("MTLOG_PENDING_ANNOTATIONS")
        
        fun storePendingAnnotations(file: GoFile, result: MtlogResult) {
            file.putUserData(PENDING_ANNOTATIONS_KEY, result)
            MtlogLogger.info("Stored pending annotations for ${file.name}: ${result.diagnostics.size} diagnostics", file.project)
        }
    }
    
    override fun annotate(element: PsiElement, holder: AnnotationHolder) {
        // Only process GoFile root elements
        if (element !is GoFile) return
        
        val project = element.project
        val service = project.service<MtlogProjectService>()
        if (!service.state.enabled) return
        
        // Check for pending annotations
        val pendingResult = element.getUserData(PENDING_ANNOTATIONS_KEY)
        if (pendingResult != null) {
            MtlogLogger.info("=== MtlogForceAnnotator: Applying ${pendingResult.diagnostics.size} pending annotations for ${element.name} ===", project)
            
            // Clear the pending annotations
            element.putUserData(PENDING_ANNOTATIONS_KEY, null)
            
            // Apply the annotations
            val state = service.state
            
            for (diagnostic in pendingResult.diagnostics) {
                val severity = when (diagnostic.severity) {
                    DiagnosticSeverity.ERROR -> getSeverity(state.errorSeverity ?: "ERROR")
                    DiagnosticSeverity.WARNING -> getSeverity(state.warningSeverity ?: "WARNING")
                    DiagnosticSeverity.SUGGESTION -> getSeverity(state.suggestionSeverity ?: "WEAK_WARNING")
                }
                
                // Find the PSI element at the diagnostic range
                val leaf = element.findElementAt(diagnostic.range.startOffset)
                if (leaf == null) {
                    MtlogLogger.warn("Force annotator: No PSI element found at offset ${diagnostic.range.startOffset}", project)
                    continue
                }
                
                MtlogLogger.info("Force annotator: Creating annotation at range ${diagnostic.range}", project)
                
                val builder = holder.newAnnotation(severity, diagnostic.message)
                    .range(diagnostic.range)
                    .needsUpdateOnTyping(true)
                
                // Add quick fixes from analyzer-provided suggested fixes
                if (diagnostic.suggestedFixes.isNotEmpty()) {
                    for (suggestedFix in diagnostic.suggestedFixes) {
                        builder.withFix(com.mtlog.analyzer.quickfix.AnalyzerSuggestedQuickFix(leaf, suggestedFix))
                    }
                }
                
                // Add suppression quick fix if diagnostic ID is available
                if (diagnostic.diagnosticId != null) {
                    builder.withFix(com.mtlog.analyzer.quickfix.SuppressDiagnosticQuickFix(diagnostic.diagnosticId))
                }
                
                builder.create()
            }
            
            MtlogLogger.info("=== MtlogForceAnnotator: Completed applying pending annotations ===", project)
        }
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
}
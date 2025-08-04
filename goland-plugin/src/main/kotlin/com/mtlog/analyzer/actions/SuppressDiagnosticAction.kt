package com.mtlog.analyzer.actions

import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.components.service
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.ui.Messages
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.annotator.MtlogExternalAnnotator

/**
 * Action to suppress a specific mtlog diagnostic.
 */
class SuppressDiagnosticAction(
    private val diagnosticId: String,
    private val diagnosticDescription: String
) : AnAction() {
    
    companion object {
        private val LOG = logger<SuppressDiagnosticAction>()
        
        val DIAGNOSTIC_DESCRIPTIONS = mapOf(
            "MTLOG001" to "Template/argument count mismatch",
            "MTLOG002" to "Invalid format specifier",
            "MTLOG003" to "Duplicate property names",
            "MTLOG004" to "Property naming (PascalCase)",
            "MTLOG005" to "Missing capturing hints",
            "MTLOG006" to "Error logging without error value",
            "MTLOG007" to "Context key constant suggestion",
            "MTLOG008" to "Dynamic template warning"
        )
    }
    
    init {
        templatePresentation.text = "Suppress $diagnosticId diagnostic"
        templatePresentation.description = "Suppress $diagnosticDescription diagnostics project-wide"
    }
    
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Add diagnostic to suppressed list
        val suppressed = state.suppressedDiagnostics.toMutableList()
        if (!suppressed.contains(diagnosticId)) {
            suppressed.add(diagnosticId)
            state.suppressedDiagnostics = suppressed
            
            LOG.info("Suppressed diagnostic: $diagnosticId")
            
            // Clear both caches to ensure fresh analysis
            service.clearCache()
            MtlogExternalAnnotator.clearCache()
            
            // Force immediate re-analysis of all open files
            DaemonCodeAnalyzer.getInstance(project).restart()
        }
    }
    
    override fun update(e: AnActionEvent) {
        val project = e.project ?: return
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Disable if already suppressed
        e.presentation.isEnabled = !state.suppressedDiagnostics.contains(diagnosticId)
    }
}
package com.mtlog.analyzer.quickfix

import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.codeInsight.intention.IntentionAction
import com.intellij.openapi.components.service
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiFile
import com.mtlog.analyzer.actions.SuppressDiagnosticAction
import com.mtlog.analyzer.service.MtlogProjectService

/**
 * Quick fix to suppress a specific diagnostic type.
 */
class SuppressDiagnosticQuickFix(private val diagnosticId: String) : IntentionAction {
    
    companion object {
        private val LOG = logger<SuppressDiagnosticQuickFix>()
    }
    
    override fun getText(): String {
        val description = SuppressDiagnosticAction.DIAGNOSTIC_DESCRIPTIONS[diagnosticId] 
            ?: "this diagnostic"
        return "Suppress $diagnosticId ($description)"
    }
    
    override fun getFamilyName(): String = "Suppress mtlog diagnostic"
    
    override fun isAvailable(project: Project, editor: Editor?, file: PsiFile?): Boolean {
        if (project.isDisposed) return false
        
        val service = project.service<MtlogProjectService>()
        // Only available if not already suppressed
        return !service.state.suppressedDiagnostics.contains(diagnosticId)
    }
    
    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Add diagnostic to suppressed list
        val suppressed = state.suppressedDiagnostics.toMutableList()
        if (!suppressed.contains(diagnosticId)) {
            suppressed.add(diagnosticId)
            state.suppressedDiagnostics = suppressed
            
            LOG.info("Suppressed diagnostic: $diagnosticId")
            
            // Clear both service cache and annotator cache
            service.clearCache()
            com.mtlog.analyzer.annotator.MtlogExternalAnnotator.clearCache()
            
            // Force immediate re-analysis of the current file and all open files
            DaemonCodeAnalyzer.getInstance(project).restart()
        }
    }
    
    override fun startInWriteAction(): Boolean = false
}
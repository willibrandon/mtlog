package com.mtlog.analyzer.actions

import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.components.service
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.DialogWrapper
import com.intellij.ui.components.JBCheckBox
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBScrollPane
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.annotator.MtlogExternalAnnotator
import java.awt.BorderLayout
import java.awt.Dimension
import javax.swing.*

/**
 * Action to manage suppressed mtlog diagnostics.
 */
class ManageSuppressedDiagnosticsAction : AnAction("Manage Suppressed mtlog Diagnostics") {
    
    companion object {
        private val LOG = logger<ManageSuppressedDiagnosticsAction>()
    }
    
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val dialog = SuppressionManagerDialog(project)
        
        if (dialog.showAndGet()) {
            // Re-analyze all files
            DaemonCodeAnalyzer.getInstance(project).restart()
        }
    }
}

/**
 * Dialog for managing suppressed diagnostics.
 */
open class SuppressionManagerDialog(private val project: Project) : DialogWrapper(project) {
    
    private val service = project.service<MtlogProjectService>()
    protected val checkboxes = mutableMapOf<String, JBCheckBox>()
    
    init {
        title = "Manage Suppressed mtlog Diagnostics"
        init()
    }
    
    public override fun createCenterPanel(): JComponent {
        val panel = JPanel(BorderLayout())
        
        val suppressedList: MutableList<String> = service.state.suppressedDiagnostics
        
        if (suppressedList.isEmpty()) {
            panel.add(JBLabel("No diagnostics are currently suppressed"), BorderLayout.CENTER)
            return panel
        }
        
        val listPanel = JPanel()
        listPanel.layout = BoxLayout(listPanel, BoxLayout.Y_AXIS)
        
        // Create checkboxes for each suppressed diagnostic
        for (diagnosticId in suppressedList) {
            val description = SuppressDiagnosticAction.DIAGNOSTIC_DESCRIPTIONS[diagnosticId] 
                ?: "Unknown diagnostic"
            val checkbox = JBCheckBox("$diagnosticId - $description", true)
            checkboxes[diagnosticId] = checkbox
            listPanel.add(checkbox)
        }
        
        val scrollPane = JBScrollPane(listPanel)
        scrollPane.preferredSize = Dimension(400, 300)
        panel.add(scrollPane, BorderLayout.CENTER)
        
        // Add help text
        val helpText = JBLabel("Unchecked diagnostics will be unsuppressed")
        panel.add(helpText, BorderLayout.SOUTH)
        
        return panel
    }
    
    public override fun doOKAction() {
        // Update suppressed list based on checkbox states
        val newSuppressed = mutableListOf<String>()
        
        for ((diagnosticId, checkbox) in checkboxes) {
            if (checkbox.isSelected) {
                newSuppressed.add(diagnosticId)
            }
        }
        
        service.state.suppressedDiagnostics = newSuppressed
        service.restartProcesses()
        
        // Trigger re-analysis
        DaemonCodeAnalyzer.getInstance(project).restart()
        
        super.doOKAction()
    }
}
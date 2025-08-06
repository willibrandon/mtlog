package com.mtlog.analyzer.startup

import com.intellij.openapi.components.service
import com.intellij.openapi.project.Project
import com.intellij.openapi.startup.ProjectActivity
import com.intellij.openapi.application.invokeLater
import com.intellij.openapi.wm.ToolWindowManager
import com.mtlog.analyzer.logging.MtlogLogger
import com.mtlog.analyzer.notification.MtlogNotificationService
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.listener.MtlogDocumentListener
import com.intellij.openapi.editor.EditorFactory

/**
 * Startup activity that runs when a project is opened.
 */
class MtlogStartupActivity : ProjectActivity {
    
    override suspend fun execute(project: Project) {
        // Tool window will be shown on demand when user opens it
        
        // Register document listener to handle undo operations
        val documentListener = MtlogDocumentListener(project)
        EditorFactory.getInstance().eventMulticaster.addDocumentListener(documentListener, project)
        MtlogLogger.info("Registered MtlogDocumentListener", project)
        
        // Check if analyzer is available
        val service = project.service<MtlogProjectService>()
        if (service.state.enabled) {
            val analyzerPath = service.findAnalyzerPath()
            if (analyzerPath == null) {
                MtlogLogger.warn("mtlog-analyzer not found at startup", project)
                // Don't show notification at startup - wait until user tries to use it
            } else {
                MtlogLogger.info("mtlog-analyzer found at: $analyzerPath", project)
            }
        }
        
        // Log startup message
        MtlogLogger.info("mtlog-analyzer plugin started", project)
    }
}
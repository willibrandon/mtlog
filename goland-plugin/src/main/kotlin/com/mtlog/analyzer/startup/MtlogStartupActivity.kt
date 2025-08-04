package com.mtlog.analyzer.startup

import com.intellij.openapi.components.service
import com.intellij.openapi.project.Project
import com.intellij.openapi.startup.ProjectActivity
import com.mtlog.analyzer.notification.MtlogNotificationService
import com.mtlog.analyzer.service.MtlogProjectService

/**
 * Startup activity that runs when a project is opened.
 */
class MtlogStartupActivity : ProjectActivity {
    
    override suspend fun execute(project: Project) {
        // No longer show notification on startup - status bar widget is sufficient
        // Users can see the analyzer state in the status bar
    }
}
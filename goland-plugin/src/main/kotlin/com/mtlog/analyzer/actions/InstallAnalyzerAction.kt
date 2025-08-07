package com.mtlog.analyzer.actions

import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.components.service
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.ProgressManager
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import java.io.BufferedReader
import java.io.InputStreamReader

/**
 * Action to install mtlog-analyzer via go install.
 */
class InstallAnalyzerAction : AnAction() {
    
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        installAnalyzer(project)
    }
    
    companion object {
        fun installAnalyzer(project: Project) {
            ProgressManager.getInstance().run(object : Task.Backgroundable(
                project,
                "Installing mtlog-analyzer",
                true
            ) {
                override fun run(indicator: ProgressIndicator) {
                    indicator.text = "Running go install..."
                    indicator.isIndeterminate = true
                    
                    try {
                        val process = ProcessBuilder(
                            "go", "install", 
                            "github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest"
                        ).start()
                        
                        val reader = BufferedReader(InputStreamReader(process.inputStream))
                        val errorReader = BufferedReader(InputStreamReader(process.errorStream))
                        
                        val output = StringBuilder()
                        val error = StringBuilder()
                        
                        reader.lines().forEach { output.append(it).append("\n") }
                        errorReader.lines().forEach { error.append(it).append("\n") }
                        
                        val exitCode = process.waitFor()
                        
                        if (exitCode == 0) {
                            showNotification(
                                project,
                                MtlogBundle.message("notification.analyzer.installed"),
                                NotificationType.INFORMATION
                            )
                            
                            // Restart processes to pick up new analyzer
                            project.service<MtlogProjectService>().restartProcesses()
                        } else {
                            showNotification(
                                project,
                                MtlogBundle.message("notification.analyzer.install.failed", error.toString()),
                                NotificationType.ERROR
                            )
                        }
                    } catch (e: Exception) {
                        showNotification(
                            project,
                            MtlogBundle.message("notification.analyzer.install.failed", e.message ?: "Unknown error"),
                            NotificationType.ERROR
                        )
                    }
                }
            })
        }
        
        private fun showNotification(project: Project, content: String, type: NotificationType) {
            NotificationGroupManager.getInstance()
                .getNotificationGroup("mtlog.notifications")
                .createNotification(content, type)
                .notify(project)
        }
    }
}
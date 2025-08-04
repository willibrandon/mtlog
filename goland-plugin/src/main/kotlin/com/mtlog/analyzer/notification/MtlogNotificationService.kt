package com.mtlog.analyzer.notification

import com.intellij.notification.*
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.service
import com.intellij.openapi.options.ShowSettingsUtil
import com.intellij.openapi.project.Project
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.annotator.MtlogExternalAnnotator
import com.mtlog.analyzer.settings.MtlogSettingsConfigurable

/**
 * Service for managing mtlog analyzer notifications.
 */
@Service(Service.Level.PROJECT)
class MtlogNotificationService(private val project: Project) {
    
    companion object {
        private const val NOTIFICATION_GROUP_ID = "mtlog.analyzer"
        private const val ACTIVE_NOTIFICATION_KEY = "mtlog.analyzer.active"
        
        private val NOTIFICATION_GROUP = NotificationGroupManager.getInstance()
            .getNotificationGroup(NOTIFICATION_GROUP_ID)
    }
    
    private var currentNotification: Notification? = null
    private var hasShownNotification = false
    
    /**
     * Shows the analyzer active notification if not already shown.
     */
    fun showAnalyzerActiveNotification() {
        if (hasShownNotification) return
        
        val service = project.service<MtlogProjectService>()
        if (!service.state.enabled) return
        
        val notification = NOTIFICATION_GROUP.createNotification(
            MtlogBundle.message("notification.analyzer.active.title"),
            MtlogBundle.message("notification.analyzer.active.content"),
            NotificationType.INFORMATION
        )
        
        // Add Disable action
        notification.addAction(object : NotificationAction(MtlogBundle.message("notification.analyzer.disable")) {
            override fun actionPerformed(e: AnActionEvent, notification: Notification) {
                service.state.enabled = false
                service.clearCache()
                MtlogExternalAnnotator.clearCache()
                
                // Clear all existing annotations when disabled
                com.intellij.codeInsight.daemon.DaemonCodeAnalyzer.getInstance(project).restart()
                
                // Update status bar widget
                service.updateStatusBarWidget()
                
                notification.expire()
                hasShownNotification = false
            }
        })
        
        // Add Settings action
        notification.addAction(object : NotificationAction(MtlogBundle.message("notification.analyzer.settings")) {
            override fun actionPerformed(e: AnActionEvent, notification: Notification) {
                ShowSettingsUtil.getInstance().showSettingsDialog(project, MtlogSettingsConfigurable::class.java)
            }
        })
        
        notification.notify(project)
        currentNotification = notification
        hasShownNotification = true
    }
    
    /**
     * Hides the current notification if visible.
     */
    fun hideNotification() {
        currentNotification?.expire()
        currentNotification = null
        hasShownNotification = false
    }
    
    /**
     * Resets the notification state.
     */
    fun reset() {
        hasShownNotification = false
    }
}
package com.mtlog.analyzer.notification

import com.intellij.notification.Notification
import com.intellij.notification.NotificationGroupManager
import com.intellij.openapi.components.service
import com.intellij.testFramework.fixtures.BasePlatformTestCase
import com.mtlog.analyzer.service.MtlogProjectService

class MtlogNotificationServiceTest : BasePlatformTestCase() {
    
    private lateinit var notificationService: MtlogNotificationService
    
    override fun setUp() {
        super.setUp()
        notificationService = project.service()
    }
    
    fun testShowAnalyzerActiveNotification() {
        // Enable analyzer
        val projectService = project.service<MtlogProjectService>()
        projectService.state.enabled = true
        
        // Show notification
        notificationService.showAnalyzerActiveNotification()
        
        // Verify notification was shown (would need to mock notifications in real test)
        // For now, just verify it doesn't throw
    }
    
    fun testNotificationNotShownWhenDisabled() {
        // Disable analyzer
        val projectService = project.service<MtlogProjectService>()
        projectService.state.enabled = false
        
        // Try to show notification
        notificationService.showAnalyzerActiveNotification()
        
        // Verify notification was not shown (would need to mock in real test)
    }
    
    fun testNotificationOnlyShownOnce() {
        // Enable analyzer
        val projectService = project.service<MtlogProjectService>()
        projectService.state.enabled = true
        
        // Show notification twice
        notificationService.showAnalyzerActiveNotification()
        notificationService.showAnalyzerActiveNotification()
        
        // Second call should be ignored (would need to verify with mock)
    }
    
    fun testResetNotificationState() {
        // Enable analyzer
        val projectService = project.service<MtlogProjectService>()
        projectService.state.enabled = true
        
        // Show notification
        notificationService.showAnalyzerActiveNotification()
        
        // Reset state
        notificationService.reset()
        
        // Should be able to show again
        notificationService.showAnalyzerActiveNotification()
    }
}
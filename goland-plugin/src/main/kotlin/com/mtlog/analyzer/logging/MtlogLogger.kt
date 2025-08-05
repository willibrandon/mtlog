package com.mtlog.analyzer.logging

import com.intellij.execution.ui.ConsoleView
import com.intellij.execution.ui.ConsoleViewContentType
import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.components.service
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Key
import com.mtlog.analyzer.settings.MtlogSettingsState
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter

/**
 * Centralized logger for mtlog analyzer that outputs to multiple destinations:
 * - IntelliJ's diagnostic logger (idea.log)
 * - ConsoleView in tool window
 * - Event Log notifications for important messages
 */
object MtlogLogger {
    private val LOG: Logger = Logger.getInstance(MtlogLogger::class.java)
    private const val NOTIFICATION_GROUP_ID = "mtlog.analyzer.log"
    
    // Key for storing ConsoleView in project user data
    val CONSOLE_VIEW_KEY: Key<ConsoleView> = Key.create("mtlog.analyzer.console")
    
    private val timeFormatter = DateTimeFormatter.ofPattern("HH:mm:ss.SSS")
    
    enum class LogLevel(val priority: Int) {
        ERROR(1),
        WARN(2),
        INFO(3),
        DEBUG(4),
        TRACE(5)
    }
    
    /**
     * Get the current log level from settings or default to INFO
     */
    private fun getLogLevel(project: Project?): LogLevel {
        return project?.let {
            try {
                val settingsState = it.service<MtlogSettingsState>()
                LogLevel.valueOf(settingsState.logLevel ?: "INFO")
            } catch (e: Exception) {
                LogLevel.INFO
            }
        } ?: LogLevel.INFO
    }
    
    /**
     * Check if a message should be logged based on current log level
     */
    private fun shouldLog(level: LogLevel, project: Project?): Boolean {
        val currentLevel = getLogLevel(project)
        return level.priority <= currentLevel.priority
    }
    
    /**
     * Format a log message with timestamp and level
     */
    private fun formatMessage(level: LogLevel, message: String): String {
        val timestamp = LocalDateTime.now().format(timeFormatter)
        return "[$timestamp] [${level.name}] $message"
    }
    
    /**
     * Get ConsoleView content type for a log level
     */
    private fun getContentType(level: LogLevel): ConsoleViewContentType {
        return when (level) {
            LogLevel.ERROR -> ConsoleViewContentType.ERROR_OUTPUT
            LogLevel.WARN -> ConsoleViewContentType.LOG_WARNING_OUTPUT
            LogLevel.DEBUG, LogLevel.TRACE -> ConsoleViewContentType.LOG_DEBUG_OUTPUT
            else -> ConsoleViewContentType.NORMAL_OUTPUT
        }
    }
    
    /**
     * Write to ConsoleView if available
     */
    private fun writeToConsole(project: Project?, level: LogLevel, message: String) {
        project?.getUserData(CONSOLE_VIEW_KEY)?.let { console ->
            val formattedMessage = formatMessage(level, message)
            console.print("$formattedMessage\n", getContentType(level))
        }
    }
    
    /**
     * Send notification to Event Log for important messages
     */
    private fun sendNotification(project: Project?, level: LogLevel, message: String) {
        if (project == null || level.priority > LogLevel.WARN.priority) return
        
        val notificationType = when (level) {
            LogLevel.ERROR -> NotificationType.ERROR
            LogLevel.WARN -> NotificationType.WARNING
            else -> NotificationType.INFORMATION
        }
        
        NotificationGroupManager.getInstance()
            .getNotificationGroup(NOTIFICATION_GROUP_ID)
            .createNotification("mtlog Analyzer", message, notificationType)
            .notify(project)
    }
    
    // Public logging methods
    
    fun error(message: String, project: Project? = null, throwable: Throwable? = null) {
        if (!shouldLog(LogLevel.ERROR, project)) return
        
        val fullMessage = if (throwable != null) {
            "$message: ${throwable.message}"
        } else {
            message
        }
        
        LOG.error(fullMessage, throwable)
        writeToConsole(project, LogLevel.ERROR, fullMessage)
        sendNotification(project, LogLevel.ERROR, fullMessage)
        
        throwable?.let { 
            writeToConsole(project, LogLevel.ERROR, throwable.stackTraceToString())
        }
    }
    
    fun warn(message: String, project: Project? = null) {
        if (!shouldLog(LogLevel.WARN, project)) return
        
        LOG.warn(message)
        writeToConsole(project, LogLevel.WARN, message)
        sendNotification(project, LogLevel.WARN, message)
    }
    
    fun info(message: String, project: Project? = null) {
        if (!shouldLog(LogLevel.INFO, project)) return
        
        LOG.info(message)
        writeToConsole(project, LogLevel.INFO, message)
    }
    
    fun debug(message: String, project: Project? = null) {
        if (!shouldLog(LogLevel.DEBUG, project)) return
        
        LOG.debug(message)
        writeToConsole(project, LogLevel.DEBUG, message)
    }
    
    fun trace(message: String, project: Project? = null) {
        if (!shouldLog(LogLevel.TRACE, project)) return
        
        LOG.trace(message)
        writeToConsole(project, LogLevel.TRACE, message)
    }
    
    /**
     * Log analyzer-specific events with context
     */
    fun logAnalysis(
        project: Project?,
        file: String,
        diagnosticId: String? = null,
        message: String,
        level: LogLevel = LogLevel.DEBUG
    ) {
        val contextMessage = buildString {
            append("[")
            append(file)
            if (diagnosticId != null) {
                append(", ")
                append(diagnosticId)
            }
            append("] ")
            append(message)
        }
        
        when (level) {
            LogLevel.ERROR -> error(contextMessage, project)
            LogLevel.WARN -> warn(contextMessage, project)
            LogLevel.INFO -> info(contextMessage, project)
            LogLevel.DEBUG -> debug(contextMessage, project)
            LogLevel.TRACE -> trace(contextMessage, project)
        }
    }
    
    /**
     * Clear the console view if available
     */
    fun clearConsole(project: Project) {
        project.getUserData(CONSOLE_VIEW_KEY)?.clear()
    }
}
package com.mtlog.analyzer.toolwindow

import com.intellij.execution.filters.TextConsoleBuilderFactory
import com.intellij.execution.ui.ConsoleView
import com.intellij.openapi.actionSystem.ActionManager
import com.intellij.openapi.actionSystem.ActionToolbar
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.DefaultActionGroup
import com.intellij.openapi.project.DumbAwareAction
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.content.ContentFactory
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.logging.MtlogLogger
import java.awt.BorderLayout
import javax.swing.JPanel

/**
 * Tool window factory for mtlog analyzer console output.
 */
class MtlogConsoleToolWindowFactory : ToolWindowFactory {
    
    override fun createToolWindowContent(project: Project, toolWindow: ToolWindow) {
        val consoleView = createConsoleView(project)
        
        // Store console view in project user data for logger access
        project.putUserData(MtlogLogger.CONSOLE_VIEW_KEY, consoleView)
        
        // Create panel with console and toolbar
        val panel = JPanel(BorderLayout())
        panel.add(consoleView.component, BorderLayout.CENTER)
        
        // Create toolbar with actions
        val toolbar = createToolbar(project, consoleView)
        panel.add(toolbar.component, BorderLayout.WEST)
        
        // Add content to tool window
        val contentFactory = ContentFactory.getInstance()
        val content = contentFactory.createContent(
            panel,
            MtlogBundle.message("toolwindow.console.title", "Console"),
            false
        )
        toolWindow.contentManager.addContent(content)
        
        // Register disposable to clean up console view
        Disposer.register(toolWindow.disposable, consoleView)
        
        // Log initial message
        MtlogLogger.info("mtlog analyzer console initialized", project)
    }
    
    private fun createConsoleView(project: Project): ConsoleView {
        return TextConsoleBuilderFactory.getInstance()
            .createBuilder(project)
            .console
    }
    
    private fun createToolbar(project: Project, consoleView: ConsoleView): ActionToolbar {
        val actionGroup = DefaultActionGroup()
        
        // Clear console action
        actionGroup.add(object : DumbAwareAction(
            MtlogBundle.message("action.console.clear", "Clear"),
            MtlogBundle.message("action.console.clear.description", "Clear console output"),
            com.intellij.icons.AllIcons.Actions.GC
        ) {
            override fun actionPerformed(e: AnActionEvent) {
                consoleView.clear()
                MtlogLogger.debug("Console cleared", project)
            }
        })
        
        // Separator
        actionGroup.addSeparator()
        
        // Scroll to end action
        actionGroup.add(object : DumbAwareAction(
            MtlogBundle.message("action.console.scrollToEnd", "Scroll to End"),
            MtlogBundle.message("action.console.scrollToEnd.description", "Scroll to the end of console output"),
            com.intellij.icons.AllIcons.RunConfigurations.Scroll_down
        ) {
            override fun actionPerformed(e: AnActionEvent) {
                consoleView.scrollTo(consoleView.contentSize)
            }
        })
        
        return ActionManager.getInstance().createActionToolbar(
            "MtlogConsoleToolbar",
            actionGroup,
            false
        ).apply {
            setTargetComponent(consoleView.component)
        }
    }
    
    override fun shouldBeAvailable(project: Project): Boolean = true
}
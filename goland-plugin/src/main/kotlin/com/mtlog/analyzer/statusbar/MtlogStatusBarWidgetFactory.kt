package com.mtlog.analyzer.statusbar

import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.StatusBar
import com.intellij.openapi.wm.StatusBarWidget
import com.intellij.openapi.wm.StatusBarWidgetFactory
import com.mtlog.analyzer.MtlogBundle

/**
 * Factory for creating mtlog status bar widgets.
 */
class MtlogStatusBarWidgetFactory : StatusBarWidgetFactory {
    
    override fun getId(): String = MtlogStatusBarWidget.ID
    
    override fun getDisplayName(): String = MtlogBundle.message("statusbar.widget.name")
    
    override fun isAvailable(project: Project): Boolean = true
    
    override fun createWidget(project: Project): StatusBarWidget = MtlogStatusBarWidget(project)
    
    override fun disposeWidget(widget: StatusBarWidget) {
        // Widget disposal is handled by the widget itself
    }
    
    override fun canBeEnabledOn(statusBar: StatusBar): Boolean = true
}
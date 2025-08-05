package com.mtlog.analyzer.statusbar

import com.intellij.openapi.components.service
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.wm.StatusBar
import com.intellij.openapi.wm.StatusBarWidget
import com.intellij.openapi.wm.StatusBarWidget.WidgetPresentation
import com.intellij.openapi.wm.impl.status.EditorBasedWidget
import com.intellij.util.Consumer
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.annotator.MtlogExternalAnnotator
import java.awt.Component
import java.awt.event.MouseEvent
import javax.swing.Icon
import com.intellij.icons.AllIcons

/**
 * Status bar widget for mtlog analyzer.
 */
class MtlogStatusBarWidget(project: Project) : EditorBasedWidget(project), StatusBarWidget.IconPresentation {
    
    companion object {
        const val ID = "MtlogAnalyzer"
    }
    
    override fun ID(): String = ID
    
    override fun getPresentation(): WidgetPresentation = this
    
    override fun getTooltipText(): String {
        val service = project.service<MtlogProjectService>()
        return if (service.state.enabled) {
            val suppressedCount = service.state.suppressedDiagnostics.size
            if (suppressedCount > 0) {
                MtlogBundle.message("statusbar.tooltip.enabled.with.suppressed", suppressedCount)
            } else {
                MtlogBundle.message("statusbar.tooltip.enabled")
            }
        } else {
            MtlogBundle.message("statusbar.tooltip.disabled")
        }
    }
    
    override fun getIcon(): Icon {
        val service = project.service<MtlogProjectService>()
        return if (service.state.enabled) {
            AllIcons.Actions.Execute // Green play icon when enabled
        } else {
            AllIcons.Actions.Suspend // Paused icon when disabled
        }
    }
    
    override fun getClickConsumer(): Consumer<MouseEvent>? {
        return Consumer { _ ->
            val service = project.service<MtlogProjectService>()
            service.state.enabled = !service.state.enabled
            
            // Restart processes to force re-analysis
            service.restartProcesses()
            
            // Update the widget
            myStatusBar?.updateWidget(ID)
            
            // Trigger re-analysis (will clear annotations if disabled, re-analyze if enabled)
            com.intellij.codeInsight.daemon.DaemonCodeAnalyzer.getInstance(project).restart()
            
            // No notification - the status bar widget itself shows the state
        }
    }
    
    override fun install(statusBar: StatusBar) {
        super.install(statusBar)
    }
    
    override fun dispose() {
        Disposer.dispose(this)
    }
}
package com.mtlog.analyzer.settings

import com.intellij.openapi.components.service
import com.intellij.openapi.fileChooser.FileChooser
import com.intellij.openapi.fileChooser.FileChooserDescriptor
import com.intellij.openapi.options.BoundConfigurable
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.TextFieldWithBrowseButton
import com.intellij.ui.dsl.builder.*
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.annotator.MtlogExternalAnnotator
import javax.swing.DefaultComboBoxModel

/**
 * Settings UI for mtlog-analyzer configuration.
 */
class MtlogSettingsConfigurable(private val project: Project) : BoundConfigurable(
    MtlogBundle.message("settings.display.name")
) {
    private val service = project.service<MtlogProjectService>()
    
    override fun createPanel() = panel {
        val state = service.state
        
        row {
            checkBox(MtlogBundle.message("settings.enabled"))
                .bindSelected(state::enabled)
        }
        
        separator()
        
        row(MtlogBundle.message("settings.analyzer.path")) {
            val textField = TextFieldWithBrowseButton()
            val descriptor = FileChooserDescriptor(true, false, false, false, false, false)
            descriptor.title = MtlogBundle.message("settings.analyzer.path.choose")
            descriptor.description = MtlogBundle.message("settings.analyzer.path.description")
            
            textField.addActionListener {
                val file = FileChooser.chooseFile(descriptor, project, null)
                if (file != null) {
                    textField.text = file.path
                }
            }
            
            cell(textField)
                .bindText(
                    getter = { state.analyzerPath ?: "" },
                    setter = { state.analyzerPath = it }
                )
        }
        
        row(MtlogBundle.message("settings.analyzer.flags")) {
            expandableTextField()
                .bindText(
                    getter = { state.analyzerFlags.joinToString(" ") },
                    setter = { text -> 
                        state.analyzerFlags = text.split(" ")
                            .filter { it.isNotBlank() }
                            .toMutableList()
                    }
                )
        }
        
        separator()
        
        row("Suppressed Diagnostics") {
            button("Manage Suppressed Diagnostics...") {
                val dialog = com.mtlog.analyzer.actions.SuppressionManagerDialog(project)
                if (dialog.showAndGet()) {
                    // Dialog handles the updates
                    service.restartProcesses()
                }
            }
            comment("Configure which diagnostic types to suppress project-wide")
        }
        
        separator()
        
        group("Logging") {
            row("Log Level") {
                comboBox(logLevelModel())
                    .bindItem(
                        getter = { state.logLevel },
                        setter = { state.logLevel = it ?: "INFO" }
                    )
                comment("Controls the verbosity of plugin logging in the mtlog Analyzer console")
            }
        }
        
        separator()
        
        group("Severity Mapping") {
            row(MtlogBundle.message("settings.severity.error")) {
                comboBox(severityModel())
                    .bindItem(
                        getter = { state.errorSeverity },
                        setter = { state.errorSeverity = it ?: "ERROR" }
                    )
            }
            
            row(MtlogBundle.message("settings.severity.warning")) {
                comboBox(severityModel())
                    .bindItem(
                        getter = { state.warningSeverity },
                        setter = { state.warningSeverity = it ?: "WARNING" }
                    )
            }
            
            row(MtlogBundle.message("settings.severity.suggestion")) {
                comboBox(severityModel())
                    .bindItem(
                        getter = { state.suggestionSeverity },
                        setter = { state.suggestionSeverity = it ?: "WEAK_WARNING" }
                    )
            }
        }
    }
    
    override fun apply() {
        val wasEnabled = service.state.enabled
        super.apply()
        val isEnabled = service.state.enabled
        
        // Restart processes when settings change
        service.restartProcesses()
        
        // If enabled state changed, trigger re-analysis or clear annotations
        if (wasEnabled != isEnabled) {
            com.intellij.codeInsight.daemon.DaemonCodeAnalyzer.getInstance(project).restart()
            service.updateStatusBarWidget()
        }
    }
    
    private fun severityModel() = DefaultComboBoxModel(
        arrayOf("ERROR", "WARNING", "WEAK_WARNING", "INFO")
    )
    
    private fun logLevelModel() = DefaultComboBoxModel(
        arrayOf("ERROR", "WARN", "INFO", "DEBUG", "TRACE")
    )
}
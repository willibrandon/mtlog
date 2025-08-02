package com.mtlog.analyzer.settings

import com.intellij.openapi.components.service
import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory
import com.intellij.openapi.options.BoundConfigurable
import com.intellij.openapi.project.Project
import com.intellij.ui.dsl.builder.*
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.service.MtlogProjectService
import javax.swing.DefaultComboBoxModel

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
            textFieldWithBrowseButton(
                fileChooserDescriptor = FileChooserDescriptorFactory.createSingleFileDescriptor()
            ).bindText(
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
        super.apply()
        // Clear cache when settings change
        service.clearCache()
    }
    
    private fun severityModel() = DefaultComboBoxModel(
        arrayOf("ERROR", "WARNING", "WEAK_WARNING", "INFO")
    )
}
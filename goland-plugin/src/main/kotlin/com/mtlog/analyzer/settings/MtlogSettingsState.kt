package com.mtlog.analyzer.settings

import com.intellij.openapi.components.BaseState
import com.intellij.util.xmlb.annotations.Tag

/**
 * Persistent plugin settings.
 */
class MtlogSettingsState : BaseState() {
    @get:Tag("analyzerPath")
    var analyzerPath by string("mtlog-analyzer")
    
    @get:Tag("analyzerFlags")
    var analyzerFlags by list<String>()
    
    @get:Tag("errorSeverity")
    var errorSeverity by string("ERROR")
    
    @get:Tag("warningSeverity")
    var warningSeverity by string("WARNING")
    
    @get:Tag("suggestionSeverity")
    var suggestionSeverity by string("WEAK_WARNING")
    
    @get:Tag("enabled")
    var enabled by property(true)
    
    @get:Tag("suppressedDiagnostics")
    var suppressedDiagnostics by list<String>()
    
    @get:Tag("logLevel")
    var logLevel by string("INFO")
}
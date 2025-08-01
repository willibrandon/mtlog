package com.mtlog.goland.integration

import com.intellij.openapi.components.service
import com.mtlog.goland.service.AnalyzerDiagnostic
import com.mtlog.goland.service.MtlogProjectService
import com.mtlog.goland.settings.MtlogSettingsState
import java.io.File

class MtlogExternalAnnotatorIntegrationTest : MtlogIntegrationTestBase() {
    
    fun testRealAnalyzerDetectsTemplateArgumentMismatch() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // This should trigger an error - 2 properties but 1 argument
                log.Information("User {UserId} logged in at {Time}", 123)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val errors = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "error" }
        
        assertEquals(1, errors.size)
        val error = errors[0]
        assertTrue("Error should mention argument count", 
            error.message.contains("2 properties but 1 argument") ||
            error.message.contains("template has 2 properties but 1 arguments provided"))
    }
    
    fun testRealAnalyzerDetectsPascalCaseWarning() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // This should trigger a warning - non-PascalCase property
                log.Debug("Processing {user_id}", 456)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "warning" || it.severity == "suggestion" }
        
        assertTrue("Should have at least one warning", warnings.isNotEmpty())
        val warning = warnings[0]
        assertTrue("Warning should mention PascalCase", 
            warning.message.contains("PascalCase") ||
            warning.message.contains("consider using PascalCase"))
        assertTrue("Warning should mention the property name", 
            warning.message.contains("user_id"))
    }
    
    fun testRealAnalyzerIgnoresCorrectUsage() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // This should not trigger any errors or warnings
                log.Warning("Disk usage at {Percentage:P1}", 0.85)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val problems = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "error" || 
                     it.severity == "warning" }
        
        assertTrue("Should have no errors or warnings for correct usage", problems.isEmpty())
    }
    
    fun testRealAnalyzerWithComplexTemplate() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            type User struct {
                ID   int
                Name string
            }
            
            func main() {
                log := mtlog.New()
                user := User{ID: 123, Name: "John"}
                
                // Should suggest using @ for complex type
                log.Information("User logged in: {User}", user)
                
                // This is correct
                log.Information("User logged in: {@User}", user)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val suggestions = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "suggestion" }
        
        // Should have a suggestion for the first log statement
        assertTrue("Should have suggestions for complex type usage", suggestions.isNotEmpty())
    }
    
    fun testRealAnalyzerHandlesMultipleFiles() {
        // Create multiple Go files using real files only
        val userDir = File(realProjectDir, "user")
        userDir.mkdirs()
        
        File(userDir, "user.go").writeText("""
            package user
            
            import "github.com/willibrandon/mtlog"
            
            func LogUser(log *mtlog.Logger, id int) {
                // Error: wrong argument count
                log.Information("User {UserId} action {Action}", id)
            }
        """.trimIndent())
        
        createGoFile("main.go", """
            package main
            
            import (
                "github.com/willibrandon/mtlog"
                "testproject/user"
            )
            
            func main() {
                log := mtlog.New()
                // Warning: non-PascalCase
                log.Debug("Starting {app_name}", "myapp")
                
                user.LogUser(log, 123)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val allProblems = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
        
        // Should detect problems in the current file only (main.go)
        assertTrue("Should detect problems in the current file", allProblems.isNotEmpty())
    }
    
    fun testRealAnalyzerWithDifferentSeveritySettings() {
        // Change severity settings
        val service = project.service<MtlogProjectService>()
        val originalState = MtlogSettingsState().apply {
            enabled = service.state.enabled
            analyzerPath = service.state.analyzerPath
            analyzerFlags = service.state.analyzerFlags.toMutableList()
            errorSeverity = service.state.errorSeverity
            warningSeverity = service.state.warningSeverity
            suggestionSeverity = service.state.suggestionSeverity
        }
        
        try {
            // Set all severities to INFO
            service.state.errorSeverity = "INFO"
            service.state.warningSeverity = "INFO"
            service.state.suggestionSeverity = "INFO"
            
            createGoFile("main.go", """
                package main
                
                import "github.com/willibrandon/mtlog"
                
                func main() {
                    log := mtlog.New()
                    log.Information("User {UserId} logged in at {Time}", 123)
                }
            """.trimIndent())
            
            val diagnostics = runRealAnalyzer()
            val allDiagnostics = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            
            // The service still returns "error" severity - the mapping to INFO happens in the annotator
            assertTrue("Should still detect diagnostics", allDiagnostics.isNotEmpty())
            assertEquals("Service should return error severity", "error", allDiagnostics[0].severity)
            
        } finally {
            // Restore original settings
            service.loadState(originalState)
        }
    }
    
    fun testRealAnalyzerDisabledState() {
        val service = project.service<MtlogProjectService>()
        val originalEnabled = service.state.enabled
        
        try {
            // Disable the analyzer
            service.state.enabled = false
            
            createGoFile("main.go", """
                package main
                
                import "github.com/willibrandon/mtlog"
                
                func main() {
                    log := mtlog.New()
                    // This would normally be an error
                    log.Information("User {UserId} logged in at {Time}", 123)
                }
            """.trimIndent())
            
            val diagnostics = runRealAnalyzer()
            val mtlogProblems = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
                .filter { it.message.contains("mtlog", ignoreCase = true) ||
                         it.message.contains("template", ignoreCase = true) ||
                         it.message.contains("properties", ignoreCase = true) }
            
            assertTrue("Should have no mtlog problems when disabled", mtlogProblems.isEmpty())
            
        } finally {
            service.state.enabled = originalEnabled
        }
    }
    
    fun testHighlightRangesAreCorrect() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("User {UserId} logged in at {Time}", 123)
                log.Debug("Processing {user_id}", 456)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        
        // Check that diagnostics have proper line/column information
        for (diagnostic in diagnostics.filterIsInstance<AnalyzerDiagnostic>()) {
            assertTrue("Line number should be positive", diagnostic.lineNumber > 0)
            assertTrue("Column number should be positive", diagnostic.columnNumber > 0)
            
            // For template errors, verify line number
            if (diagnostic.message.contains("properties") && diagnostic.message.contains("argument")) {
                assertEquals("Should be on line 7", 7, diagnostic.lineNumber)
            }
            
            // For PascalCase warnings, verify property name extraction
            if (diagnostic.message.contains("PascalCase") && diagnostic.message.contains("user_id")) {
                assertEquals("Should extract property name", "user_id", diagnostic.propertyName)
            }
        }
    }
}
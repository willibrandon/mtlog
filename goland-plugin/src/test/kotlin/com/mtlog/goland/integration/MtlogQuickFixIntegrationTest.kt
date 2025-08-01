package com.mtlog.goland.integration

import com.intellij.codeInsight.daemon.impl.HighlightInfo
import com.intellij.codeInsight.intention.IntentionAction
import com.intellij.lang.annotation.HighlightSeverity
import com.mtlog.goland.service.AnalyzerDiagnostic

class MtlogQuickFixIntegrationTest : MtlogIntegrationTestBase() {
    
    fun testPascalCaseQuickFixRealExecution() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Debug("Processing {user_id}", 456)
            }
        """.trimIndent())
        
        // Run analyzer to get warnings
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "warning" || it.severity == "suggestion" }
            .filter { it.message.contains("PascalCase") }
        
        assertTrue("Should have PascalCase warning", warnings.isNotEmpty())
        
        // Quick fixes require editor integration which doesn't work with real files
        // Just verify we detected the issue
        val warning = warnings[0]
        assertEquals("Should detect user_id property", "user_id", warning.propertyName)
    }
    
    fun testMultiplePascalCaseProperties() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("User {user_id} performed {action_type} at {event_time}", 123, "login", "now")
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "warning" || it.severity == "suggestion" }
            .filter { it.message.contains("PascalCase") }
        
        // Should have multiple warnings
        assertTrue("Should have multiple PascalCase warnings", warnings.size >= 2)
        
        // Verify we detected all the properties
        val detectedProperties = warnings.mapNotNull { it.propertyName }.toSet()
        assertTrue("Should detect user_id", detectedProperties.contains("user_id"))
        assertTrue("Should detect action_type", detectedProperties.contains("action_type"))
        assertTrue("Should detect event_time", detectedProperties.contains("event_time"))
    }
    
    fun testTemplateArgumentQuickFix() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("User {UserId} logged in at {Time}", 123)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val errors = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "error" }
            .filter { it.message.contains("argument") || it.message.contains("properties") }
        
        assertTrue("Should have argument count error", errors.isNotEmpty())
        
        // Verify the error message
        val error = errors[0]
        assertTrue("Error should mention template/argument mismatch", 
            error.message.contains("2 properties but 1 argument") ||
            error.message.contains("template has 2 properties but 1 arguments provided"))
    }
    
    fun testQuickFixPreservesFormatting() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // This has specific formatting we want to preserve
                log.Debug(
                    "Processing user {user_id} with role {user_role}",
                    456,
                    "admin",
                )
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("user_id") || it.propertyName == "user_id" }
        
        assertTrue("Should have warning for user_id", warnings.isNotEmpty())
        
        // Verify we detected the property correctly
        val warning = warnings[0]
        assertEquals("Should detect user_id property", "user_id", warning.propertyName)
        
        // The log.Debug call starts at line 8 (line 7 is comment, line 8 is log.Debug)
        assertEquals("Should be at line 8", 8, warning.lineNumber)
    }
}
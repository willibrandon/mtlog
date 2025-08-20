package com.mtlog.analyzer.integration

import com.intellij.openapi.components.service
import com.mtlog.analyzer.service.AnalyzerDiagnostic
import com.mtlog.analyzer.service.MtlogProjectService
import java.io.File

class WithDiagnosticsIntegrationTest : MtlogIntegrationTestBase() {
    
    override fun shouldSetupRealTestProject(): Boolean = true
    
    fun testMTLOG009_OddNumberOfArguments() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // MTLOG009: Odd number of arguments
                log.With("key1", "value1", "key2")
                
                // Valid usage
                log.With("key1", "value1", "key2", "value2")
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val errors = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG009") || it.message.contains("even number") }
        
        assertEquals("Should detect odd number of arguments", 1, errors.size)
        assertTrue("Error should mention even number requirement", 
            errors[0].message.contains("even number") || 
            errors[0].message.contains("key-value pairs"))
        assertEquals("Should be on line 8", 8, errors[0].lineNumber)
    }
    
    fun testMTLOG010_NonStringKeys() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                
                // MTLOG010: Non-string keys
                log.With(123, "value")
                log.With(true, "value")
                
                userId := 456
                log.With(userId, "someValue")  // Variable that isn't a string
                
                // Valid usage
                log.With("key", "value")
                
                const key = "myKey"
                log.With(key, "value")  // String constant is ok
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val errors = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG010") || it.message.contains("key must be a string") }
        
        assertTrue("Should detect non-string keys", errors.size >= 3)
        
        // Check specific errors
        val numericKeyError = errors.find { it.lineNumber == 9 }
        assertNotNull("Should detect numeric key on line 9", numericKeyError)
        assertTrue("Should mention numeric literal", 
            numericKeyError!!.message.contains("numeric") || 
            numericKeyError.message.contains("123"))
        
        val boolKeyError = errors.find { it.lineNumber == 10 }
        assertNotNull("Should detect boolean key on line 10", boolKeyError)
        
        val variableKeyError = errors.find { it.lineNumber == 13 }
        assertNotNull("Should detect non-string variable key on line 13", variableKeyError)
    }
    
    fun testMTLOG011_CrossCallDuplicates() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                
                // MTLOG011: Cross-call duplicates
                logger := log.With("service", "api")
                logger = logger.With("service", "auth")  // Overrides previous value
                
                // Also works with ForContext
                ctx := log.ForContext("user", "alice")
                ctx = ctx.ForContext("user", "bob")  // Overrides previous value
                
                // Method chaining
                log.With("id", 1).With("id", 2)  // Overrides in chain
                
                // Valid usage - different keys
                logger2 := log.With("service", "api")
                logger2 = logger2.With("version", "v1")  // Different key, no problem
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG011") || it.message.contains("overrides property") }
        
        assertTrue("Should detect cross-call duplicates", warnings.size >= 3)
        
        // Check specific warnings
        val withOverride = warnings.find { it.lineNumber == 10 }
        assertNotNull("Should detect With() override on line 10", withOverride)
        assertTrue("Should mention property 'service'", 
            withOverride!!.message.contains("service"))
        
        val forContextOverride = warnings.find { it.lineNumber == 14 }
        assertNotNull("Should detect ForContext() override on line 14", forContextOverride)
        assertTrue("Should mention property 'user'", 
            forContextOverride!!.message.contains("user"))
        
        val chainOverride = warnings.find { it.lineNumber == 17 }
        assertNotNull("Should detect chained override on line 17", chainOverride)
        assertTrue("Should mention property 'id'", 
            chainOverride!!.message.contains("id"))
    }
    
    fun testMTLOG012_ReservedProperties() {
        // This test requires the -check-reserved flag to be enabled
        val service = project.service<MtlogProjectService>()
        val originalFlags = service.state.analyzerFlags.toMutableList()
        
        try {
            // Enable reserved property checking
            service.state.analyzerFlags.clear()
            service.state.analyzerFlags.add("-check-reserved")
            
            createGoFile("main.go", """
                package main
                
                import "github.com/willibrandon/mtlog"
                
                func main() {
                    log := mtlog.New()
                    
                    // MTLOG012: Reserved properties (when flag is enabled)
                    log.With("Message", "custom message")
                    log.With("Timestamp", "2024-01-01")
                    log.With("Level", "DEBUG")
                    log.With("Exception", "my error")
                    
                    // Valid usage - non-reserved properties
                    log.With("UserMessage", "custom message")
                    log.With("CustomTimestamp", "2024-01-01")
                }
            """.trimIndent())
            
            val diagnostics = runRealAnalyzer()
            val suggestions = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
                .filter { it.message.contains("MTLOG012") || it.message.contains("shadows") }
            
            assertTrue("Should detect reserved properties when flag is enabled", suggestions.size >= 4)
            
            // Check specific suggestions
            val messageWarning = suggestions.find { it.lineNumber == 9 }
            assertNotNull("Should detect 'Message' as reserved on line 9", messageWarning)
            assertTrue("Should mention it shadows a built-in property", 
                messageWarning!!.message.contains("shadows a built-in property"))
            
        } finally {
            // Restore original flags
            service.state.analyzerFlags.clear()
            service.state.analyzerFlags.addAll(originalFlags)
        }
    }
    
    fun testMTLOG013_EmptyKeys() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                
                // MTLOG013: Empty keys
                log.With("", "value")
                
                // Empty string constant
                const emptyKey = ""
                log.With(emptyKey, "value")
                
                // Valid usage
                log.With("key", "value")
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val errors = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG013") || it.message.contains("empty") && it.message.contains("key") }
        
        assertTrue("Should detect empty keys", errors.size >= 1)
        
        val emptyKeyError = errors.find { it.lineNumber == 9 }
        assertNotNull("Should detect empty key on line 9", emptyKeyError)
        assertTrue("Should mention key will be ignored", 
            emptyKeyError!!.message.contains("ignored") || 
            emptyKeyError.message.contains("empty"))
    }
    
    fun testMTLOG003_DuplicateKeysInSameCall() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                
                // MTLOG003: Duplicate keys in same call
                log.With("id", 1, "name", "test", "id", 2)
                
                // Multiple duplicates
                log.With(
                    "service", "api",
                    "version", "v1",
                    "service", "auth",
                    "version", "v2",
                )
                
                // Valid usage - unique keys
                log.With("id", 1, "name", "test", "age", 30)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG003") || it.message.contains("duplicate key") }
        
        assertTrue("Should detect duplicate keys in same call", warnings.isNotEmpty())
        
        // Check for 'id' duplicate
        val idDuplicate = warnings.find { it.message.contains("'id'") }
        assertNotNull("Should detect duplicate 'id' key", idDuplicate)
        assertTrue("Should mention previous position", 
            idDuplicate!!.message.contains("previous") || 
            idDuplicate.message.contains("position"))
    }
    
    fun testComplexWithScenarios() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                
                // Complex scenario: Multiple issues in one call
                log.With(
                    123, "value1",        // MTLOG010: non-string key
                    "valid", "value2",    
                    "", "value3",         // MTLOG013: empty key
                    "valid", "value4",    // MTLOG003: duplicate
                    "lastKey")            // MTLOG009: odd number
                
                // Cross-call with multiple overrides
                logger := log.With("env", "dev", "service", "api")
                logger = logger.With("env", "prod")  // MTLOG011: overrides env
                logger = logger.With("service", "auth")  // MTLOG011: overrides service
                
                // Valid complex usage
                finalLogger := log.With(
                    "environment", "production",
                    "service", "payment-api",
                    "version", "2.0.1",
                    "region", "us-west",
                )
                finalLogger.Information("Service started")
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val allProblems = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG") }
        
        // Should detect multiple issues
        assertTrue("Should detect multiple With() issues", allProblems.size >= 5)
        
        // Verify different diagnostic types are detected
        val hasNonStringKey = allProblems.any { it.message.contains("MTLOG010") }
        val hasEmptyKey = allProblems.any { it.message.contains("MTLOG013") }
        val hasDuplicate = allProblems.any { it.message.contains("MTLOG003") }
        val hasOddArgs = allProblems.any { it.message.contains("MTLOG009") }
        val hasCrossCallDup = allProblems.any { it.message.contains("MTLOG011") }
        
        assertTrue("Should detect non-string key", hasNonStringKey)
        assertTrue("Should detect empty key", hasEmptyKey)
        assertTrue("Should detect duplicate key", hasDuplicate)
        assertTrue("Should detect odd arguments", hasOddArgs)
        assertTrue("Should detect cross-call duplicates", hasCrossCallDup)
    }
    
    fun testWithDiagnosticsWithConfigurationFlags() {
        val service = project.service<MtlogProjectService>()
        val originalFlags = service.state.analyzerFlags.toMutableList()
        
        try {
            // Test with disabled checks
            service.state.analyzerFlags.clear()
            service.state.analyzerFlags.add("-disable=with-odd,with-nonstring")
            
            createGoFile("main.go", """
                package main
                
                import "github.com/willibrandon/mtlog"
                
                func main() {
                    log := mtlog.New()
                    
                    // These should be suppressed with flags
                    log.With("key1", "value1", "key2")  // Odd args - suppressed
                    log.With(123, "value")  // Non-string key - suppressed
                    
                    // This should still be detected
                    log.With("id", 1, "id", 2)  // Duplicate key
                }
            """.trimIndent())
            
            val diagnostics = runRealAnalyzer()
            val allProblems = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
                .filter { it.message.contains("MTLOG") }
            
            // Should only detect duplicate, not odd args or non-string key
            val hasOddArgs = allProblems.any { it.message.contains("MTLOG009") }
            val hasNonStringKey = allProblems.any { it.message.contains("MTLOG010") }
            val hasDuplicate = allProblems.any { it.message.contains("MTLOG003") }
            
            assertFalse("Should not detect odd args when disabled", hasOddArgs)
            assertFalse("Should not detect non-string key when disabled", hasNonStringKey)
            assertTrue("Should still detect duplicate key", hasDuplicate)
            
        } finally {
            // Restore original flags
            service.state.analyzerFlags.clear()
            service.state.analyzerFlags.addAll(originalFlags)
        }
    }
}
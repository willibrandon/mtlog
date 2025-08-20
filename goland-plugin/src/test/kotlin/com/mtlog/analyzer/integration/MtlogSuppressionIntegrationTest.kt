package com.mtlog.analyzer.integration

import com.intellij.openapi.components.service
import com.mtlog.analyzer.annotator.MtlogExternalAnnotator
import com.mtlog.analyzer.service.MtlogProjectService
import com.mtlog.analyzer.actions.SuppressDiagnosticAction
import org.junit.Test

class MtlogSuppressionIntegrationTest : MtlogIntegrationTestBase() {
    
    override fun shouldSetupRealTestProject(): Boolean = true
    
    /**
     * Test diagnostic ID extraction from various message formats.
     */
    @Test
    fun testDiagnosticIdExtraction() {
        // Test extraction from [MTLOG00X] format
        assertEquals("MTLOG001", extractDiagnosticIdFromMessage("[MTLOG001] template has 1 properties but 2 arguments provided"))
        assertEquals("MTLOG002", extractDiagnosticIdFromMessage("[MTLOG002] invalid format specifier"))
        assertEquals("MTLOG003", extractDiagnosticIdFromMessage("[MTLOG003] duplicate property 'Id' in template"))
        assertEquals("MTLOG004", extractDiagnosticIdFromMessage("[MTLOG004] suggestion: consider using PascalCase for property 'userId'"))
        assertEquals("MTLOG005", extractDiagnosticIdFromMessage("[MTLOG005] warning: using @ prefix for basic type"))
        assertEquals("MTLOG006", extractDiagnosticIdFromMessage("[MTLOG006] suggestion: Error level log without error value"))
        assertEquals("MTLOG007", extractDiagnosticIdFromMessage("[MTLOG007] suggestion: consider defining a constant for commonly used context key"))
        assertEquals("MTLOG008", extractDiagnosticIdFromMessage("[MTLOG008] warning: dynamic template strings are not analyzed"))
        
        // Test extraction based on message content (without explicit ID)
        assertEquals("MTLOG001", extractDiagnosticIdFromMessage("template has 1 properties but 2 arguments provided"))
        assertEquals("MTLOG002", extractDiagnosticIdFromMessage("invalid format specifier in property 'Count:XYZ'"))
        assertEquals("MTLOG003", extractDiagnosticIdFromMessage("duplicate property 'UserId' in template"))
        assertEquals("MTLOG004", extractDiagnosticIdFromMessage("suggestion: consider using PascalCase for property 'user_id'"))
        assertEquals("MTLOG005", extractDiagnosticIdFromMessage("warning: using @ prefix for basic type string"))
        assertEquals("MTLOG006", extractDiagnosticIdFromMessage("Error level log without error value, consider including the error"))
        assertEquals("MTLOG007", extractDiagnosticIdFromMessage("suggestion: consider defining a constant for commonly used context key 'request_id'"))
        assertEquals("MTLOG008", extractDiagnosticIdFromMessage("warning: dynamic template strings are not analyzed"))
        
        // Test null return for unrecognized messages
        assertNull(extractDiagnosticIdFromMessage("some random error message"))
        assertNull(extractDiagnosticIdFromMessage(""))
    }
    
    /**
     * Test suppression state persistence.
     */
    @Test
    fun testSuppressionStatePersistence() {
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Initially, no diagnostics should be suppressed
        assertTrue("Initially no diagnostics should be suppressed", state.suppressedDiagnostics.isEmpty())
        
        // Add some suppressed diagnostics
        state.suppressedDiagnostics = mutableListOf("MTLOG001", "MTLOG004")
        
        // Verify they're stored
        assertEquals(2, state.suppressedDiagnostics.size)
        assertTrue(state.suppressedDiagnostics.contains("MTLOG001"))
        assertTrue(state.suppressedDiagnostics.contains("MTLOG004"))
        
        // Add another one
        val newList = state.suppressedDiagnostics.toMutableList()
        newList.add("MTLOG006")
        state.suppressedDiagnostics = newList
        
        // Verify all three are stored
        assertEquals(3, state.suppressedDiagnostics.size)
        assertTrue(state.suppressedDiagnostics.contains("MTLOG001"))
        assertTrue(state.suppressedDiagnostics.contains("MTLOG004"))
        assertTrue(state.suppressedDiagnostics.contains("MTLOG006"))
        
        // Remove one
        val removedList = state.suppressedDiagnostics.toMutableList()
        removedList.remove("MTLOG004")
        state.suppressedDiagnostics = removedList
        
        // Verify it's removed
        assertEquals(2, state.suppressedDiagnostics.size)
        assertTrue(state.suppressedDiagnostics.contains("MTLOG001"))
        assertFalse(state.suppressedDiagnostics.contains("MTLOG004"))
        assertTrue(state.suppressedDiagnostics.contains("MTLOG006"))
    }
    
    /**
     * Test that suppressed diagnostics are passed to analyzer via environment.
     */
    @Test
    fun testSuppressionEnvironmentVariable() {
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Clear any existing suppressions from previous tests
        state.suppressedDiagnostics = mutableListOf()
        service.restartProcesses()
        
        // Set up test file with known issues
        createGoFile("test.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // MTLOG001: Template/argument mismatch
                log.Information("User {UserId} logged in", 123, 456)
                // MTLOG004: PascalCase warning
                log.Warning("User {userId} is active", 789)
                // MTLOG006: Error without error value
                log.Error("Something went wrong")
            }
        """.trimIndent())
        
        // Run analyzer without suppression
        val diagnosticsBeforeSuppression = runRealAnalyzer()
        assertTrue("Should have diagnostics before suppression", diagnosticsBeforeSuppression.isNotEmpty())
        
        // Count diagnostic types
        var mtlog001Count = 0
        var mtlog004Count = 0
        var mtlog006Count = 0
        
        for (diag in diagnosticsBeforeSuppression) {
            val message = diag.toString()
            when {
                message.contains("template has") && message.contains("arguments") -> mtlog001Count++
                message.contains("PascalCase") -> mtlog004Count++
                message.contains("Error level log without error") -> mtlog006Count++
            }
        }
        
        assertTrue("Should have MTLOG001 diagnostic", mtlog001Count > 0)
        assertTrue("Should have MTLOG004 diagnostic", mtlog004Count > 0)
        assertTrue("Should have MTLOG006 diagnostic", mtlog006Count > 0)
        
        // Now suppress MTLOG001 and MTLOG004
        state.suppressedDiagnostics = mutableListOf("MTLOG001", "MTLOG004")
        service.restartProcesses()
        
        // Run analyzer again with suppression
        val diagnosticsAfterSuppression = runRealAnalyzer()
        
        // Debug: Print all diagnostics after suppression
        println("Diagnostics after suppression (${diagnosticsAfterSuppression.size} total):")
        for (diag in diagnosticsAfterSuppression) {
            println("  - $diag")
        }
        
        // Count diagnostic types again
        var mtlog001CountAfter = 0
        var mtlog004CountAfter = 0
        var mtlog006CountAfter = 0
        
        for (diag in diagnosticsAfterSuppression) {
            val message = diag.toString()
            when {
                message.contains("template has") && message.contains("arguments") -> mtlog001CountAfter++
                message.contains("PascalCase") -> mtlog004CountAfter++
                message.contains("Error level log without error") || message.contains("Error logging without error") -> mtlog006CountAfter++
            }
        }
        
        // MTLOG001 and MTLOG004 should be suppressed, but MTLOG006 should still appear
        assertEquals("MTLOG001 should be suppressed", 0, mtlog001CountAfter)
        assertEquals("MTLOG004 should be suppressed", 0, mtlog004CountAfter)
        assertTrue("MTLOG006 should still appear (found: $mtlog006CountAfter)", mtlog006CountAfter > 0)
    }
    
    /**
     * Test diagnostic descriptions mapping.
     */
    @Test
    fun testDiagnosticDescriptions() {
        val descriptions = SuppressDiagnosticAction.DIAGNOSTIC_DESCRIPTIONS
        
        assertEquals("Template/argument count mismatch", descriptions["MTLOG001"])
        assertEquals("Invalid format specifier", descriptions["MTLOG002"])
        assertEquals("Duplicate property names", descriptions["MTLOG003"])
        assertEquals("Property naming (PascalCase)", descriptions["MTLOG004"])
        assertEquals("Missing capturing hints", descriptions["MTLOG005"])
        assertEquals("Error logging without error value", descriptions["MTLOG006"])
        assertEquals("Context key constant suggestion", descriptions["MTLOG007"])
        assertEquals("Dynamic template warning", descriptions["MTLOG008"])
        
        // Verify all expected IDs are present
        assertEquals(8, descriptions.size)
    }
    
    /**
     * Test suppression workflow integration.
     */
    @Test
    fun testSuppressionWorkflow() {
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Start with no suppressions
        state.suppressedDiagnostics = mutableListOf()
        
        // Create file with PascalCase issue
        createGoFile("workflow.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("User {user_id} logged in", 123)
            }
        """.trimIndent())
        
        // Run analyzer - should get PascalCase warning
        val diagnosticsBeforeSuppression = runRealAnalyzer()
        assertTrue("Should have diagnostics", diagnosticsBeforeSuppression.isNotEmpty())
        
        val hasPascalCaseWarning = diagnosticsBeforeSuppression.any { diag ->
            diag.toString().contains("PascalCase")
        }
        assertTrue("Should have PascalCase warning", hasPascalCaseWarning)
        
        // Suppress MTLOG004 (PascalCase)
        state.suppressedDiagnostics = mutableListOf("MTLOG004")
        service.restartProcesses()
        
        // Run analyzer again - should not have PascalCase warning
        val diagnosticsAfterSuppression = runRealAnalyzer()
        val stillHasPascalCaseWarning = diagnosticsAfterSuppression.any { diag ->
            diag.toString().contains("PascalCase")
        }
        assertFalse("Should not have PascalCase warning after suppression", stillHasPascalCaseWarning)
        
        // Unsuppress MTLOG004
        state.suppressedDiagnostics = mutableListOf()
        service.restartProcesses()
        
        // Run analyzer again - should have PascalCase warning again
        val diagnosticsAfterUnsuppression = runRealAnalyzer()
        val hasPascalCaseWarningAgain = diagnosticsAfterUnsuppression.any { diag ->
            diag.toString().contains("PascalCase")
        }
        assertTrue("Should have PascalCase warning after unsuppression", hasPascalCaseWarningAgain)
    }
    
    // Helper function to extract diagnostic ID (mimics the real implementation)
    private fun extractDiagnosticIdFromMessage(message: String): String? {
        // First try to extract from [MTLOG00X] format
        val idMatch = Regex("\\[(MTLOG\\d{3})\\]").find(message)
        if (idMatch != null) {
            return idMatch.groupValues[1]
        }
        
        // Otherwise, determine from message content
        val msgLower = message.lowercase()
        return when {
            msgLower.contains("template has") && msgLower.contains("properties") && msgLower.contains("arguments") -> "MTLOG001"
            msgLower.contains("invalid format specifier") -> "MTLOG002"
            msgLower.contains("duplicate property") -> "MTLOG003"
            msgLower.contains("pascalcase") -> "MTLOG004"
            msgLower.contains("capturing") || msgLower.contains("@ prefix") || msgLower.contains("$ prefix") -> "MTLOG005"
            msgLower.contains("error level log without error") || msgLower.contains("error logging without error") -> "MTLOG006"
            msgLower.contains("context key") -> "MTLOG007"
            msgLower.contains("dynamic template") -> "MTLOG008"
            else -> null
        }
    }
}
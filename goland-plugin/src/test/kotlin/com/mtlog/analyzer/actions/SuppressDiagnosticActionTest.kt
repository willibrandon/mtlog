package com.mtlog.analyzer.actions

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.components.service
import com.intellij.testFramework.TestActionEvent
import com.intellij.testFramework.fixtures.BasePlatformTestCase
import com.mtlog.analyzer.service.MtlogProjectService
import org.junit.Test

class SuppressDiagnosticActionTest : BasePlatformTestCase() {
    
    @Test
    fun testSuppressDiagnosticAction() {
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Start with empty suppression list
        state.suppressedDiagnostics = mutableListOf()
        
        // Create action for MTLOG001
        val action = SuppressDiagnosticAction("MTLOG001", "Template/argument count mismatch")
        
        // Create action event
        val event = TestActionEvent.createTestEvent(action)
        
        // Action should be enabled initially
        action.update(event)
        assertTrue("Action should be enabled initially", event.presentation.isEnabled)
        
        // Perform the action
        action.actionPerformed(event)
        
        // Verify the diagnostic was suppressed
        assertTrue("MTLOG001 should be suppressed", state.suppressedDiagnostics.contains("MTLOG001"))
        
        // Action should now be disabled
        action.update(event)
        assertFalse("Action should be disabled after suppression", event.presentation.isEnabled)
    }
    
    @Test
    fun testMultipleSuppression() {
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Start with empty suppression list
        state.suppressedDiagnostics = mutableListOf()
        
        // Create actions for different diagnostics
        val action1 = SuppressDiagnosticAction("MTLOG001", "Template/argument count mismatch")
        val action4 = SuppressDiagnosticAction("MTLOG004", "Property naming (PascalCase)")
        val action6 = SuppressDiagnosticAction("MTLOG006", "Error logging without error value")
        
        // Suppress them one by one
        action1.actionPerformed(TestActionEvent.createTestEvent(action1))
        assertEquals(1, state.suppressedDiagnostics.size)
        assertTrue(state.suppressedDiagnostics.contains("MTLOG001"))
        
        action4.actionPerformed(TestActionEvent.createTestEvent(action4))
        assertEquals(2, state.suppressedDiagnostics.size)
        assertTrue(state.suppressedDiagnostics.contains("MTLOG001"))
        assertTrue(state.suppressedDiagnostics.contains("MTLOG004"))
        
        action6.actionPerformed(TestActionEvent.createTestEvent(action6))
        assertEquals(3, state.suppressedDiagnostics.size)
        assertTrue(state.suppressedDiagnostics.contains("MTLOG001"))
        assertTrue(state.suppressedDiagnostics.contains("MTLOG004"))
        assertTrue(state.suppressedDiagnostics.contains("MTLOG006"))
    }
    
    @Test
    fun testDuplicateSuppression() {
        val service = project.service<MtlogProjectService>()
        val state = service.state
        
        // Start with MTLOG001 already suppressed
        state.suppressedDiagnostics = mutableListOf("MTLOG001")
        
        // Create action for MTLOG001
        val action = SuppressDiagnosticAction("MTLOG001", "Template/argument count mismatch")
        val event = TestActionEvent.createTestEvent(action)
        
        // Action should be disabled
        action.update(event)
        assertFalse("Action should be disabled when already suppressed", event.presentation.isEnabled)
        
        // Perform the action (should do nothing)
        action.actionPerformed(event)
        
        // Should still have only one instance
        assertEquals(1, state.suppressedDiagnostics.size)
        assertEquals(1, state.suppressedDiagnostics.count { it == "MTLOG001" })
    }
    
    @Test
    fun testActionPresentation() {
        // Test action presentation text
        val action = SuppressDiagnosticAction("MTLOG004", "Property naming (PascalCase)")
        
        assertEquals("Suppress MTLOG004 diagnostic", action.templatePresentation.text)
        assertEquals("Suppress Property naming (PascalCase) diagnostics project-wide", action.templatePresentation.description)
    }
    
    @Test
    fun testAllDiagnosticDescriptions() {
        // Verify all diagnostic IDs have descriptions
        val allIds = listOf("MTLOG001", "MTLOG002", "MTLOG003", "MTLOG004", 
                           "MTLOG005", "MTLOG006", "MTLOG007", "MTLOG008")
        
        for (id in allIds) {
            val description = SuppressDiagnosticAction.DIAGNOSTIC_DESCRIPTIONS[id]
            assertNotNull("Description should exist for $id", description)
            assertTrue("Description should not be empty for $id", description!!.isNotEmpty())
        }
    }
}
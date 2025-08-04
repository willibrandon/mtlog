package com.mtlog.analyzer.quickfix

import com.intellij.openapi.components.service
import com.intellij.testFramework.fixtures.BasePlatformTestCase
import com.mtlog.analyzer.service.MtlogProjectService
import org.junit.Test

class SuppressDiagnosticQuickFixTest : BasePlatformTestCase() {
    
    @Test
    fun testQuickFixText() {
        val quickFix = SuppressDiagnosticQuickFix("MTLOG001")
        assertEquals("Suppress MTLOG001 (Template/argument count mismatch)", quickFix.text)
        assertEquals("Suppress mtlog diagnostic", quickFix.familyName)
        
        val quickFix2 = SuppressDiagnosticQuickFix("MTLOG004")
        assertEquals("Suppress MTLOG004 (Property naming (PascalCase))", quickFix2.text)
        
        // Test with unknown diagnostic ID
        val quickFixUnknown = SuppressDiagnosticQuickFix("MTLOG999")
        assertEquals("Suppress MTLOG999 (this diagnostic)", quickFixUnknown.text)
    }
    
    @Test
    fun testQuickFixAvailability() {
        val service = project.service<MtlogProjectService>()
        val quickFix = SuppressDiagnosticQuickFix("MTLOG001")
        
        // Initially available
        service.state.suppressedDiagnostics = mutableListOf()
        assertTrue("Should be available when not suppressed", 
                  quickFix.isAvailable(project, null, null))
        
        // Not available when already suppressed
        service.state.suppressedDiagnostics = mutableListOf("MTLOG001")
        assertFalse("Should not be available when already suppressed", 
                   quickFix.isAvailable(project, null, null))
        
        // Available again when unsuppressed
        service.state.suppressedDiagnostics = mutableListOf()
        assertTrue("Should be available again when unsuppressed", 
                  quickFix.isAvailable(project, null, null))
    }
    
    @Test
    fun testQuickFixInvoke() {
        val service = project.service<MtlogProjectService>()
        service.state.suppressedDiagnostics = mutableListOf()
        
        val quickFix = SuppressDiagnosticQuickFix("MTLOG004")
        
        // Invoke the quick fix
        quickFix.invoke(project, null, null)
        
        // Verify the diagnostic was suppressed
        assertTrue("MTLOG004 should be suppressed", 
                  service.state.suppressedDiagnostics.contains("MTLOG004"))
        assertEquals(1, service.state.suppressedDiagnostics.size)
    }
    
    @Test
    fun testMultipleQuickFixes() {
        val service = project.service<MtlogProjectService>()
        service.state.suppressedDiagnostics = mutableListOf()
        
        // Create and invoke multiple quick fixes
        val quickFix1 = SuppressDiagnosticQuickFix("MTLOG001")
        val quickFix4 = SuppressDiagnosticQuickFix("MTLOG004")
        val quickFix6 = SuppressDiagnosticQuickFix("MTLOG006")
        
        quickFix1.invoke(project, null, null)
        assertEquals(1, service.state.suppressedDiagnostics.size)
        assertTrue(service.state.suppressedDiagnostics.contains("MTLOG001"))
        
        quickFix4.invoke(project, null, null)
        assertEquals(2, service.state.suppressedDiagnostics.size)
        assertTrue(service.state.suppressedDiagnostics.contains("MTLOG001"))
        assertTrue(service.state.suppressedDiagnostics.contains("MTLOG004"))
        
        quickFix6.invoke(project, null, null)
        assertEquals(3, service.state.suppressedDiagnostics.size)
        assertTrue(service.state.suppressedDiagnostics.contains("MTLOG001"))
        assertTrue(service.state.suppressedDiagnostics.contains("MTLOG004"))
        assertTrue(service.state.suppressedDiagnostics.contains("MTLOG006"))
    }
    
    @Test
    fun testWriteAction() {
        val quickFix = SuppressDiagnosticQuickFix("MTLOG001")
        
        // Verify it doesn't require write action
        assertFalse("Should not require write action", quickFix.startInWriteAction())
    }
    
    @Test
    fun testDuplicateInvoke() {
        val service = project.service<MtlogProjectService>()
        service.state.suppressedDiagnostics = mutableListOf("MTLOG001")
        
        val quickFix = SuppressDiagnosticQuickFix("MTLOG001")
        
        // Invoke on already suppressed diagnostic
        quickFix.invoke(project, null, null)
        
        // Should still have only one instance
        assertEquals(1, service.state.suppressedDiagnostics.size)
        assertEquals(1, service.state.suppressedDiagnostics.count { it == "MTLOG001" })
    }
}
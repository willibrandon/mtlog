package com.mtlog.analyzer.actions

import com.intellij.openapi.components.service
import com.intellij.testFramework.fixtures.BasePlatformTestCase
import com.mtlog.analyzer.service.MtlogProjectService
import org.junit.Test
import javax.swing.JCheckBox

class SuppressionManagerDialogTest : BasePlatformTestCase() {
    
    @Test
    fun testEmptySuppressionList() {
        val service = project.service<MtlogProjectService>()
        service.state.suppressedDiagnostics = mutableListOf()
        
        val dialog = SuppressionManagerDialog(project)
        val panel = dialog.createCenterPanel()
        
        assertNotNull("Panel should be created", panel)
        
        // Dialog should indicate no suppressions
        val components = panel.components
        assertTrue("Should have components", components.isNotEmpty())
    }
    
    @Test
    fun testSuppressionListDisplay() {
        val service = project.service<MtlogProjectService>()
        service.state.suppressedDiagnostics = mutableListOf("MTLOG001", "MTLOG004", "MTLOG006")
        
        val dialog = SuppressionManagerDialog(project)
        val panel = dialog.createCenterPanel()
        
        assertNotNull("Panel should be created", panel)
        
        // Verify the correct number of diagnostics are displayed
        val checkboxes = findCheckboxes(panel)
        assertEquals("Should have 3 checkboxes", 3, checkboxes.size)
        
        // Verify checkboxes are initially selected
        for (checkbox in checkboxes) {
            assertTrue("Checkbox should be initially selected", checkbox.isSelected)
        }
        
        // Verify the text contains diagnostic IDs and descriptions
        val texts = checkboxes.map { it.text }
        assertTrue("Should contain MTLOG001", texts.any { it.contains("MTLOG001") })
        assertTrue("Should contain MTLOG004", texts.any { it.contains("MTLOG004") })
        assertTrue("Should contain MTLOG006", texts.any { it.contains("MTLOG006") })
        
        // Verify descriptions are included
        assertTrue("Should contain Template/argument description", 
                  texts.any { it.contains("Template/argument") })
        assertTrue("Should contain PascalCase description", 
                  texts.any { it.contains("PascalCase") })
        assertTrue("Should contain Error logging description", 
                  texts.any { it.contains("Error logging") })
    }
    
    @Test
    fun testUnsuppression() {
        val service = project.service<MtlogProjectService>()
        service.state.suppressedDiagnostics = mutableListOf("MTLOG001", "MTLOG004", "MTLOG006")
        
        val dialog = TestableSuppressionManagerDialog(project)
        val panel = dialog.createCenterPanel()
        
        // Uncheck MTLOG004
        val checkboxes = dialog.getTestCheckboxes()
        val mtlog004Checkbox = checkboxes["MTLOG004"]
        assertNotNull("Should have MTLOG004 checkbox", mtlog004Checkbox)
        mtlog004Checkbox!!.isSelected = false
        
        // Simulate OK action
        dialog.doOKAction()
        
        // Verify MTLOG004 was removed from suppressed list
        val suppressed = service.state.suppressedDiagnostics
        assertEquals(2, suppressed.size)
        assertTrue(suppressed.contains("MTLOG001"))
        assertFalse(suppressed.contains("MTLOG004"))
        assertTrue(suppressed.contains("MTLOG006"))
    }
    
    @Test
    fun testKeepAllSuppressed() {
        val service = project.service<MtlogProjectService>()
        val initialList = mutableListOf("MTLOG001", "MTLOG004", "MTLOG006")
        service.state.suppressedDiagnostics = initialList
        
        val dialog = TestableSuppressionManagerDialog(project)
        dialog.createCenterPanel()
        
        // Keep all checkboxes selected (default state)
        dialog.doOKAction()
        
        // Verify nothing changed
        val suppressed = service.state.suppressedDiagnostics
        assertEquals(3, suppressed.size)
        assertEquals(initialList, suppressed)
    }
    
    @Test
    fun testUnsuppressAll() {
        val service = project.service<MtlogProjectService>()
        service.state.suppressedDiagnostics = mutableListOf("MTLOG001", "MTLOG004", "MTLOG006")
        
        val dialog = TestableSuppressionManagerDialog(project)
        dialog.createCenterPanel()
        
        // Uncheck all
        for (checkbox in dialog.getTestCheckboxes().values) {
            checkbox.isSelected = false
        }
        
        // Simulate OK action
        dialog.doOKAction()
        
        // Verify all were unsuppressed
        val suppressed = service.state.suppressedDiagnostics
        assertTrue("Should have no suppressions", suppressed.isEmpty())
    }
    
    private fun findCheckboxes(component: java.awt.Component): List<JCheckBox> {
        val checkboxes = mutableListOf<JCheckBox>()
        
        if (component is JCheckBox) {
            checkboxes.add(component)
        }
        
        if (component is java.awt.Container) {
            for (child in component.components) {
                checkboxes.addAll(findCheckboxes(child))
            }
        }
        
        return checkboxes
    }
    
    /**
     * Testable version of the dialog that exposes internal state.
     */
    private class TestableSuppressionManagerDialog(project: com.intellij.openapi.project.Project) 
        : SuppressionManagerDialog(project) {
        
        fun getTestCheckboxes(): Map<String, javax.swing.JCheckBox> {
            // Access the protected checkboxes field
            return checkboxes
        }
        
        override fun show() {
            // Don't actually show the dialog in tests
        }
    }
}
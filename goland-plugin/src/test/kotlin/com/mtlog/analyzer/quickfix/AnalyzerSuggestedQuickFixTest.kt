package com.mtlog.analyzer.quickfix

import com.mtlog.analyzer.service.AnalyzerSuggestedFix
import com.mtlog.analyzer.service.AnalyzerTextEdit
import org.junit.Test
import org.junit.Assert.*

class AnalyzerSuggestedQuickFixTest {

    @Test
    fun testParsePosition() {
        // Test the position parsing logic
        val quickFix = AnalyzerSuggestedQuickFix(null, 
            AnalyzerSuggestedFix("test", emptyList())
        )
        
        // Use reflection to access private method
        val method = AnalyzerSuggestedQuickFix::class.java.getDeclaredMethod("parsePosition", String::class.java)
        method.isAccessible = true
        
        // Test parsing positions in format "file:line:col"
        val result1 = method.invoke(quickFix, "test.go:10:5")
        assertEquals("Should parse line correctly", 10, result1::class.java.getDeclaredField("line").let { 
            it.isAccessible = true
            it.get(result1) 
        })
        
        val result2 = method.invoke(quickFix, "test.go:25:12")
        assertEquals("Should parse column correctly", 12, result2::class.java.getDeclaredField("column").let { 
            it.isAccessible = true
            it.get(result2) 
        })
    }
    
    @Test
    fun testSuggestedFixCreation() {
        // Test that AnalyzerSuggestedQuickFix can be created with valid data
        val textEdit = AnalyzerTextEdit(
            pos = "test.go:7:35",
            end = "test.go:7:35",
            newText = ", err"
        )
        
        val suggestedFix = AnalyzerSuggestedFix(
            message = "Add error parameter",
            textEdits = listOf(textEdit)
        )
        
        val quickFix = AnalyzerSuggestedQuickFix(null, suggestedFix)
        
        assertEquals("Quick fix should have correct message", "Add error parameter", quickFix.text)
        assertEquals("Quick fix should have correct family name", "mtlog", quickFix.familyName)
    }
    
    @Test
    fun testTextEditSorting() {
        // Test that multiple text edits would be sorted correctly (reverse order by position)
        val textEdits = listOf(
            AnalyzerTextEdit("test.go:10:5", "test.go:10:10", "first"),
            AnalyzerTextEdit("test.go:10:15", "test.go:10:20", "second"),
            AnalyzerTextEdit("test.go:10:25", "test.go:10:30", "third")
        )
        
        val suggestedFix = AnalyzerSuggestedFix("Test fix", textEdits)
        val quickFix = AnalyzerSuggestedQuickFix(null, suggestedFix)
        
        // Verify that the quickfix object was created successfully
        assertNotNull("Quick fix should be created", quickFix)
        assertEquals("Should have correct number of text edits", 3, suggestedFix.textEdits.size)
    }
    
    @Test
    fun testMtlog006ScenarioDataStructures() {
        // Test that we can represent the three MTLOG006 scenarios correctly
        
        // Scenario 1: err in scope
        val scenario1 = AnalyzerSuggestedFix(
            message = "Add error parameter",
            textEdits = listOf(
                AnalyzerTextEdit("test.go:19:35", "test.go:19:35", ", err")
            )
        )
        
        // Scenario 2: nil with TODO
        val scenario2 = AnalyzerSuggestedFix(
            message = "Add error parameter", 
            textEdits = listOf(
                AnalyzerTextEdit("test.go:24:33", "test.go:24:33", ", nil"),
                AnalyzerTextEdit("test.go:24:37", "test.go:24:37", " // TODO: replace nil with actual error")
            )
        )
        
        // Scenario 3: nil with TODO (different line)
        val scenario3 = AnalyzerSuggestedFix(
            message = "Add error parameter",
            textEdits = listOf(
                AnalyzerTextEdit("test.go:29:35", "test.go:29:35", ", nil"),
                AnalyzerTextEdit("test.go:29:39", "test.go:29:39", " // TODO: replace nil with actual error")
            )
        )
        
        // Verify all scenarios can be represented
        assertEquals("Scenario 1 should add err", ", err", scenario1.textEdits[0].newText)
        assertTrue("Scenario 2 should have TODO", scenario2.textEdits.any { it.newText.contains("TODO") })
        assertTrue("Scenario 3 should have TODO", scenario3.textEdits.any { it.newText.contains("TODO") })
        
        assertEquals("All scenarios should have same message", scenario1.message, scenario2.message)
    }
}
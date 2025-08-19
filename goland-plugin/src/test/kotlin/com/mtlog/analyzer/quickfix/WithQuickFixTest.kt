package com.mtlog.analyzer.quickfix

import com.intellij.openapi.components.service
import com.mtlog.analyzer.integration.MtlogIntegrationTestBase
import com.mtlog.analyzer.service.AnalyzerDiagnostic
import com.mtlog.analyzer.service.MtlogProjectService
import java.io.File

class WithQuickFixTest : MtlogIntegrationTestBase() {
    
    override fun shouldSetupRealTestProject(): Boolean = true
    
    fun testMTLOG009_AddEmptyStringValueQuickFix() {
        val originalContent = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.With("key1", "value1", "key2")
            }
        """.trimIndent()
        
        createGoFile("main.go", originalContent)
        
        val diagnostics = runRealAnalyzerWithQuickFixes()
        
        // Print what we got for debugging
        println("Test found ${diagnostics.size} diagnostics")
        diagnostics.forEach { d ->
            if (d is AnalyzerDiagnostic) {
                println("  - ${d.message}")
            }
        }
        
        // First check we got ANY diagnostics
        assertTrue("Expected to find diagnostics, but got none. Diagnostics list: $diagnostics", diagnostics.isNotEmpty())
        assertTrue("Expected to find at least one diagnostic, found ${diagnostics.size}", diagnostics.size >= 1)
        
        val oddArgsError = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .find { it.message.contains("MTLOG009") || it.message.contains("even number") }
        
        assertNotNull("Should detect odd number of arguments. Found diagnostics: ${diagnostics.map { (it as? AnalyzerDiagnostic)?.message }}", oddArgsError)
        assertTrue("Should have suggested fixes", oddArgsError!!.suggestedFixes.isNotEmpty())
        
        // Find the "Add empty string value" fix
        val addValueFix = oddArgsError.suggestedFixes.find { 
            it.message.contains("Add empty string") || it.message.contains("empty string value")
        }
        assertNotNull("Should have 'Add empty string value' fix", addValueFix)
        
        // Apply the fix
        applyQuickFixAndCheck(addValueFix!!) { content ->
            assertTrue("Should add empty string after last key", 
                content.contains("""log.With("key1", "value1", "key2", "")"""))
        }
    }
    
    fun testMTLOG009_RemoveDanglingKeyQuickFix() {
        val originalContent = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.With("key1", "value1", "key2", "value2", "key3")
            }
        """.trimIndent()
        
        createGoFile("main.go", originalContent)
        
        val diagnostics = runRealAnalyzerWithQuickFixes()
        val oddArgsError = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .find { it.message.contains("MTLOG009") }
        
        assertNotNull("Should detect odd number of arguments", oddArgsError)
        
        // Find the "Remove the dangling key" fix
        val removeKeyFix = oddArgsError!!.suggestedFixes.find { 
            it.message.contains("Remove") && it.message.contains("dangling")
        }
        assertNotNull("Should have 'Remove dangling key' fix", removeKeyFix)
        
        // Apply the fix
        applyQuickFixAndCheck(removeKeyFix!!) { content ->
            assertTrue("Should remove the dangling key", 
                content.contains("""log.With("key1", "value1", "key2", "value2")"""))
            assertFalse("Should not contain key3", content.contains("key3"))
        }
    }
    
    fun testMTLOG010_ConvertNumberToStringQuickFix() {
        val originalContent = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.With(123, "value")
                log.With(456.78, "another")
            }
        """.trimIndent()
        
        createGoFile("main.go", originalContent)
        
        val diagnostics = runRealAnalyzerWithQuickFixes()
        val nonStringErrors = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG010") }
        
        assertTrue("Should detect non-string keys", nonStringErrors.size >= 2)
        
        // Check integer conversion
        val intKeyError = nonStringErrors.find { it.lineNumber == 7 }
        assertNotNull("Should detect integer key", intKeyError)
        
        val convertIntFix = intKeyError!!.suggestedFixes.find {
            it.message.contains("Convert") && it.message.contains("123")
        }
        assertNotNull("Should have conversion fix for integer", convertIntFix)
        
        // Apply the fix for integer
        applyQuickFixAndCheck(convertIntFix!!) { content ->
            assertTrue("Should convert 123 to string", 
                content.contains("""log.With("123", "value")"""))
        }
        
        // Check float conversion
        val floatKeyError = nonStringErrors.find { it.lineNumber == 8 }
        assertNotNull("Should detect float key", floatKeyError)
        
        val convertFloatFix = floatKeyError!!.suggestedFixes.find {
            it.message.contains("Convert") && it.message.contains("456.78")
        }
        assertNotNull("Should have conversion fix for float", convertFloatFix)
    }
    
    fun testMTLOG010_UseVariableAsValueQuickFix() {
        val originalContent = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                userId := 123
                log.With(userId, "someValue")
            }
        """.trimIndent()
        
        createGoFile("main.go", originalContent)
        
        val diagnostics = runRealAnalyzerWithQuickFixes()
        val nonStringError = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .find { it.message.contains("MTLOG010") && it.message.contains("userId") }
        
        assertNotNull("Should detect non-string variable key", nonStringError)
        
        val useAsValueFix = nonStringError!!.suggestedFixes.find {
            it.message.contains("Use") && it.message.contains("as value")
        }
        assertNotNull("Should have 'use as value' fix", useAsValueFix)
        
        // Apply the fix
        applyQuickFixAndCheck(useAsValueFix!!) { content ->
            assertTrue("Should use variable as value with string key", 
                content.contains("""log.With("user_id", userId""") ||
                content.contains("""log.With("userId", userId"""))
        }
    }
    
    fun testMTLOG012_RenameReservedPropertyQuickFix() {
        val service = project.service<MtlogProjectService>()
        val originalFlags = service.state.analyzerFlags.toMutableList()
        
        try {
            // Enable reserved property checking
            service.state.analyzerFlags.clear()
            service.state.analyzerFlags.add("-check-reserved")
            
            val originalContent = """
                package main
                
                import "github.com/willibrandon/mtlog"
                
                func main() {
                    log := mtlog.New()
                    log.With("Message", "my custom message")
                    log.With("Timestamp", "2024-01-01")
                }
            """.trimIndent()
            
            createGoFile("main.go", originalContent)
            
            val diagnostics = runRealAnalyzerWithQuickFixes()
            val reservedWarnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
                .filter { it.message.contains("MTLOG012") }
            
            assertTrue("Should detect reserved properties", reservedWarnings.isNotEmpty())
            
            // Check Message rename
            val messageWarning = reservedWarnings.find { it.message.contains("Message") }
            assertNotNull("Should detect 'Message' as reserved", messageWarning)
            
            val renameFix = messageWarning!!.suggestedFixes.firstOrNull()
            assertNotNull("Should have rename suggestion", renameFix)
            
            // Apply the fix
            applyQuickFixAndCheck(renameFix!!) { content ->
                assertTrue("Should rename Message to UserMessage or CustomMessage", 
                    content.contains(""""UserMessage"""") || 
                    content.contains(""""CustomMessage""""))
                assertFalse("Should not contain original 'Message'", 
                    content.contains(""""Message""""))
            }
            
        } finally {
            // Restore original flags
            service.state.analyzerFlags.clear()
            service.state.analyzerFlags.addAll(originalFlags)
        }
    }
    
    fun testMTLOG013_RemoveEmptyKeyPairQuickFix() {
        val originalContent = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.With("key1", "value1", "", "emptyKeyValue", "key2", "value2")
            }
        """.trimIndent()
        
        createGoFile("main.go", originalContent)
        
        val diagnostics = runRealAnalyzerWithQuickFixes()
        val emptyKeyError = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .find { it.message.contains("MTLOG013") || (it.message.contains("empty") && it.message.contains("key")) }
        
        assertNotNull("Should detect empty key", emptyKeyError)
        assertTrue("Should have suggested fix", emptyKeyError!!.suggestedFixes.isNotEmpty())
        
        val removeFix = emptyKeyError.suggestedFixes.find {
            it.message.contains("Remove empty key-value pair")
        }
        assertNotNull("Should have remove fix", removeFix)
        
        // Apply the fix
        applyQuickFixAndCheck(removeFix!!) { content ->
            assertTrue("Should remove empty key-value pair", 
                content.contains("""log.With("key1", "value1", "key2", "value2")"""))
            assertFalse("Should not contain empty string key", 
                content.contains(""", "", "emptyKeyValue","""))
        }
    }
    
    fun testMultipleQuickFixesInSingleCall() {
        val originalContent = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // Multiple issues in one call
                log.With(
                    123, "value1",        // Non-string key
                    "", "value2",         // Empty key
                    "key3")               // Odd number
            }
        """.trimIndent()
        
        createGoFile("main.go", originalContent)
        
        val diagnostics = runRealAnalyzerWithQuickFixes()
        val allProblems = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("MTLOG") }
        
        // Should detect all three issues
        assertTrue("Should detect multiple issues", allProblems.size >= 3)
        
        // Verify each has appropriate fixes
        val nonStringFix = allProblems.find { it.message.contains("MTLOG010") }
        assertNotNull("Should detect non-string key", nonStringFix)
        assertTrue("Non-string key should have fixes", nonStringFix!!.suggestedFixes.isNotEmpty())
        
        val emptyKeyFix = allProblems.find { it.message.contains("MTLOG013") }
        assertNotNull("Should detect empty key", emptyKeyFix)
        assertTrue("Empty key should have fixes", emptyKeyFix!!.suggestedFixes.isNotEmpty())
        
        val oddArgsFix = allProblems.find { it.message.contains("MTLOG009") }
        assertNotNull("Should detect odd arguments", oddArgsFix)
        assertTrue("Odd arguments should have fixes", oddArgsFix!!.suggestedFixes.isNotEmpty())
    }
    
    fun testQuickFixPreservesFormatting() {
        val originalContent = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                
                // Multiline With() call with formatting
                log.With(
                    "key1", "value1",
                    "key2", "value2",
                    "key3")  // Missing value
            }
        """.trimIndent()
        
        createGoFile("main.go", originalContent)
        
        val diagnostics = runRealAnalyzerWithQuickFixes()
        val oddArgsError = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .find { it.message.contains("MTLOG009") }
        
        assertNotNull("Should detect odd arguments", oddArgsError)
        
        val addValueFix = oddArgsError!!.suggestedFixes.find {
            it.message.contains("Add empty string")
        }
        assertNotNull("Should have add value fix", addValueFix)
        
        // Apply fix and check formatting is preserved
        applyQuickFixAndCheck(addValueFix!!) { content ->
            // Should maintain multiline format
            assertTrue("Should preserve multiline format", content.contains("log.With(\n"))
            assertTrue("Should add empty string to key3", 
                content.contains(""""key3", """"") || 
                content.contains(""""key3", ""  // Missing value"""))
        }
    }
    
    private fun applyQuickFixAndCheck(
        fix: com.mtlog.analyzer.service.AnalyzerSuggestedFix,
        check: (String) -> Unit
    ) {
        // Apply all text edits from the fix
        val file = File(realProjectDir, "main.go")
        var content = file.readText()
        
        // Sort edits by position (reverse order to not affect positions)
        val sortedEdits = fix.textEdits.sortedByDescending { it.pos }
        
        for (edit in sortedEdits) {
            // Parse positions and apply edit
            val startPos = parseFilePosition(edit.pos)
            val endPos = parseFilePosition(edit.end)
            
            // Convert line:column to offset
            val lines = content.lines()
            val startOffset = getOffset(lines, startPos.first, startPos.second)
            val endOffset = getOffset(lines, endPos.first, endPos.second)
            
            // Apply the edit
            content = content.substring(0, startOffset) + 
                      edit.newText + 
                      content.substring(endOffset)
        }
        
        // Write back and verify
        file.writeText(content)
        
        // Run the check
        check(content)
        
        // Verify no more diagnostics of this type
        val afterDiagnostics = runRealAnalyzer()
        val remainingProblems = afterDiagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains(fix.message.substringBefore(" ")) }
        
        assertTrue("Fix should resolve the issue", remainingProblems.isEmpty())
    }
    
    private fun parseFilePosition(pos: String): Pair<Int, Int> {
        // Format: "file.go:line:column" or "C:\path\file.go:line:column" (Windows)
        val colonCount = pos.count { it == ':' }
        
        return if (colonCount >= 3 && pos.length > 2 && pos[1] == ':') {
            // Windows path with drive letter (e.g., "C:\path\file.go:7:2")
            val lastColonIndex = pos.lastIndexOf(':')
            val secondLastColonIndex = pos.lastIndexOf(':', lastColonIndex - 1)
            val line = pos.substring(secondLastColonIndex + 1, lastColonIndex).toIntOrNull() ?: 1
            val column = pos.substring(lastColonIndex + 1).toIntOrNull() ?: 1
            Pair(line, column)
        } else if (colonCount >= 2) {
            // Unix-style path (e.g., "/path/file.go:7:2")
            val parts = pos.split(":")
            val line = parts[parts.size - 2].toIntOrNull() ?: 1
            val column = parts[parts.size - 1].toIntOrNull() ?: 1
            Pair(line, column)
        } else {
            Pair(1, 1)
        }
    }
    
    private fun getOffset(lines: List<String>, line: Int, column: Int): Int {
        var offset = 0
        for (i in 0 until line - 1) {
            if (i < lines.size) {
                offset += lines[i].length + 1 // +1 for newline
            }
        }
        if (line - 1 < lines.size) {
            offset += minOf(column - 1, lines[line - 1].length)
        }
        return offset
    }
}
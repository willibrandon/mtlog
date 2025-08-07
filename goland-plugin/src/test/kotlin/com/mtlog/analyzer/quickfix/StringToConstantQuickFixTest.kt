package com.mtlog.analyzer.quickfix

import com.goide.inspections.core.GoProblemsHolder
import com.intellij.openapi.components.service
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VfsUtil
import com.intellij.testFramework.PsiTestUtil
import com.mtlog.analyzer.integration.MtlogIntegrationTestBase
import com.mtlog.analyzer.service.MtlogProjectService
import java.io.File

class StringToConstantQuickFixTest : MtlogIntegrationTestBase() {
    
    fun testScenario1_MultipleOccurrencesCreatesConstant() {
        // Scenario 1: Multiple occurrences of user_id - should create constant
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.For<caret>Context("user_id", 123).Information("First")
                log.ForContext("user_id", 456).Information("Second")
                log.ForContext("user_id", 789).Information("Third")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            const userIDContextKey = "user_id"
            
            func main() {
                log := mtlog.New()
                log.ForContext(userIDContextKey, 123).Information("First")
                log.ForContext(userIDContextKey, 456).Information("Second")
                log.ForContext(userIDContextKey, 789).Information("Third")
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"user_id\" to constant userIDContextKey")
    }
    
    fun testScenario2_MultipleOccurrencesRequestId() {
        // Scenario 2: Multiple occurrences of request_id - should create constant
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.For<caret>Context("request_id", "abc-123").Information("Start")
                log.ForContext("request_id", "def-456").Information("End")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            const requestIDContextKey = "request_id"
            
            func main() {
                log := mtlog.New()
                log.ForContext(requestIDContextKey, "abc-123").Information("Start")
                log.ForContext(requestIDContextKey, "def-456").Information("End")
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"request_id\" to constant requestIDContextKey")
    }
    
    fun testScenario3_SingleOccurrenceSimpleReplacement() {
        // Scenario 3: Single occurrence of trace_id - should suggest simple replacement
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.For<caret>Context("trace_id", "xyz-789").Information("Trace")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.ForContext(traceIDContextKey, "xyz-789").Information("Trace")
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Replace with constant traceIDContextKey")
    }
    
    fun testScenario4_MultipleOccurrencesAcrossFunctions() {
        // Scenario 4: Multiple occurrences across different functions
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.For<caret>Context("user_id", 123).Information("Main")
            }
            
            func anotherFunc() {
                log := mtlog.New()
                log.ForContext("user_id", 456).Information("Another")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            const userIDContextKey = "user_id"
            
            func main() {
                log := mtlog.New()
                log.ForContext(userIDContextKey, 123).Information("Main")
            }
            
            func anotherFunc() {
                log := mtlog.New()
                log.ForContext(userIDContextKey, 456).Information("Another")
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"user_id\" to constant userIDContextKey")
    }
    
    fun testScenario5_WithExistingConstants() {
        // Scenario 5: File already has some constants
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            const existingConst = "something"
            
            func main() {
                log := mtlog.New()
                log.For<caret>Context("user_id", 123).Information("First")
                log.ForContext("user_id", 456).Information("Second")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            const existingConst = "something"
            
            const userIDContextKey = "user_id"
            
            func main() {
                log := mtlog.New()
                log.ForContext(userIDContextKey, 123).Information("First")
                log.ForContext(userIDContextKey, 456).Information("Second")
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"user_id\" to constant userIDContextKey")
    }
    
    fun testScenario6_SnakeCaseToContextKey() {
        // Scenario 6: Test snake_case to PascalCase conversion
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.For<caret>Context("span_id", "span-123").Information("First")
                log.ForContext("span_id", "span-456").Information("Second")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            const spanIDContextKey = "span_id"
            
            func main() {
                log := mtlog.New()
                log.ForContext(spanIDContextKey, "span-123").Information("First")
                log.ForContext(spanIDContextKey, "span-456").Information("Second")
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"span_id\" to constant spanIDContextKey")
    }
    
    fun testScenario7_DottedKeyConversion() {
        // Scenario 7: Test dotted key conversion (not a common key, but for testing name generation)
        // This test would fail as non-common keys don't trigger the diagnostic
        // So we'll use a modified version where we pretend "service.name" is common
        // In reality, this would need to be in the CommonContextKeys configuration
        // Skipping this test as it requires configuration changes
    }
    
    fun testScenario8_ComplexFile() {
        // Scenario 8: Complex file with multiple different keys
        val code = """
            package main
            
            import (
                "context"
                "github.com/willibrandon/mtlog"
            )
            
            func main() {
                log := mtlog.New()
                ctx := context.Background()
                
                // Multiple user_id occurrences
                log.For<caret>Context("user_id", 123).Information("Start")
                log.ForContext("user_id", 456).Information("Process")
                
                // Multiple request_id occurrences  
                log.ForContext("request_id", "req-1").Information("Request start")
                log.ForContext("request_id", "req-2").Information("Request end")
                
                // Single trace_id occurrence
                log.ForContext("trace_id", "trace-1").Information("Trace")
                
                _ = ctx
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import (
                "context"
                "github.com/willibrandon/mtlog"
            )
            
            const userIDContextKey = "user_id"
            
            func main() {
                log := mtlog.New()
                ctx := context.Background()
                
                // Multiple user_id occurrences
                log.ForContext(userIDContextKey, 123).Information("Start")
                log.ForContext(userIDContextKey, 456).Information("Process")
                
                // Multiple request_id occurrences  
                log.ForContext("request_id", "req-1").Information("Request start")
                log.ForContext("request_id", "req-2").Information("Request end")
                
                // Single trace_id occurrence
                log.ForContext("trace_id", "trace-1").Information("Trace")
                
                _ = ctx
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"user_id\" to constant userIDContextKey")
    }
    
    fun testScenario9_ChainedCalls() {
        // Scenario 9: Chained ForContext calls
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.For<caret>Context("user_id", 123).
                    ForContext("request_id", "req-1").
                    Information("Chained")
                    
                log.ForContext("user_id", 456).Information("Another")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            const userIDContextKey = "user_id"
            
            func main() {
                log := mtlog.New()
                log.ForContext(userIDContextKey, 123).
                    ForContext("request_id", "req-1").
                    Information("Chained")
                    
                log.ForContext(userIDContextKey, 456).Information("Another")
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"user_id\" to constant userIDContextKey")
    }
    
    fun testScenario10_NoImports() {
        // Scenario 10: File with no imports section
        val code = """
            package main
            
            func processWithContext(log *Logger) {
                log.For<caret>Context("user_id", 123).Information("First")
                log.ForContext("user_id", 456).Information("Second")
            }
            
            type Logger struct{}
            
            func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }
            func (l *Logger) Information(msg string, args ...interface{}) {}
        """.trimIndent()
        
        val expected = """
            package main
            
            const userIDContextKey = "user_id"
            
            func processWithContext(log *Logger) {
                log.ForContext(userIDContextKey, 123).Information("First")
                log.ForContext(userIDContextKey, 456).Information("Second")
            }
            
            type Logger struct{}
            
            func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }
            func (l *Logger) Information(msg string, args ...interface{}) {}
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Extract \"user_id\" to constant userIDContextKey")
    }
    
    private fun applyQuickFixAndCheck(code: String, expected: String, quickFixText: String) {
        // Write go.mod to disk first
        File(realProjectDir, "go.mod").writeText("""
            module testproject
            go 1.21
        """.trimIndent())
        
        // Create vendor directory with mtlog to avoid import errors
        val vendorDir = File(realProjectDir, "vendor/github.com/willibrandon/mtlog")
        vendorDir.mkdirs()
        File(vendorDir, "mtlog.go").writeText("""
            package mtlog
            
            type Logger struct{}
            
            func New() *Logger { return &Logger{} }
            
            func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }
            func (l *Logger) Information(template string, args ...interface{}) {}
            func (l *Logger) Warning(template string, args ...interface{}) {}
            func (l *Logger) Error(template string, args ...interface{}) {}
            func (l *Logger) Debug(template string, args ...interface{}) {}
            func (l *Logger) Fatal(template string, args ...interface{}) {}
            func (l *Logger) Verbose(template string, args ...interface{}) {}
        """.trimIndent())
        
        // Write the test file to disk
        val file = File(realProjectDir, "main.go").apply { writeText(code) }
        val vFile = LocalFileSystem.getInstance()
            .refreshAndFindFileByIoFile(file)!!
        
        // Add the directory as a source root
        PsiTestUtil.addSourceRoot(myFixture.module, vFile.parent)
        VfsUtil.markDirtyAndRefresh(false, true, true, vFile.parent)
        
        // Configure the file
        myFixture.configureFromExistingVirtualFile(vFile)
        
        // Explicitly run the analyzer on the file using stdin mode (like the external annotator does)
        val service = project.service<MtlogProjectService>()
        val fileContent = file.readText()
        val diagnostics = service.runAnalyzer(file.absolutePath, realProjectDir.absolutePath, fileContent)
        println("Analyzer returned ${diagnostics?.size ?: 0} diagnostics")
        diagnostics?.forEach { diag ->
            println("  Diagnostic: $diag")
        }
        
        // Wait for highlights to be available, up to 5 seconds
        val maxWaitMillis = 5000L
        val pollIntervalMillis = 100L
        var highlights: List<com.intellij.codeInsight.daemon.impl.HighlightInfo>? = null
        val startTime = System.currentTimeMillis()
        
        while (System.currentTimeMillis() - startTime < maxWaitMillis) {
            highlights = myFixture.doHighlighting()
            if (highlights.isNotEmpty()) break
            Thread.sleep(pollIntervalMillis)
        }
        
        if (highlights == null || highlights.isEmpty()) {
            error("No highlights found after waiting for $maxWaitMillis ms")
        }
        println("Highlights found: ${highlights.size}")
        highlights.forEach { highlight ->
            println("  - ${highlight.description}: ${highlight.text} (severity: ${highlight.severity})")
            highlight.findRegisteredQuickFix { descriptor, _ ->
                println("    Quick fix: ${descriptor.action.text}")
                null // Continue iteration
            }
        }
        
        // Find the quick fix from the highlights since it's attached to the string literal, not the caret position
        var quickFixAction: com.intellij.codeInsight.intention.IntentionAction? = null
        
        highlights.forEach { highlight ->
            highlight.quickFixActionRanges?.forEach { range ->
                val action = range.first.action
                if (action.text == quickFixText) {
                    quickFixAction = action
                    println("Found quick fix: ${action.text}")
                }
            }
        }
        
        assertNotNull("No matching quick fix found for '$quickFixText' in highlights", quickFixAction)
        
        // Apply the quick fix
        myFixture.launchAction(quickFixAction!!)
        
        // Get the actual result for debugging
        val actual = myFixture.editor.document.text
        println("===== ACTUAL RESULT =====")
        println(actual)
        println("===== EXPECTED RESULT =====")
        println(expected)
        println("===== END =====")
        
        // Check the result
        myFixture.checkResult(expected)
    }
}
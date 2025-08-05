package com.mtlog.analyzer.quickfix

import com.goide.inspections.core.GoProblemsHolder
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VfsUtil
import com.intellij.testFramework.PsiTestUtil
import com.mtlog.analyzer.integration.MtlogIntegrationTestBase
import java.io.File

class MissingErrorQuickFixTest : MtlogIntegrationTestBase() {
    
    
    fun testScenario1_ErrorFromFunctionReturn() {
        // Scenario 1: Error from function return - should find 'err'
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                result, err := doSomething()
                if err != nil {
                    log.E<caret>rror("Something failed")
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                result, err := doSomething()
                if err != nil {
                    log.Error("Something failed", err)
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario2_MultipleReturnWithError() {
        // Scenario 2: Multiple return with error - should find 'err'
        val code = """
            package main
            
            import (
                "net"
                "github.com/willibrandon/mtlog"
            )
            
            func main() {
                log := mtlog.New()
                conn, err := net.Dial("tcp", "localhost:8080")
                if err != nil {
                    log.E<caret>rror("Connection failed")
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import (
                "net"
                "github.com/willibrandon/mtlog"
            )
            
            func main() {
                log := mtlog.New()
                conn, err := net.Dial("tcp", "localhost:8080")
                if err != nil {
                    log.Error("Connection failed", err)
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario3a_VariableNamedE() {
        // Scenario 3a: Variable named 'e' - should find 'e'
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                result, e := doSomething()
                if e != nil {
                    log.E<caret>rror("Operation failed")
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                result, e := doSomething()
                if e != nil {
                    log.Error("Operation failed", e)
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario3b_VariableNamedMyErr() {
        // Scenario 3b: Variable named 'myErr' - should find 'myErr'
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                result, myErr := doSomething()
                if myErr != nil {
                    log.E<caret>rror("Custom error occurred")
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                result, myErr := doSomething()
                if myErr != nil {
                    log.Error("Custom error occurred", myErr)
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario4_ErrorAsFunctionParameter() {
        // Scenario 4: Error as function parameter - should find 'err'
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func handleError(log *mtlog.Logger, err error) {
                if err != nil {
                    log.E<caret>rror("Error occurred")
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func handleError(log *mtlog.Logger, err error) {
                if err != nil {
                    log.Error("Error occurred", err)
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario5_NoErrorInScope() {
        // Scenario 5: No error in scope - should add 'nil' with TODO
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.E<caret>rror("Something bad happened")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Error("Something bad happened", nil) // TODO: replace nil with actual error
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario6_DifferentScopeError() {
        // Scenario 6: Error in different scope - should add 'nil' with TODO
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                {
                    _, err := doSomething()
                    if err != nil {
                        return
                    }
                }
                log.E<caret>rror("Later error")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                {
                    _, err := doSomething()
                    if err != nil {
                        return
                    }
                }
                log.Error("Later error", nil) // TODO: replace nil with actual error
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario7_ErrorInNestedBlock() {
        // Scenario 7: Error in nested block - should use parent error
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                _, err := doSomething()
                if err != nil {
                    if shouldLog() {
                        log.E<caret>rror("Nested error")
                    }
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                _, err := doSomething()
                if err != nil {
                    if shouldLog() {
                        log.Error("Nested error", err)
                    }
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario8_NamedReturnValue() {
        // Scenario 8: Named return value - should find 'err'
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func doWork(log *mtlog.Logger) (result string, err error) {
                result, err = performOperation()
                if err != nil {
                    log.E<caret>rror("Operation failed")
                    return
                }
                return result, nil
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func doWork(log *mtlog.Logger) (result string, err error) {
                result, err = performOperation()
                if err != nil {
                    log.Error("Operation failed", err)
                    return
                }
                return result, nil
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario9_ErrorInLoop() {
        // Scenario 9: Error in loop - should find 'err'
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                for i := 0; i < 10; i++ {
                    data, err := fetchData(i)
                    if err != nil {
                        log.E<caret>rror("Failed to fetch data")
                        continue
                    }
                    process(data)
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                for i := 0; i < 10; i++ {
                    data, err := fetchData(i)
                    if err != nil {
                        log.Error("Failed to fetch data", err)
                        continue
                    }
                    process(data)
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario10_Case1_HasErrorInScope() {
        // Scenario 10 Case 1: Has 'err' in scope from net.Dial
        val code = """
            package main
            
            import (
                "net"
                "github.com/willibrandon/mtlog"
            )
            
            func main() {
                log := mtlog.New()
                
                // Has 'err' in scope from net.Dial
                conn, err := net.Dial("tcp", "localhost:8080")
                if err != nil {
                    log.E<caret>rror("Connection failed")
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import (
                "net"
                "github.com/willibrandon/mtlog"
            )
            
            func main() {
                log := mtlog.New()
                
                // Has 'err' in scope from net.Dial
                conn, err := net.Dial("tcp", "localhost:8080")
                if err != nil {
                    log.Error("Connection failed", err)
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario10_Case2_DifferentLogicalContext() {
        // Scenario 10 Case 2: Different logical context
        val code = """
            package main
            
            import (
                "net"
                "github.com/willibrandon/mtlog"
            )
            
            func main() {
                log := mtlog.New()
                conn, err := net.Dial("tcp", "localhost:8080")
                if err != nil {
                    log.Error("Connection failed", err)
                }
                
                // Different block, no err
                if conn != nil {
                    log.E<caret>rror("Connection issue")
                }
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import (
                "net"
                "github.com/willibrandon/mtlog"
            )
            
            func main() {
                log := mtlog.New()
                conn, err := net.Dial("tcp", "localhost:8080")
                if err != nil {
                    log.Error("Connection failed", err)
                }
                
                // Different block, no err
                if conn != nil {
                    log.Error("Connection issue", nil) // TODO: replace nil with actual error
                }
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testScenario10_Case3_IgnoredError() {
        // Scenario 10 Case 3: Ignored error
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // Ignore error with blank identifier
                _, _ = doSomething()
                log.E<caret>rror("Ignored error case")
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // Ignore error with blank identifier
                _, _ = doSomething()
                log.Error("Ignored error case", nil) // TODO: replace nil with actual error
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
    }
    
    fun testWithExistingComment() {
        // Test adding error when line already has a comment
        val code = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.E<caret>rror("Something failed") // existing comment
            }
        """.trimIndent()
        
        val expected = """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Error("Something failed", nil) // existing comment
                // TODO: replace nil with actual error
            }
        """.trimIndent()
        
        applyQuickFixAndCheck(code, expected, "Add error parameter")
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
            
            func (l *Logger) Information(template string, args ...interface{}) {}
            func (l *Logger) Warning(template string, args ...interface{}) {}
            func (l *Logger) Error(template string, args ...interface{}) {}
            func (l *Logger) Debug(template string, args ...interface{}) {}
            func (l *Logger) Fatal(template string, args ...interface{}) {}
            func (l *Logger) Verbose(template string, args ...interface{}) {}
        """.trimIndent())
        
        // Create helpers.go with stub functions to make the code compile
        File(realProjectDir, "helpers.go").writeText("""
            package main
            
            import (
                "errors"
                "net"
            )
            
            func doSomething() (interface{}, error) {
                return nil, errors.New("test error")
            }
            
            func performOperation() (string, error) {
                return "", errors.New("test error")
            }
            
            func shouldLog() bool {
                return true
            }
            
            func fetchData(i int) (interface{}, error) {
                return nil, errors.New("test error")
            }
            
            func process(data interface{}) {
                // stub
            }
            
            // For scenario 10 case 2 - we need net.Dial to return something
            // but Go's net package already provides this
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
        
        // Force a full highlighting pass which includes external annotators
        val highlights = myFixture.doHighlighting()
        println("Highlights found: ${highlights.size}")
        highlights.forEach { highlight ->
            println("  - ${highlight.description}: ${highlight.text} (severity: ${highlight.severity})")
            highlight.quickFixActionRanges?.forEach { qf ->
                println("    Quick fix: ${qf.first.action.text}")
            }
        }
        
        // Get all available intentions to debug
        val allIntentions = myFixture.availableIntentions
        println("Available intentions at caret: ${allIntentions.map { it.text }}")
        
        // Find and apply the quick fix
        val intention = myFixture.findSingleIntention(quickFixText)
        myFixture.launchAction(intention)
        
        // Check the result
        myFixture.checkResult(expected)
    }
}
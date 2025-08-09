package com.mtlog.analyzer.quickfix

import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VfsUtil
import com.intellij.testFramework.PsiTestUtil
import com.mtlog.analyzer.integration.MtlogIntegrationTestBase
import java.io.File

class LogValueStubQuickFixTest : MtlogIntegrationTestBase() {
    
    fun testAddAtPrefixQuickFix() {
        val code = """
            package main
            
            import (
                "time"
                "github.com/willibrandon/mtlog"
            )
            
            type User struct {
                ID        int
                Username  string
                Email     string
                Password  string
                APIKey    string
                LastLogin time.Time
            }
            
            func main() {
                log := mtlog.New()
                user := User{
                    ID:        123,
                    Username:  "alice",
                    Email:     "alice@example.com",
                    Password:  "secret123",
                    APIKey:    "sk_live_abc123",
                    LastLogin: time.Now(),
                }
                log.Information("User logged in: {<caret>User}", user)
            }
        """.trimIndent()

        val expected = """
            package main
            
            import (
                "time"
                "github.com/willibrandon/mtlog"
            )
            
            type User struct {
                ID        int
                Username  string
                Email     string
                Password  string
                APIKey    string
                LastLogin time.Time
            }
            
            func main() {
                log := mtlog.New()
                user := User{
                    ID:        123,
                    Username:  "alice",
                    Email:     "alice@example.com",
                    Password:  "secret123",
                    APIKey:    "sk_live_abc123",
                    LastLogin: time.Now(),
                }
                log.Information("User logged in: {@User}", user)
            }
        """.trimIndent()

        applyQuickFixAndCheck(code, expected, "Add @ prefix to 'User' for capturing")
    }
    
    fun testGenerateLogValueStubForUser() {
        val code = """
            package main
            
            import (
                "time"
                "github.com/willibrandon/mtlog"
            )
            
            type User struct {
                ID        int
                Username  string
                Email     string
                Password  string
                APIKey    string
                LastLogin time.Time
            }
            
            func main() {
                log := mtlog.New()
                user := User{
                    ID:        123,
                    Username:  "alice",
                    Email:     "alice@example.com",
                    Password:  "secret123",
                    APIKey:    "sk_live_abc123",
                    LastLogin: time.Now(),
                }
                log.Information("User logged in: {<caret>User}", user)
            }
        """.trimIndent()

        // Note: The expected result should show the LogValue method being added
        // Since we can't predict the exact formatting/position, we'll check for key elements
        applyQuickFixAndCheckContains(
            code, 
            "Generate LogValue() method for User",
            listOf(
                "func (u User) LogValue() any",
                "\"ID\": u.ID",
                "\"Username\": u.Username",
                "// \"Password\": u.Password", // Should be commented out
                "// \"APIKey\": u.APIKey", // Should be commented out
                "TODO: Review - potentially sensitive field"
            )
        )
    }
    
    fun testGenerateLogValueStubForOrder() {
        val code = """
            package main
            
            import (
                "time"
                "github.com/willibrandon/mtlog"
            )
            
            type Order struct {
                ID         int
                CustomerID int
                Total      float64
                Status     string
                CreatedAt  time.Time
            }
            
            func main() {
                log := mtlog.New()
                order := Order{
                    ID:         456,
                    CustomerID: 123,
                    Total:      99.99,
                    Status:     "pending",
                    CreatedAt:  time.Now(),
                }
                log.Information("Order created: {<caret>Order}", order)
            }
        """.trimIndent()

        applyQuickFixAndCheckContains(
            code,
            "Generate LogValue() method for Order",
            listOf(
                "func (o Order) LogValue() any",
                "\"ID\": o.ID",
                "\"CustomerID\": o.CustomerID",
                "\"Total\": o.Total",
                "\"Status\": o.Status",
                "\"CreatedAt\": o.CreatedAt"
            )
        )
    }
    
    private fun applyQuickFixAndCheck(code: String, expected: String, quickFixText: String) {
        setupTestFile(code)
        
        // Force a full highlighting pass which includes external annotators
        val highlights = myFixture.doHighlighting()
        
        // Verify MTLOG005 diagnostic exists
        val hasMtlog005 = highlights.any { it.description?.contains("MTLOG005") == true }
        assertTrue("Should have MTLOG005 diagnostic", hasMtlog005)
        
        // Get all available intentions
        val allIntentions = myFixture.availableIntentions
        
        // Find the MTLOG005 highlight and its quick fixes
        val mtlog005Highlight = highlights.find { it.description?.contains("MTLOG005") == true }
        assertNotNull("Should find MTLOG005 highlight", mtlog005Highlight)
        
        // Try to find and apply the quick fix from the highlight
        var quickFixApplied = false
        mtlog005Highlight?.findRegisteredQuickFix { descriptor, range ->
            if (descriptor.action.text == quickFixText) {
                myFixture.launchAction(descriptor.action)
                quickFixApplied = true
                return@findRegisteredQuickFix descriptor.action
            }
            null
        }
        
        if (!quickFixApplied) {
            // Fallback to intention-based approach
            val intention = myFixture.findSingleIntention(quickFixText)
            myFixture.launchAction(intention)
        }
        
        // Check the result
        myFixture.checkResult(expected)
    }
    
    private fun applyQuickFixAndCheckContains(code: String, quickFixText: String, expectedContents: List<String>) {
        setupTestFile(code)
        
        // Force a full highlighting pass which includes external annotators
        val highlights = myFixture.doHighlighting()
        
        // Verify MTLOG005 diagnostic exists
        val hasMtlog005 = highlights.any { it.description?.contains("MTLOG005") == true }
        assertTrue("Should have MTLOG005 diagnostic", hasMtlog005)
        
        // Get all available intentions
        val allIntentions = myFixture.availableIntentions
        
        // Find the MTLOG005 highlight and its quick fixes
        val mtlog005Highlight = highlights.find { it.description?.contains("MTLOG005") == true }
        assertNotNull("Should find MTLOG005 highlight", mtlog005Highlight)
        
        // Try to find and apply the quick fix from the highlight
        var quickFixApplied = false
        mtlog005Highlight?.findRegisteredQuickFix { descriptor, range ->
            if (descriptor.action.text == quickFixText) {
                myFixture.launchAction(descriptor.action)
                quickFixApplied = true
                return@findRegisteredQuickFix descriptor.action
            }
            null
        }
        
        if (!quickFixApplied) {
            // Fallback to intention-based approach
            val intention = myFixture.findSingleIntention(quickFixText)
            myFixture.launchAction(intention)
        }
        
        // Check the result contains expected elements
        val result = myFixture.editor.document.text
        for (expectedContent in expectedContents) {
            assertTrue("Result should contain: $expectedContent", result.contains(expectedContent))
        }
    }
    
    private fun setupTestFile(code: String) {
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
        
        // Write the test file to disk
        val file = File(realProjectDir, "main.go").apply { writeText(code) }
        val vFile = LocalFileSystem.getInstance()
            .refreshAndFindFileByIoFile(file)!!
        
        // Add the directory as a source root
        PsiTestUtil.addSourceRoot(myFixture.module, vFile.parent)
        VfsUtil.markDirtyAndRefresh(false, true, true, vFile.parent)
        
        // Configure the file
        myFixture.configureFromExistingVirtualFile(vFile)
    }
}
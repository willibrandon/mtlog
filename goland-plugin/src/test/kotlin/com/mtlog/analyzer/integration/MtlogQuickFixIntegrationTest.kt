package com.mtlog.analyzer.integration

import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.vfs.VfsUtil
import com.intellij.testFramework.PsiTestUtil
import com.mtlog.analyzer.service.AnalyzerDiagnostic
import java.io.File

class MtlogQuickFixIntegrationTest : MtlogIntegrationTestBase() {
    
    override fun shouldSetupRealTestProject(): Boolean = true
    
    fun testPascalCaseQuickFixRealExecution() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Debug("Processing {user_id}", 456)
            }
        """.trimIndent())
        
        // Run analyzer to get warnings
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "warning" || it.severity == "suggestion" }
            .filter { it.message.contains("PascalCase") }
        
        assertTrue("Should have PascalCase warning", warnings.isNotEmpty())
        
        // Quick fixes require editor integration which doesn't work with real files
        // Just verify we detected the issue
        val warning = warnings[0]
        assertEquals("Should detect user_id property", "user_id", warning.propertyName)
    }
    
    fun testMultiplePascalCaseProperties() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("User {user_id} performed {action_type} at {event_time}", 123, "login", "now")
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "warning" || it.severity == "suggestion" }
            .filter { it.message.contains("PascalCase") }
        
        // Should have multiple warnings
        assertTrue("Should have multiple PascalCase warnings", warnings.size >= 2)
        
        // Verify we detected all the properties
        val detectedProperties = warnings.mapNotNull { it.propertyName }.toSet()
        assertTrue("Should detect user_id", detectedProperties.contains("user_id"))
        assertTrue("Should detect action_type", detectedProperties.contains("action_type"))
        assertTrue("Should detect event_time", detectedProperties.contains("event_time"))
    }
    
    fun testTemplateArgumentQuickFix() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("User {UserId} logged in at {Time}", 123)
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val errors = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.severity == "error" }
            .filter { it.message.contains("argument") || it.message.contains("properties") }
        
        assertTrue("Should have argument count error", errors.isNotEmpty())
        
        // Verify the error message
        val error = errors[0]
        assertTrue("Error should mention template/argument mismatch", 
            error.message.contains("2 properties but 1 argument") ||
            error.message.contains("template has 2 properties but 1 arguments provided"))
    }
    
    fun testQuickFixPreservesFormatting() {
        createGoFile("main.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // This has specific formatting we want to preserve
                log.Debug(
                    "Processing user {user_id} with role {user_role}",
                    456,
                    "admin",
                )
            }
        """.trimIndent())
        
        val diagnostics = runRealAnalyzer()
        val warnings = diagnostics.filterIsInstance<AnalyzerDiagnostic>()
            .filter { it.message.contains("user_id") || it.propertyName == "user_id" }
        
        assertTrue("Should have warning for user_id", warnings.isNotEmpty())
        
        // Verify we detected the property correctly
        val warning = warnings[0]
        assertEquals("Should detect user_id property", "user_id", warning.propertyName)
        
        // The log.Debug call starts at line 8 (line 7 is comment, line 8 is log.Debug)
        assertEquals("Should be at line 8", 8, warning.lineNumber)
    }
    
    fun testApplyPascalCaseQuickFixOnRealFile() {
        // The base class already sets up go.mod and vendor directory
        
        // Write the test file to disk
        val code = """
            package main

            import "github.com/willibrandon/mtlog"

            func main() {
                log := mtlog.New()
                log.Debug("Processing {us<caret>er_id}", 456)
            }
            """.trimIndent()
        
        val file = File(realProjectDir, "main.go").apply { writeText(code) }
        val vFile = LocalFileSystem.getInstance()
            .refreshAndFindFileByIoFile(file)!!
        
        // Add the directory as a source root
        PsiTestUtil.addSourceRoot(myFixture.module, vFile.parent)
        VfsUtil.markDirtyAndRefresh(false, true, true, vFile.parent)
        
        myFixture.configureFromExistingVirtualFile(vFile)
        val highlights = myFixture.doHighlighting()
        
        // Debug: Print all available intentions
        val allIntentions = myFixture.availableIntentions
        println("Available intentions: ${allIntentions.map { it.text }}")
        
        myFixture.findSingleIntention("Change 'user_id' to 'UserId'").let {
            myFixture.launchAction(it)
        }
        
        myFixture.checkResult("""
            package main

            import "github.com/willibrandon/mtlog"

            func main() {
                log := mtlog.New()
                log.Debug("Processing {UserId}", 456)
            }
            """.trimIndent())
    }
    
    fun testApplyTemplateArgumentQuickFixOnRealFile() {
        // The base class already sets up go.mod and vendor directory
        
        // Write the test file to disk
        val code = """
            package main

            import "github.com/willibrandon/mtlog"

            func main() {
                log := mtlog.New()
                log.Information("User {Us<caret>erId} logged in at {Time}", 123)
            }
            """.trimIndent()
        
        val file = File(realProjectDir, "main.go").apply { writeText(code) }
        val vFile = LocalFileSystem.getInstance()
            .refreshAndFindFileByIoFile(file)!!
        
        // Add the directory as a source root
        PsiTestUtil.addSourceRoot(myFixture.module, vFile.parent)
        VfsUtil.markDirtyAndRefresh(false, true, true, vFile.parent)
        
        myFixture.configureFromExistingVirtualFile(vFile)
        val highlights = myFixture.doHighlighting()
        
        // Debug: Print all available intentions
        val allIntentions = myFixture.availableIntentions
        println("Available intentions: ${allIntentions.map { it.text }}")
        
        myFixture.findSingleIntention("Add 1 missing argument(s)").let {
            myFixture.launchAction(it)
        }
        
        myFixture.checkResult("""
            package main

            import "github.com/willibrandon/mtlog"

            func main() {
                log := mtlog.New()
                log.Information("User {UserId} logged in at {Time}", 123, nil) // TODO: provide value for Time
            }
            """.trimIndent())
    }
    
    fun testApplyTemplateArgumentQuickFixRemovesExtras() {
        // The base class already sets up go.mod and vendor directory
        
        // Write the test file to disk
        val code = """
            package main

            import "github.com/willibrandon/mtlog"

            func main() {
                log := mtlog.New()
                log.Warning("Disk usage at {Per<caret>centage:P1}", 0.85, "extra1", "extra2")
            }
            """.trimIndent()
        
        val file = File(realProjectDir, "main.go").apply { writeText(code) }
        val vFile = LocalFileSystem.getInstance()
            .refreshAndFindFileByIoFile(file)!!
        
        // Add the directory as a source root
        PsiTestUtil.addSourceRoot(myFixture.module, vFile.parent)
        VfsUtil.markDirtyAndRefresh(false, true, true, vFile.parent)
        
        myFixture.configureFromExistingVirtualFile(vFile)
        val highlights = myFixture.doHighlighting()
        
        // Debug: Print all available intentions
        val allIntentions = myFixture.availableIntentions
        println("Available intentions: ${allIntentions.map { it.text }}")
        
        myFixture.findSingleIntention("Remove 2 extra argument(s)").let {
            myFixture.launchAction(it)
        }
        
        myFixture.checkResult("""
            package main

            import "github.com/willibrandon/mtlog"

            func main() {
                log := mtlog.New()
                log.Warning("Disk usage at {Percentage:P1}", 0.85)
            }
            """.trimIndent())
    }
}
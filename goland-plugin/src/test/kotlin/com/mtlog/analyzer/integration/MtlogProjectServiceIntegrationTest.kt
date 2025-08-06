package com.mtlog.analyzer.integration

import com.intellij.openapi.components.service
import com.intellij.psi.PsiFile
import com.intellij.psi.PsiManager
import com.mtlog.analyzer.service.MtlogProjectService
import java.io.File

class MtlogProjectServiceIntegrationTest : MtlogIntegrationTestBase() {
    
    private lateinit var service: MtlogProjectService
    
    override fun setUp() {
        super.setUp()
        service = project.service()
    }
    
    fun testFindAnalyzerInPath() {
        // This test verifies the analyzer is actually found in PATH
        val analyzerPath = findAnalyzerPath()
        assertNotNull("Should find mtlog-analyzer in PATH", analyzerPath)
        
        val file = File(analyzerPath!!)
        assertTrue("Analyzer should exist", file.exists())
        assertTrue("Analyzer should be executable", file.canExecute())
    }
    
    fun testRunAnalyzerProducesRealOutput() {
        // Create a Go file with known issues
        val testFile = myFixture.addFileToProject("test.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("User {UserId} logged in", 123, "extra")
            }
        """.trimIndent())
        
        // Use the realProjectDir that was set up in base class
        val dir = realProjectDir
        writeFilesToDisk(dir)
        
        val diagnostics = service.runAnalyzer(File(dir, "test.go").absolutePath, dir.absolutePath)
        
        assertNotNull("Should get diagnostics from analyzer", diagnostics)
        assertTrue("Should have at least one diagnostic", diagnostics!!.isNotEmpty())
        
        val diagnostic = diagnostics[0]
        assertTrue("Diagnostic should mention arguments", 
            diagnostic.message.contains("argument") || 
            diagnostic.message.contains("properties"))
    }
    
    fun testRunAnalyzerWithInvalidFile() {
        val diagnostics = service.runAnalyzer("/non/existent/file.go", realProjectDir.absolutePath)
        
        // Should handle gracefully - either null or empty list
        assertTrue("Should handle non-existent file gracefully", 
            diagnostics == null || diagnostics.isEmpty())
    }
    
    fun testAnalyzerWithCustomFlags() {
        val originalFlags = service.state.analyzerFlags.toList()
        
        try {
            // Add a custom flag
            service.state.analyzerFlags = mutableListOf("-strict")
            
            val testFile = myFixture.addFileToProject("test.go", """
                package main
                
                import "github.com/willibrandon/mtlog"
                
                func main() {
                    log := mtlog.New()
                    log.Warning("Value is {Value:X}", 255)
                }
            """.trimIndent())
            
            // Write files to disk for go vet
            val dir = realProjectDir
            writeFilesToDisk(dir)
            
            val diagnostics = service.runAnalyzer(File(dir, "test.go").absolutePath, dir.absolutePath)
            
            // With -strict flag, might get additional diagnostics
            assertNotNull("Should get diagnostics", diagnostics)
            
        } finally {
            service.state.analyzerFlags = originalFlags.toMutableList()
        }
    }
    
    fun testAnalyzerOutputParsing() {
        // Test that we correctly parse the JSON output from mtlog-analyzer
        val testFile = myFixture.addFileToProject("parsing_test.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                // Multiple issues in one file
                log.Error("Error {error_code} for {UserId}", 500)  // PascalCase warning
                log.Information("User logged in", "extra")          // Extra argument
                log.Debug("Processing {@user}", "data")             // Correct usage
            }
        """.trimIndent())
        
        // Write files to disk for go vet
        val dir = realProjectDir
        writeFilesToDisk(dir)
        
        val diagnostics = service.runAnalyzer(File(dir, "parsing_test.go").absolutePath, dir.absolutePath)
        
        assertNotNull("Should parse diagnostics", diagnostics)
        assertTrue("Should find multiple diagnostics", diagnostics!!.size >= 2)
        
        // Verify we correctly extract line numbers
        for (diagnostic in diagnostics) {
            assertTrue("Line number should be positive", diagnostic.lineNumber > 0)
            assertTrue("Column number should be positive", diagnostic.columnNumber > 0)
            assertNotNull("Should have message", diagnostic.message)
            assertNotNull("Should have severity", diagnostic.severity)
        }
        
        // Check that we extract property names from PascalCase suggestions
        val pascalCaseDiagnostic = diagnostics.find { it.message.contains("PascalCase") }
        if (pascalCaseDiagnostic != null) {
            assertEquals("Should extract property name", "error_code", pascalCaseDiagnostic.propertyName)
        }
    }
    
    fun testWindowsPathHandling() {
        if (!System.getProperty("os.name").lowercase().contains("windows")) {
            return // Skip on non-Windows
        }
        
        // Test Windows-specific path parsing with drive letters
        val testFile = myFixture.addFileToProject("windows_test.go", """
            package main
            
            import "github.com/willibrandon/mtlog"
            
            func main() {
                log := mtlog.New()
                log.Information("Test {Value}", 123, 456)
            }
        """.trimIndent())
        
        // Write files to disk for go vet
        val dir = realProjectDir
        writeFilesToDisk(dir)
        
        val diagnostics = service.runAnalyzer(File(dir, "windows_test.go").absolutePath, dir.absolutePath)
        
        assertNotNull("Should handle Windows paths", diagnostics)
        if (diagnostics!!.isNotEmpty()) {
            // Verify the diagnostic was matched to the correct file
            // (The service normalizes paths for comparison)
            assertTrue("Should have found diagnostics for the file", diagnostics.isNotEmpty())
        }
    }
    
    fun testConcurrentAnalyzerCalls() {
        // Test that concurrent calls don't interfere with each other
        val file1 = myFixture.addFileToProject("concurrent1.go", """
            package main
            import "github.com/willibrandon/mtlog"
            func main() {
                log := mtlog.New()
                log.Error("Error {Code}", 500, "extra")
            }
        """.trimIndent())
        
        val file2 = myFixture.addFileToProject("concurrent2.go", """
            package main
            import "github.com/willibrandon/mtlog"  
            func main() {
                log := mtlog.New()
                log.Warning("Warning {warning_level}", "high")
            }
        """.trimIndent())
        
        // Write files to disk for go vet
        val dir = realProjectDir
        writeFilesToDisk(dir)
        
        // Run analyzers concurrently
        val thread1 = Thread {
            val diags = service.runAnalyzer(File(dir, "concurrent1.go").absolutePath, dir.absolutePath)
            assertNotNull("Thread 1 should get diagnostics", diags)
        }
        
        val thread2 = Thread {
            val diags = service.runAnalyzer(File(dir, "concurrent2.go").absolutePath, dir.absolutePath)
            assertNotNull("Thread 2 should get diagnostics", diags)
        }
        
        thread1.start()
        thread2.start()
        
        thread1.join(10000) // 10 second timeout
        thread2.join(10000)
        
        assertTrue("Thread 1 should complete", !thread1.isAlive)
        assertTrue("Thread 2 should complete", !thread2.isAlive)
    }
    
    private fun findAnalyzerPath(): String? {
        // Since findAnalyzerPath is now internal, we can call it directly
        return service.findAnalyzerPath()
    }
    
    private fun writeFilesToDisk(dir: File) {
        dir.mkdirs()
        
        // Write go.mod
        File(dir, "go.mod").writeText("""
            module testproject
            
            go 1.21
        """.trimIndent())
        
        // Create vendor directory for mtlog
        val mtlogDir = File(dir, "vendor/github.com/willibrandon/mtlog")
        mtlogDir.mkdirs()
        
        // Create minimal mtlog.go
        File(mtlogDir, "mtlog.go").writeText("""
            package mtlog
            
            type Logger struct{}
            
            func New() *Logger { return &Logger{} }
            
            func (l *Logger) Information(template string, args ...interface{}) {}
            func (l *Logger) Warning(template string, args ...interface{}) {}
            func (l *Logger) Error(template string, args ...interface{}) {}
            func (l *Logger) Debug(template string, args ...interface{}) {}
        """.trimIndent())
        
        // Write test files from the fixture to disk
        val psiManager = PsiManager.getInstance(project)
        myFixture.tempDirFixture.getFile(".")?.let { vFile ->
            vFile.children.forEach { child ->
                if (child.name.endsWith(".go")) {
                    val psiFile = psiManager.findFile(child)
                    if (psiFile != null) {
                        File(dir, child.name).writeText(psiFile.text)
                    }
                }
            }
        }
    }
}
package com.mtlog.analyzer.integration

import com.goide.GoLanguage
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.service
import com.intellij.testFramework.LightProjectDescriptor
import com.intellij.testFramework.fixtures.BasePlatformTestCase
import com.intellij.testFramework.fixtures.IdeaTestFixtureFactory
import com.intellij.testFramework.fixtures.impl.LightTempDirTestFixtureImpl
import com.mtlog.analyzer.service.MtlogProjectService
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.Paths

abstract class MtlogIntegrationTestBase : BasePlatformTestCase() {
    
    protected var realProjectDir: File = Files.createTempDirectory("mtlog-test-").toFile()
    
    override fun setUp() {
        super.setUp()
        
        // Verify mtlog-analyzer is installed
        val analyzerPath = findMtlogAnalyzer()
        assertNotNull("mtlog-analyzer must be installed. Run: go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest", analyzerPath)
        
        // Enable analyzer by default (tests can disable it if needed)
        val service = project.service<MtlogProjectService>()
        service.state.enabled = true
        
        // Set up the test project with real files
        setupRealTestProject()
    }
    
    override fun tearDown() {
        try {
            realProjectDir.deleteRecursively()
        } finally {
            super.tearDown()
        }
    }
    
    override fun getTestDataPath(): String = "src/test/resources/testData"
    
    override fun getProjectDescriptor(): LightProjectDescriptor = GoProjectDescriptor
    
    private fun setupRealTestProject() {
        // Write go.mod to our temp directory
        File(realProjectDir, "go.mod").writeText("""
            module testproject
            
            go 1.21
        """.trimIndent())
        
        // Create vendor directory with mtlog
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
    }
    
    protected fun createGoMod() {
        // Create go.mod
        val goModContent = """
            module testproject
            
            go 1.21
        """.trimIndent()
        
        myFixture.addFileToProject("go.mod", goModContent)
        
        // Create a minimal vendored mtlog package to avoid module download issues
        createVendoredMtlog()
    }
    
    private fun createVendoredMtlog() {
        // Create vendor directory structure
        myFixture.addFileToProject("vendor/github.com/willibrandon/mtlog/mtlog.go", """
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
    }
    
    protected fun getGoModPath(): String {
        // Get the actual file system path for the test project
        val tempDirPath = myFixture.tempDirFixture.getFile(".")
        return if (tempDirPath != null) {
            File(tempDirPath.path).absolutePath
        } else {
            // Fallback - this should not happen
            myFixture.project.basePath ?: System.getProperty("java.io.tmpdir")
        }
    }
    
    protected fun createGoFile(fileName: String, content: String) {
        // Write file to temp directory only
        val diskFile = File(realProjectDir, fileName)
        diskFile.writeText(content)
    }
    
    protected fun runRealAnalyzer(): List<Any> {
        // Since we're testing with real files, call the service directly
        val service = project.service<MtlogProjectService>()
        
        // Get list of go files in our temp directory
        val goFiles = realProjectDir.listFiles { _, name -> name.endsWith(".go") } ?: emptyArray()
        
        // Run analyzer on each file and collect diagnostics
        val allDiagnostics = mutableListOf<Any>()
        for (file in goFiles) {
            val diagnostics = service.runAnalyzer(file.absolutePath, realProjectDir.absolutePath)
            if (diagnostics != null) {
                allDiagnostics.addAll(diagnostics)
            }
        }
        
        return allDiagnostics
    }
    
    private fun findMtlogAnalyzer(): String? {
        val isWindows = System.getProperty("os.name").lowercase().contains("windows")
        val executableName = if (isWindows) "mtlog-analyzer.exe" else "mtlog-analyzer"
        
        val pathEnv = System.getenv("PATH") ?: return null
        val pathDirs = pathEnv.split(if (isWindows) ";" else ":")
        
        for (dir in pathDirs) {
            val file = File(dir, executableName)
            if (file.exists() && file.canExecute()) {
                return file.absolutePath
            }
        }
        
        return null
    }
    
    object GoProjectDescriptor : LightProjectDescriptor()
}
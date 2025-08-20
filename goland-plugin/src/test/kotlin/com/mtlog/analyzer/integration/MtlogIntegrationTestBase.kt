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
        
        // Only set up the test project for tests that need it
        // (tests that use runRealAnalyzer or runRealAnalyzerWithQuickFixes)
        if (shouldSetupRealTestProject()) {
            setupRealTestProject()
        }
    }
    
    /**
     * Override this method to return true for tests that require a real Go project setup
     * with vendor dependencies and go.mod file. This is needed for tests that use
     * runRealAnalyzer() or runRealAnalyzerWithQuickFixes().
     * 
     * Setting up a real project involves creating:
     * - go.mod file with mtlog dependency
     * - vendor/ directory with mtlog package stubs
     * - modules.txt for vendor validation
     * 
     * @return true if the test needs a real project setup, false otherwise
     */
    protected open fun shouldSetupRealTestProject(): Boolean {
        // By default, don't set up unless the test class opts in
        return false
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
        // Create vendor directory with mtlog package that matches the expected import path
        val vendorDir = File(realProjectDir, "vendor/github.com/willibrandon/mtlog")
        vendorDir.mkdirs()
        
        // Create the mtlog package in vendor
        File(vendorDir, "mtlog.go").writeText("""
            package mtlog
            
            type Logger struct {
                fields []interface{}
            }
            
            func New() *Logger { return &Logger{} }
            
            func (l *Logger) Information(template string, args ...interface{}) {}
            func (l *Logger) Warning(template string, args ...interface{}) {}
            func (l *Logger) Error(template string, args ...interface{}) {}
            func (l *Logger) Debug(template string, args ...interface{}) {}
            func (l *Logger) Fatal(template string, args ...interface{}) {}
            func (l *Logger) Verbose(template string, args ...interface{}) {}
            func (l *Logger) With(args ...interface{}) *Logger { 
                newLogger := &Logger{fields: append(l.fields, args...)}
                return newLogger
            }
            func (l *Logger) ForContext(key string, value interface{}) *Logger { 
                return l.With(key, value)
            }
        """.trimIndent())
        
        // Create go.mod for the test project
        File(realProjectDir, "go.mod").writeText("""
            module testproject
            
            go 1.21
            
            require github.com/willibrandon/mtlog v0.0.0-00010101000000-000000000000
        """.trimIndent())
        
        // Create modules.txt to make vendor directory valid
        File(File(realProjectDir, "vendor"), "modules.txt").writeText("""
            # github.com/willibrandon/mtlog v0.0.0-00010101000000-000000000000
            ## explicit
            github.com/willibrandon/mtlog
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
            
            type Logger struct {
                fields []interface{}
            }
            
            func New() *Logger { return &Logger{} }
            
            func (l *Logger) Information(template string, args ...interface{}) {}
            func (l *Logger) Warning(template string, args ...interface{}) {}
            func (l *Logger) Error(template string, args ...interface{}) {}
            func (l *Logger) Debug(template string, args ...interface{}) {}
            func (l *Logger) Fatal(template string, args ...interface{}) {}
            func (l *Logger) Verbose(template string, args ...interface{}) {}
            func (l *Logger) With(args ...interface{}) *Logger { 
                newLogger := &Logger{fields: append(l.fields, args...)}
                return newLogger
            }
            func (l *Logger) ForContext(key string, value interface{}) *Logger { 
                return l.With(key, value)
            }
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
    
    protected fun runRealAnalyzerWithQuickFixes(): List<Any> {
        // For With() method tests, we need to analyze the package directory
        // to avoid "command-line-arguments" synthetic package issues
        return runAnalyzerOnPackage()
    }
    
    private fun runAnalyzerOnPackage(): List<Any> {
        val service = project.service<MtlogProjectService>()
        val analyzerPath = service.findAnalyzerPath() ?: return emptyList()
        
        // Run go vet on the package directory
        val commandLine = com.intellij.execution.configurations.GeneralCommandLine().apply {
            exePath = "go"
            addParameter("vet")
            addParameter("-json")
            addParameter("-vettool=$analyzerPath")
            // Add any additional analyzer flags BEFORE the package
            service.state.analyzerFlags.forEach { flag ->
                addParameter(flag)
            }
            addParameter(".")  // Analyze current package
            workDirectory = realProjectDir
        }
        
        return try {
            val process = commandLine.createProcess()
            
            // Read output and errors concurrently to avoid deadlock
            val output = process.inputStream.bufferedReader().use { it.readText() }
            val errors = process.errorStream.bufferedReader().use { it.readText() }
            
            // Wait for process with timeout
            val completed = process.waitFor(30, java.util.concurrent.TimeUnit.SECONDS)
            if (!completed) {
                process.destroyForcibly()
                System.err.println("ERROR: go vet process timed out after 30 seconds")
                return emptyList()
            }
            
            val exitCode = process.exitValue()
            
            // Debug output (disabled)
            // System.err.println("DEBUG: go vet exit code: $exitCode")
            // System.err.println("DEBUG: go vet stdout: $output")
            // if (errors.isNotEmpty()) {
            //     System.err.println("DEBUG: go vet stderr: $errors")
            // }
            
            // Try to parse analyzer output from stdout or stderr
            return tryParseAnalyzerOutput(output, errors)
        } catch (e: Exception) {
            System.err.println("DEBUG: Exception running go vet: $e")
            emptyList()
        }
    }
    
    /**
     * Try to parse analyzer output from stdout or stderr
     */
    private fun tryParseAnalyzerOutput(stdout: String, stderr: String): List<Any> {
        return when {
            stdout.isNotEmpty() && stdout != "{}" -> parseGoVetOutput(stdout)
            stderr.contains("{") && stderr.contains("}") -> {
                extractJsonFromMixedOutput(stderr)?.let { parseGoVetOutput(it) } ?: emptyList()
            }
            else -> emptyList()
        }
    }
    
    /**
     * Extract JSON content from mixed output that contains package comment lines
     */
    private fun extractJsonFromMixedOutput(errors: String): String? {
        // Extract just the JSON part (skip package comment lines that start with #)
        val lines = errors.lines()
        val jsonLines = mutableListOf<String>()
        var inJson = false
        var braceCount = 0
        
        for (line in lines) {
            if (line.contains("{")) {
                inJson = true
                braceCount += line.count { it == '{' }
            }
            if (inJson) {
                jsonLines.add(line)
                braceCount -= line.count { it == '}' }
                if (braceCount == 0) {
                    break
                }
            }
        }
        
        return if (jsonLines.isNotEmpty()) {
            jsonLines.joinToString("\n")
        } else {
            null
        }
    }

    private fun parseGoVetOutput(output: String): List<Any> {
        val diagnostics = mutableListOf<com.mtlog.analyzer.service.AnalyzerDiagnostic>()
        
        try {
            val json = com.google.gson.JsonParser.parseString(output).asJsonObject
            
            json.entrySet().forEach { packageEntry ->
                val packageObj = packageEntry.value.asJsonObject
                if (packageObj.has("mtlog")) {
                    val mtlogArray = packageObj.getAsJsonArray("mtlog")
                    
                    mtlogArray.forEach { element ->
                        val diagnostic = element.asJsonObject
                        val posn = diagnostic.get("posn").asString
                        val message = diagnostic.get("message").asString
                        
                        // Parse position
                        val parts = posn.split(":")
                        val lineNum = parts.getOrNull(parts.size - 2)?.toIntOrNull() ?: 1
                        val columnNum = parts.getOrNull(parts.size - 1)?.toIntOrNull() ?: 1
                        
                        // Extract diagnostic ID
                        val diagnosticIdMatch = Regex("^\\[(MTLOG\\d+)\\]\\s*").find(message)
                        val diagnosticId = diagnosticIdMatch?.groupValues?.get(1)
                        
                        // Parse suggested fixes
                        val suggestedFixes = mutableListOf<com.mtlog.analyzer.service.AnalyzerSuggestedFix>()
                        val fixesArray = diagnostic.get("suggested_fixes")?.asJsonArray
                        if (fixesArray != null) {
                            fixesArray.forEach { fixElement ->
                                val fixObj = fixElement.asJsonObject
                                val fixMessage = fixObj.get("message").asString
                                val textEdits = mutableListOf<com.mtlog.analyzer.service.AnalyzerTextEdit>()
                                
                                val editsArray = fixObj.get("edits")?.asJsonArray
                                if (editsArray != null) {
                                    editsArray.forEach { editElement ->
                                        val editObj = editElement.asJsonObject
                                        if (editObj.has("filename") && editObj.has("start") && editObj.has("end")) {
                                            val editFilename = editObj.get("filename").asString
                                            val startOffset = editObj.get("start").asInt
                                            val endOffset = editObj.get("end").asInt
                                            val newText = editObj.get("new").asString
                                            
                                            // Convert byte offsets to line:column
                                            val fileContent = try {
                                                java.io.File(editFilename).readText()
                                            } catch (e: Exception) {
                                                System.err.println("Could not read file for offset conversion: $editFilename - ${e.message}")
                                                ""
                                            }
                                            
                                            if (fileContent.isNotEmpty()) {
                                                val startPos = offsetToLineCol(fileContent, startOffset)
                                                val endPos = offsetToLineCol(fileContent, endOffset)
                                                
                                                textEdits.add(com.mtlog.analyzer.service.AnalyzerTextEdit(
                                                    pos = "$editFilename:${startPos.first}:${startPos.second}",
                                                    end = "$editFilename:${endPos.first}:${endPos.second}",
                                                    newText = newText
                                                ))
                                            }
                                        }
                                    }
                                }
                                
                                suggestedFixes.add(com.mtlog.analyzer.service.AnalyzerSuggestedFix(fixMessage, textEdits))
                            }
                        }
                        
                        diagnostics.add(com.mtlog.analyzer.service.AnalyzerDiagnostic(
                            lineNumber = lineNum,
                            columnNumber = columnNum,
                            message = message,
                            severity = if (message.contains("error")) "error" else "warning",
                            propertyName = null,
                            diagnosticId = diagnosticId,
                            suggestedFixes = suggestedFixes
                        ))
                    }
                }
            }
        } catch (e: Exception) {
            // Ignore parsing errors
        }
        
        return diagnostics
    }
    
    companion object {
        private const val LINE_START = 1
        private const val COLUMN_START = 1
    }

    private fun offsetToLineCol(content: String, offset: Int): Pair<Int, Int> {
        var line = LINE_START
        var col = COLUMN_START
        var currentOffset = 0
        
        for (char in content) {
            if (currentOffset == offset) {
                return Pair(line, col)
            }
            if (char == '\n') {
                line++
                col = COLUMN_START
            } else {
                col++
            }
            currentOffset++
        }
        
        return Pair(line, col)
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
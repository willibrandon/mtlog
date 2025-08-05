package com.mtlog.analyzer.service

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.*
import com.intellij.openapi.Disposable
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.WindowManager
import com.intellij.openapi.util.Key
import com.mtlog.analyzer.settings.MtlogSettingsState
import java.nio.file.Path
import java.nio.file.Paths
import java.util.concurrent.ConcurrentHashMap
import kotlin.io.path.absolutePathString
import kotlin.io.path.exists
import com.google.gson.Gson
import com.google.gson.JsonParser
import java.io.BufferedReader

/**
 * Diagnostic information from mtlog-analyzer.
 */
data class AnalyzerDiagnostic(
    val lineNumber: Int,
    val columnNumber: Int,
    val message: String,
    val severity: String,
    val propertyName: String? = null,
    val diagnosticId: String? = null
)

/**
 * Project-level service managing mtlog-analyzer processes and configuration.
 * 
 * This service acts as the central manager for all mtlog-analyzer functionality within a project:
 * - Manages a pool of analyzer processes per Go module to avoid process startup overhead
 * - Handles analyzer discovery and execution with automatic PATH resolution
 * - Provides persistent configuration storage for analyzer settings
 * - Implements caching and version mismatch detection for analyzer updates
 * - Ensures proper resource cleanup through the Disposable interface
 * 
 * The service maintains one analyzer process per go.mod file to support multi-module projects
 * efficiently. Processes are reused for multiple analysis runs within the same module.
 */
@Service(Service.Level.PROJECT)
@State(
    name = "MtlogProjectSettings",
    storages = [Storage("mtlog.xml")]
)
class MtlogProjectService(
    private val project: Project
) : PersistentStateComponent<MtlogSettingsState>, Disposable {
    
    companion object {
        private val LOG = logger<MtlogProjectService>()
        private val ANALYZER_VERSION_REGEX = Regex("""mtlog-analyzer\s+version[:\s]+([^\s]+)""")
        private const val ENV_MTLOG_SUPPRESS = "MTLOG_SUPPRESS"  // Environment variable for diagnostic suppression
        private const val LOG_TRUNCATION_LENGTH = 500  // Maximum length for debug log output
        
        /**
         * Truncates a string for debug logging to prevent excessively long log entries.
         */
        private fun truncateForLog(text: String, maxLength: Int = LOG_TRUNCATION_LENGTH): String {
            return if (text.length <= maxLength) {
                text
            } else {
                "${text.substring(0, maxLength)}... (truncated ${text.length - maxLength} chars)"
            }
        }
    }
    
    private val processPool = ConcurrentHashMap<String, OSProcessHandler>()
    private var cachedVersion: String? = null
    private var state = MtlogSettingsState()
    
    override fun getState(): MtlogSettingsState = state
    
    override fun loadState(state: MtlogSettingsState) {
        val oldEnabled = this.state.enabled
        this.state = state
        
        // Update status bar widget if enabled state changed
        if (oldEnabled != state.enabled) {
            updateStatusBarWidget()
        }
    }
    
    /**
     * Updates the status bar widget to reflect current state.
     */
    fun updateStatusBarWidget() {
        WindowManager.getInstance().getStatusBar(project)?.updateWidget("MtlogAnalyzer")
    }
    
    /**
     * Gets or creates analyzer process for the given module path.
     */
    fun getAnalyzerProcess(goModPath: String): OSProcessHandler? {
        LOG.debug("getAnalyzerProcess called for: $goModPath, enabled: ${state.enabled}")
        if (!state.enabled) return null
        
        return try {
            processPool.computeIfAbsent(goModPath) { path ->
                LOG.debug("Creating new analyzer process for: $path")
                val process = createAnalyzerProcess(path)
                if (process == null) {
                    LOG.error("Failed to create analyzer process for: $path")
                    throw IllegalStateException("Failed to create analyzer process")
                }
                
                process.also { handler ->
                    handler.addProcessListener(object : ProcessListener {
                        override fun onTextAvailable(event: ProcessEvent, outputType: Key<*>) {
                            if (outputType == ProcessOutputTypes.STDERR) {
                                checkVersionMismatch(event.text)
                            }
                        }
                        
                        override fun processTerminated(event: ProcessEvent) {
                            // Required by ProcessListener interface, but this service does not need to handle process termination events
                        }
                        
                        override fun startNotified(event: ProcessEvent) {
                            // Required by ProcessListener interface, but this service does not need to handle process start events
                        }
                    })
                    LOG.debug("Successfully created analyzer process")
                }
            }
        } catch (e: Exception) {
            LOG.error("Exception creating analyzer process", e)
            null
        }
    }
    
    private fun createAnalyzerProcess(goModPath: String): OSProcessHandler? {
        LOG.debug("createAnalyzerProcess called for: $goModPath")
        val analyzerPath = findAnalyzerPath()
        if (analyzerPath == null) {
            LOG.error("Could not find mtlog-analyzer in PATH or configured location")
            return null
        }
        LOG.debug("Found analyzer at: $analyzerPath")
        
        val commandLine = GeneralCommandLine().apply {
            exePath = "go"
            addParameter("vet")
            addParameter("-json")
            addParameter("-vettool=$analyzerPath")
            state.analyzerFlags.forEach { addParameter(it) }
            addParameter("./...")
            workDirectory = Paths.get(goModPath).toFile()
            withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
        }
        
        return try {
            val handler = OSProcessHandler(commandLine)
            handler.startNotify()
            handler
        } catch (e: Exception) {
            LOG.error("Failed to start analyzer process", e)
            null
        }
    }
    
    private fun findAnalyzerPath(): String? {
        val configuredPath = state.analyzerPath
        LOG.debug("findAnalyzerPath - configured path: $configuredPath")
        
        // If it's an absolute path, use it directly
        if (configuredPath != null && Paths.get(configuredPath).isAbsolute) {
            val exists = Paths.get(configuredPath).exists()
            LOG.debug("Absolute path exists: $exists")
            return if (exists) configuredPath else null
        }
        
        // Try to find in PATH
        return findInPath(configuredPath ?: "mtlog-analyzer")
    }
    
    private fun findInPath(name: String): String? {
        val isWindows = System.getProperty("os.name").lowercase().contains("windows")
        val executableName = if (isWindows) "$name.exe" else name
        
        val pathEnv = System.getenv("PATH") ?: return null
        val pathDirs = pathEnv.split(if (isWindows) ";" else ":")
        
        for (dir in pathDirs) {
            val file = Paths.get(dir, executableName)
            if (file.exists()) {
                return file.absolutePathString()
            }
        }
        
        return null
    }
    
    private fun checkVersionMismatch(stderr: String) {
        val matchResult = ANALYZER_VERSION_REGEX.find(stderr)
        if (matchResult != null) {
            val version = matchResult.groupValues[1]
            
            if (cachedVersion != null && cachedVersion != version) {
                LOG.info("Analyzer version changed from $cachedVersion to $version, restarting process pool")
                disposeProcessPool()
                cachedVersion = version
            } else if (cachedVersion == null) {
                cachedVersion = version
            }
        }
    }
    
    override fun dispose() {
        disposeProcessPool()
    }
    
    private fun disposeProcessPool() {
        processPool.values.toList().forEach { 
            try {
                it.destroyProcess()
            } catch (e: Exception) {
                LOG.error("Error destroying process", e)
            }
        }
        processPool.clear()
    }
    
    /**
     * Clears analyzer cache and restarts processes.
     */
    fun clearCache() {
        cachedVersion = null
        disposeProcessPool()
    }
    
    /**
     * Runs analyzer on specific file and returns diagnostics.
     */
    fun runAnalyzer(filePath: String, goModPath: String): List<AnalyzerDiagnostic>? {
        LOG.debug("runAnalyzer called for file: $filePath")
        
        // Check if analyzer is enabled
        if (!state.enabled) {
            LOG.debug("Analyzer is disabled")
            return emptyList()
        }
        
        val analyzerPath = findAnalyzerPath()
        if (analyzerPath == null) {
            LOG.error("Could not find mtlog-analyzer")
            return null
        }
        
        val commandLine = GeneralCommandLine().apply {
            exePath = "go"
            addParameter("vet")
            addParameter("-json")
            addParameter("-vettool=$analyzerPath")
            
            // Add other analyzer flags
            state.analyzerFlags.forEach { addParameter(it) }
            addParameter(filePath)
            workDirectory = Paths.get(goModPath).toFile()
            
            // Add suppression via environment variable if there are suppressed diagnostics
            if (state.suppressedDiagnostics.isNotEmpty()) {
                val suppressedList = state.suppressedDiagnostics.joinToString(",")
                withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
                environment[ENV_MTLOG_SUPPRESS] = suppressedList
                LOG.debug("Setting MTLOG_SUPPRESS environment variable: $suppressedList")
            } else {
                withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
            }
        }
        
        return try {
            LOG.debug("Running command: ${commandLine.commandLineString}")
            LOG.debug("Environment: ${commandLine.environment}")
            val process = commandLine.createProcess()
            val output = process.inputStream.bufferedReader().use { it.readText() }
            val errors = process.errorStream.bufferedReader().use { it.readText() }
            
            val exitCode = process.waitFor()
            LOG.debug("Analyzer exit code: $exitCode")
            
            if (errors.isNotEmpty()) {
                LOG.debug("Analyzer stderr (${errors.length} chars): ${truncateForLog(errors)}")
            }
            if (output.isNotEmpty()) {
                LOG.debug("Analyzer stdout (${output.length} chars): ${truncateForLog(output)}")
            }
            
            // mtlog-analyzer outputs to either stderr or stdout depending on format
            // Try both and combine results
            val stderrDiagnostics = if (errors.isNotEmpty()) {
                parseAnalyzerOutput(errors, filePath)
            } else {
                emptyList()
            }
            
            val stdoutDiagnostics = if (output.isNotEmpty()) {
                parseAnalyzerOutput(output, filePath)
            } else {
                emptyList()
            }
            
            // Combine results, preferring stderr if both have diagnostics
            val result = if (stderrDiagnostics.isNotEmpty()) {
                stderrDiagnostics
            } else {
                stdoutDiagnostics
            }
            
            LOG.debug("Total diagnostics found: ${result.size}")
            if (result.isEmpty()) {
                LOG.warn("No diagnostics found. stderr had ${stderrDiagnostics.size}, stdout had ${stdoutDiagnostics.size}")
                LOG.warn("Analyzer command was: ${commandLine.commandLineString}")
                if (errors.isNotEmpty()) {
                    LOG.warn("Stderr output: ${truncateForLog(errors)}")
                }
                if (output.isNotEmpty()) {
                    LOG.warn("Stdout output: ${truncateForLog(output)}")
                }
            }
            result
        } catch (e: Exception) {
            LOG.error("Failed to run analyzer", e)
            null
        }
    }
    
    private fun parseAnalyzerOutput(output: String, filePath: String): List<AnalyzerDiagnostic> {
        val diagnostics = mutableListOf<AnalyzerDiagnostic>()
        LOG.debug("parseAnalyzerOutput called for file: $filePath, output length: ${output.length}")
        
        // Check if this looks like plain text stderr output (go vet format)
        // Use plain text parsing if it contains diagnostic format (even with JSON present)
        if (output.contains(".go:") && output.contains(": ")) {
            LOG.debug("Found go vet format in output, using plain text parser")
            val stderrDiagnostics = parseStderrOutput(output, filePath)
            if (stderrDiagnostics.isNotEmpty()) {
                return stderrDiagnostics
            }
            // Fall through to JSON parsing if no diagnostics found
        }
        
        try {
            // Find the JSON object in the output
            val jsonStart = output.indexOf('{')
            if (jsonStart == -1) {
                LOG.warn("No JSON found in analyzer output")
                return emptyList()
            }
            
            val jsonString = output.substring(jsonStart)
            LOG.debug("Attempting to parse JSON: ${jsonString.take(200)}...")
            val json = JsonParser.parseString(jsonString).asJsonObject
            
            // Navigate to the mtlog diagnostics
            json.entrySet().forEach { packageEntry ->
                LOG.debug("Processing package: ${packageEntry.key}")
                val packageObj = packageEntry.value.asJsonObject
                if (packageObj.has("mtlog")) {
                    val mtlogArray = packageObj.getAsJsonArray("mtlog")
                LOG.debug("Found ${mtlogArray.size()} mtlog diagnostics")
                    
                    mtlogArray.forEach { element ->
                        val diagnostic = element.asJsonObject
                        val posn = diagnostic.get("posn").asString
                        
                        // Parse position: "filename:line:column"
                        // Handle Windows paths with drive letters (e.g., D:\path\file.go:line:col)
                        val colonCount = posn.count { it == ':' }
                        val diagnosticFilePath: String
                        val lineNum: Int
                        val columnNum: Int
                        
                        if (colonCount >= 3 && posn.length > 2 && posn[1] == ':') {
                            // Windows path with drive letter
                            val lastColonIndex = posn.lastIndexOf(':')
                            val secondLastColonIndex = posn.lastIndexOf(':', lastColonIndex - 1)
                            diagnosticFilePath = posn.substring(0, secondLastColonIndex)
                            lineNum = posn.substring(secondLastColonIndex + 1, lastColonIndex).toIntOrNull() ?: 1
                            columnNum = posn.substring(lastColonIndex + 1).toIntOrNull() ?: 1
                        } else {
                            // Unix-style path
                            val parts = posn.split(":")
                            if (parts.size < 3) return@forEach
                            diagnosticFilePath = parts[0]
                            lineNum = parts[1].toIntOrNull() ?: 1
                            columnNum = parts[2].toIntOrNull() ?: 1
                        }
                        
                LOG.debug("Parsed position - file: $diagnosticFilePath, line: $lineNum, col: $columnNum")
                        
                        // Compare by filename only since paths might be in different formats
                        // (e.g., test output might have Unix paths while filePath has Windows paths)
                        val diagnosticFileName = Paths.get(diagnosticFilePath).fileName.toString()
                        val targetFileName = Paths.get(filePath).fileName.toString()
                LOG.debug("Comparing filenames - diagnostic: '$diagnosticFileName', target: '$targetFileName'")
                        
                        if (diagnosticFileName == targetFileName) {
                            val message = diagnostic.get("message").asString
                            LOG.debug("Found matching diagnostic: $message at line $lineNum, col $columnNum")
                            
                            // Extract diagnostic ID if present
                            val diagnosticIdMatch = Regex("^\\[(MTLOG\\d+)\\]\\s*").find(message)
                            val diagnosticId = diagnosticIdMatch?.groupValues?.get(1)
                            
                            // Check if this diagnostic is suppressed
                            if (diagnosticId != null && state.suppressedDiagnostics.contains(diagnosticId)) {
                                LOG.debug("Skipping suppressed diagnostic: $diagnosticId")
                                return@forEach
                            }
                            
                            // Remove [MTLOGXXX] prefix if present to check the actual message content
                            val cleanMessage = message.replace(Regex("^\\[MTLOG\\d+\\]\\s*"), "")
                            
                            val severity = when {
                                // MTLOG006 should be an error to match VS Code (check this first before message content)
                                diagnosticId == "MTLOG006" -> "error"
                                cleanMessage.startsWith("suggestion:") -> "suggestion"
                                cleanMessage.contains("error") -> "error"
                                cleanMessage.contains("template has") && cleanMessage.contains("but") && cleanMessage.contains("provided") -> "error"
                                cleanMessage.contains("properties but") && cleanMessage.contains("argument") -> "error"
                                else -> "warning"
                            }
                            
                            // Extract property name from PascalCase suggestions
                            val propertyName = if (message.contains("property '")) {
                                message.substringAfter("property '").substringBefore("'")
                            } else null
                            
                            diagnostics.add(AnalyzerDiagnostic(
                                lineNumber = lineNum,
                                columnNumber = columnNum,
                                message = message,
                                severity = severity,
                                propertyName = propertyName,
                                diagnosticId = diagnosticId
                            ))
                        }
                    }
                }
            }
        } catch (e: Exception) {
            LOG.error("Failed to parse analyzer output: $output", e)
        }
        
        LOG.debug("parseAnalyzerOutput returning ${diagnostics.size} diagnostics")
        return diagnostics
    }
    
    private fun parseStderrOutput(output: String, filePath: String): List<AnalyzerDiagnostic> {
        val diagnostics = mutableListOf<AnalyzerDiagnostic>()
        LOG.debug("parseStderrOutput called for file: $filePath")
        LOG.debug("parseStderrOutput raw output: $output")
        
        // Parse lines like:
        // examples/basic/main.go:25:2: [MTLOG006] suggestion: Error level log without error value
        // or ./test.go:12:2: [MTLOG006] suggestion: Error level log without error value
        val lines = output.lines()
        for (line in lines) {
            // Skip compilation error lines that start with package name
            if (line.contains(": undefined:") || line.contains(": cannot use") || line.contains(": undeclared name")) {
                LOG.debug("Skipping compilation error: $line")
                continue
            }
            
            if (!line.contains(".go:")) continue
            
            LOG.debug("Processing line: $line")
            
            // Extract file path, line, column, and message (with optional diagnostic ID)
            val regex = Regex("""^(.+\.go):(\d+):(\d+):\s*(.+)$""")
            val match = regex.find(line) ?: continue
            
            val groups = match.groupValues
            val diagnosticFile = groups[1]
            val lineStr = groups[2]
            val columnStr = groups[3]
            val fullMessage = groups[4]
            
            // Extract diagnostic ID if present in the message
            val diagnosticIdMatch = Regex("""^\[(MTLOG\d+)\]\s*""").find(fullMessage)
            val diagnosticId = diagnosticIdMatch?.groupValues?.get(1)
            // Remove the diagnostic ID from the message if present
            val message = if (diagnosticId != null) {
                fullMessage.removePrefix("[${diagnosticId}]").trim()
            } else {
                fullMessage
            }
            
            // Check if this diagnostic is for our file
            // The diagnostic file might be relative (test.go, ./test.go) or absolute (D:/path/test.go)
            val cleanDiagnosticFile = diagnosticFile.removePrefix("./")
            
            // Safely extract filename
            val diagnosticFileName = try {
                Paths.get(cleanDiagnosticFile).fileName.toString()
            } catch (e: Exception) {
                LOG.warn("Failed to parse diagnostic file path: $cleanDiagnosticFile", e)
                continue
            }
            
            val targetFileName = Paths.get(filePath).fileName.toString()
            
            LOG.debug("Comparing files: diagnostic='$diagnosticFileName' target='$targetFileName'")
            
            // Match by filename since the paths might be in different formats
            if (diagnosticFileName == targetFileName) {
                val lineNum = lineStr.toIntOrNull() ?: 1
                val columnNum = columnStr.toIntOrNull() ?: 1
                
                LOG.debug("Found diagnostic: $message at line $lineNum, col $columnNum, diagnosticId=$diagnosticId")
                
                // Clean message for severity detection (remove [MTLOGXXX] prefix if in message)
                val cleanMessage = message.replace(Regex("^\\[MTLOG\\d+\\]\\s*"), "")
                
                val severity = when {
                    // MTLOG006 should be an error to match VS Code (check this first before message content)
                    diagnosticId == "MTLOG006" -> "error"
                    cleanMessage.startsWith("suggestion:") -> "suggestion"
                    cleanMessage.contains("error") -> "error"
                    cleanMessage.contains("template has") && cleanMessage.contains("but") && cleanMessage.contains("provided") -> "error"
                    cleanMessage.contains("properties but") && cleanMessage.contains("argument") -> "error"
                    else -> "warning"
                }
                
                // Check if this diagnostic is suppressed
                if (diagnosticId != null && state.suppressedDiagnostics.contains(diagnosticId)) {
                    LOG.debug("Skipping suppressed diagnostic: $diagnosticId from suppressed list: ${state.suppressedDiagnostics}")
                    continue
                }
                LOG.debug("Diagnostic $diagnosticId is not suppressed, adding to results")
                
                // Extract property name from PascalCase suggestions
                val propertyName = if (message.contains("property '")) {
                    message.substringAfter("property '").substringBefore("'")
                } else null
                
                // Add the diagnostic with the original message format
                val displayMessage = if (diagnosticId != null) "[$diagnosticId] $message" else message
                diagnostics.add(AnalyzerDiagnostic(
                    lineNumber = lineNum,
                    columnNumber = columnNum,
                    message = displayMessage,
                    severity = severity,
                    propertyName = propertyName,
                    diagnosticId = diagnosticId
                ))
            }
        }
        
        LOG.debug("parseStderrOutput returning ${diagnostics.size} diagnostics")
        return diagnostics
    }
}
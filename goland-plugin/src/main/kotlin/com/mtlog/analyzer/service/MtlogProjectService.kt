package com.mtlog.analyzer.service

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.*
import com.intellij.openapi.Disposable
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.mtlog.analyzer.logging.MtlogLogger
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
import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.notification.NotificationAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.actions.InstallAnalyzerAction

/**
 * Diagnostic information from mtlog-analyzer.
 */
data class AnalyzerDiagnostic(
    val lineNumber: Int,
    val columnNumber: Int,
    val message: String,
    val severity: String,
    val propertyName: String? = null,
    val diagnosticId: String? = null,
    val suggestedFixes: List<AnalyzerSuggestedFix> = emptyList()
)

data class AnalyzerSuggestedFix(
    val message: String,
    val textEdits: List<AnalyzerTextEdit>
)

data class AnalyzerTextEdit(
    val pos: String,  // Format: "file:line:col"
    val end: String,  // Format: "file:line:col"
    val newText: String
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
        WindowManager.getInstance().getStatusBar(project)?.updateWidget("mtlog-analyzer")
    }
    
    /**
     * Gets or creates analyzer process for the given module path.
     */
    fun getAnalyzerProcess(goModPath: String): OSProcessHandler? {
        MtlogLogger.debug("getAnalyzerProcess called for: $goModPath, enabled: ${state.enabled}", project)
        if (!state.enabled) return null
        
        return try {
            processPool.computeIfAbsent(goModPath) { path ->
                MtlogLogger.debug("Creating new analyzer process for: $path", project)
                val process = createAnalyzerProcess(path)
                if (process == null) {
                    MtlogLogger.error("Failed to create analyzer process for: $path", project)
                    showAnalyzerNotFoundNotification()
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
                    MtlogLogger.debug("Successfully created analyzer process", project)
                }
            }
        } catch (e: Exception) {
            MtlogLogger.error("Exception creating analyzer process", project, e)
            null
        }
    }
    
    private fun createAnalyzerProcess(goModPath: String): OSProcessHandler? {
        MtlogLogger.debug("createAnalyzerProcess called for: $goModPath", project)
        val analyzerPath = findAnalyzerPath()
        if (analyzerPath == null) {
            MtlogLogger.error("Could not find mtlog-analyzer in PATH or configured location", project)
            return null
        }
        MtlogLogger.debug("Found analyzer at: $analyzerPath", project)
        
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
            MtlogLogger.error("Failed to start analyzer process", project, e)
            null
        }
    }
    
    internal fun findAnalyzerPath(): String? {
        val configuredPath = state.analyzerPath
        MtlogLogger.debug("findAnalyzerPath - configured path: $configuredPath", project)
        
        // If it's an absolute path that exists, use it directly
        if (configuredPath != null && Paths.get(configuredPath).isAbsolute) {
            val exists = Paths.get(configuredPath).exists()
            MtlogLogger.debug("Absolute path exists: $exists", project)
            if (exists) return configuredPath
        }
        
        // Try to find in PATH
        val pathResult = findInPath(configuredPath ?: "mtlog-analyzer")
        if (pathResult != null) {
            MtlogLogger.debug("Found in PATH: $pathResult", project)
            return pathResult
        }
        
        // Try Go installation locations
        return findInGoInstallations()
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
    
    /**
     * Searches for mtlog-analyzer in standard Go installation locations.
     * Follows Go's installation precedence: GOBIN → GOPATH/bin → $HOME/go/bin
     */
    private fun findInGoInstallations(): String? {
        val isWindows = System.getProperty("os.name").lowercase().contains("windows")
        val executableName = if (isWindows) "mtlog-analyzer.exe" else "mtlog-analyzer"
        
        // 1. Check GOBIN
        val gobin = System.getenv("GOBIN")
        if (!gobin.isNullOrEmpty()) {
            val gobinPath = Paths.get(gobin, executableName)
            if (gobinPath.exists()) {
                MtlogLogger.debug("Found in GOBIN: ${gobinPath.absolutePathString()}", project)
                return gobinPath.absolutePathString()
            }
        }
        
        // 2. Check GOPATH/bin (can be multiple paths)
        val gopath = System.getenv("GOPATH")
        if (!gopath.isNullOrEmpty()) {
            val separator = if (isWindows) ";" else ":"
            gopath.split(separator).forEach { path ->
                val gopathBin = Paths.get(path, "bin", executableName)
                if (gopathBin.exists()) {
                    MtlogLogger.debug("Found in GOPATH/bin: ${gopathBin.absolutePathString()}", project)
                    return gopathBin.absolutePathString()
                }
            }
        }
        
        // 3. Check $HOME/go/bin (default when GOPATH not set)
        val userHome = System.getProperty("user.home")
        if (!userHome.isNullOrEmpty()) {
            val defaultGoBin = Paths.get(userHome, "go", "bin", executableName)
            if (defaultGoBin.exists()) {
                MtlogLogger.debug("Found in ~/go/bin: ${defaultGoBin.absolutePathString()}", project)
                return defaultGoBin.absolutePathString()
            }
        }
        
        // 4. Platform-specific locations
        when {
            isWindows -> {
                // Check %LOCALAPPDATA%\Microsoft\WindowsApps (for scoop, etc.)
                val localAppData = System.getenv("LOCALAPPDATA")
                if (!localAppData.isNullOrEmpty()) {
                    val windowsApps = Paths.get(localAppData, "Microsoft", "WindowsApps", executableName)
                    if (windowsApps.exists()) {
                        MtlogLogger.debug("Found in WindowsApps: ${windowsApps.absolutePathString()}", project)
                        return windowsApps.absolutePathString()
                    }
                }
            }
            System.getProperty("os.name").lowercase().contains("mac") -> {
                // Check /usr/local/go/bin (common on macOS)
                val usrLocalGo = Paths.get("/usr", "local", "go", "bin", executableName)
                if (usrLocalGo.exists()) {
                    MtlogLogger.debug("Found in /usr/local/go/bin: ${usrLocalGo.absolutePathString()}", project)
                    return usrLocalGo.absolutePathString()
                }
            }
        }
        
        MtlogLogger.debug("mtlog-analyzer not found in any standard Go installation location", project)
        return null
    }
    
    private fun checkVersionMismatch(stderr: String) {
        val matchResult = ANALYZER_VERSION_REGEX.find(stderr)
        if (matchResult != null) {
            val version = matchResult.groupValues[1]
            
            if (cachedVersion != null && cachedVersion != version) {
                MtlogLogger.info("Analyzer version changed from $cachedVersion to $version, restarting process pool", project)
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
                MtlogLogger.error("Error destroying process", project, e)
            }
        }
        processPool.clear()
    }
    
    /**
     * Restarts analyzer processes.
     */
    fun restartProcesses() {
        cachedVersion = null
        disposeProcessPool()
    }
    
    /**
     * Runs analyzer on specific file and returns diagnostics.
     * Uses stdin mode to pass content directly without file system dependency.
     */
    fun runAnalyzer(filePath: String, goModPath: String, content: String? = null): List<AnalyzerDiagnostic>? {
        MtlogLogger.debug("runAnalyzer called for file: $filePath", project)
        
        // Check if analyzer is enabled
        if (!state.enabled) {
            MtlogLogger.debug("Analyzer is disabled", project)
            return emptyList()
        }
        
        val analyzerPath = findAnalyzerPath()
        if (analyzerPath == null) {
            MtlogLogger.error("Could not find mtlog-analyzer", project)
            showAnalyzerNotFoundNotification()
            return null
        }
        
        val commandLine = if (content != null) {
            // Use stdin mode with content
            GeneralCommandLine().apply {
                exePath = analyzerPath
                addParameter("-stdin")
                
                // Add other analyzer flags
                state.analyzerFlags.forEach { addParameter(it) }
                
                workDirectory = Paths.get(goModPath).toFile()
                withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
                
                // Add suppression via environment variable if there are suppressed diagnostics
                if (state.suppressedDiagnostics.isNotEmpty()) {
                    val suppressedList = state.suppressedDiagnostics.joinToString(",")
                    environment[ENV_MTLOG_SUPPRESS] = suppressedList
                    MtlogLogger.debug("Setting MTLOG_SUPPRESS environment variable: $suppressedList", project)
                }
                
                MtlogLogger.info("Stdin mode command: ${commandLineString}", project)
            }
        } else {
            // Fallback to file mode
            GeneralCommandLine().apply {
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
                    MtlogLogger.debug("Setting MTLOG_SUPPRESS environment variable: $suppressedList", project)
                } else {
                    withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
                }
            }
        }
        
        return try {
            MtlogLogger.debug("Running command: ${commandLine.commandLineString}", project)
            MtlogLogger.debug("Environment: ${commandLine.environment}", project)
            
            val process = if (content != null) {
                // For stdin mode, create process and write content
                val cmdList = mutableListOf<String>()
                cmdList.add(commandLine.exePath)
                cmdList.addAll(commandLine.parametersList.parameters)
                
                val processBuilder = ProcessBuilder(cmdList)
                processBuilder.directory(commandLine.workDirectory)
                processBuilder.environment().putAll(commandLine.environment)
                
                val proc = processBuilder.start()
                
                // Create stdin request
                val stdinRequest = mapOf(
                    "filename" to filePath,
                    "content" to content,
                    "go_module" to goModPath
                )
                
                // Write JSON to stdin and close
                try {
                    proc.outputStream.bufferedWriter().use { writer ->
                        val jsonStr = Gson().toJson(stdinRequest)
                        MtlogLogger.debug("Writing JSON to stdin: ${jsonStr.take(200)}...", project)
                        writer.write(jsonStr)
                        writer.flush()
                    }
                    MtlogLogger.info("Sent ${content.length} chars to analyzer via stdin", project)
                } catch (e: Exception) {
                    MtlogLogger.error("Failed to write to stdin", project, e)
                    throw e
                }
                
                proc
            } else {
                // Log first few lines of the file being analyzed
                try {
                    val fileContent = java.io.File(filePath).readLines().take(10)
                    MtlogLogger.info("Analyzing file content preview: ${fileContent.joinToString("\\n")}", project)
                } catch (e: Exception) {
                    MtlogLogger.warn("Could not read file preview", project)
                }
                commandLine.createProcess()
            }
            
            val output = process.inputStream.bufferedReader().use { it.readText() }
            val errors = process.errorStream.bufferedReader().use { it.readText() }
            
            val exitCode = process.waitFor()
            MtlogLogger.info("Analyzer exit code: $exitCode", project)
            
            // Stderr output is expected for compilation errors, no need to log
            if (output.isNotEmpty()) {
                MtlogLogger.debug("Analyzer stdout (${output.length} chars): ${truncateForLog(output)}", project)
            } else if (content != null) {
                MtlogLogger.warn("Stdin mode produced no stdout output", project)
            }
            
            val result = if (content != null) {
                // Stdin mode returns JSON directly to stdout
                if (output.isNotEmpty()) {
                    parseStdinOutput(output)
                } else {
                    MtlogLogger.warn("No output from stdin mode analyzer", project)
                    emptyList()
                }
            } else {
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
                if (stderrDiagnostics.isNotEmpty()) {
                    stderrDiagnostics
                } else {
                    stdoutDiagnostics
                }
            }
            
            MtlogLogger.debug("Total diagnostics found: ${result.size}", project)
            if (result.isEmpty() && content == null) {
                MtlogLogger.warn("No diagnostics found", project)
                MtlogLogger.warn("Analyzer command was: ${commandLine.commandLineString}", project)
                if (errors.isNotEmpty()) {
                    MtlogLogger.warn("Stderr output: ${truncateForLog(errors)}", project)
                }
                if (output.isNotEmpty()) {
                    MtlogLogger.warn("Stdout output: ${truncateForLog(output)}", project)
                }
            }
            result
        } catch (e: Exception) {
            MtlogLogger.error("Failed to run analyzer", project, e)
            null
        }
    }
    
    private fun parseAnalyzerOutput(output: String, filePath: String): List<AnalyzerDiagnostic> {
        val diagnostics = mutableListOf<AnalyzerDiagnostic>()
        MtlogLogger.debug("parseAnalyzerOutput called for file: $filePath, output length: ${output.length}", project)
        
        // Check if this looks like plain text stderr output (go vet format)
        // Use plain text parsing if it contains diagnostic format (even with JSON present)
        if (output.contains(".go:") && output.contains(": ")) {
            MtlogLogger.debug("Found go vet format in output, using plain text parser", project)
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
                MtlogLogger.warn("No JSON found in analyzer output", project)
                return emptyList()
            }
            
            val jsonString = output.substring(jsonStart)
            MtlogLogger.debug("Attempting to parse JSON: ${jsonString.take(200)}...", project)
            val json = JsonParser.parseString(jsonString).asJsonObject
            
            // Navigate to the mtlog diagnostics
            json.entrySet().forEach { packageEntry ->
                MtlogLogger.debug("Processing package: ${packageEntry.key}", project)
                val packageObj = packageEntry.value.asJsonObject
                if (packageObj.has("mtlog")) {
                    val mtlogArray = packageObj.getAsJsonArray("mtlog")
                MtlogLogger.debug("Found ${mtlogArray.size()} mtlog diagnostics", project)
                    
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
                        
                MtlogLogger.debug("Parsed position - file: $diagnosticFilePath, line: $lineNum, col: $columnNum", project)
                        
                        // Compare by filename only since paths might be in different formats
                        // (e.g., test output might have Unix paths while filePath has Windows paths)
                        val diagnosticFileName = Paths.get(diagnosticFilePath).fileName.toString()
                        val targetFileName = Paths.get(filePath).fileName.toString()
                MtlogLogger.debug("Comparing filenames - diagnostic: '$diagnosticFileName', target: '$targetFileName'", project)
                        
                        if (diagnosticFileName == targetFileName) {
                            val message = diagnostic.get("message").asString
                            MtlogLogger.debug("Found matching diagnostic: $message at line $lineNum, col $columnNum", project)
                            
                            // Extract diagnostic ID if present
                            val diagnosticIdMatch = Regex("^\\[(MTLOG\\d+)\\]\\s*").find(message)
                            val diagnosticId = diagnosticIdMatch?.groupValues?.get(1)
                            
                            // Check if this diagnostic is suppressed
                            if (diagnosticId != null && state.suppressedDiagnostics.contains(diagnosticId)) {
                                MtlogLogger.debug("Skipping suppressed diagnostic: $diagnosticId", project)
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
            MtlogLogger.error("Failed to parse analyzer output: $output", project, e)
        }
        
        MtlogLogger.debug("parseAnalyzerOutput returning ${diagnostics.size} diagnostics", project)
        return diagnostics
    }
    
    private fun parseStderrOutput(output: String, filePath: String): List<AnalyzerDiagnostic> {
        val diagnostics = mutableListOf<AnalyzerDiagnostic>()
        MtlogLogger.debug("parseStderrOutput called for file: $filePath", project)
        MtlogLogger.debug("parseStderrOutput raw output: $output", project)
        
        // Parse lines like:
        // examples/basic/main.go:25:2: [MTLOG006] suggestion: Error level log without error value
        // or ./test.go:12:2: [MTLOG006] suggestion: Error level log without error value
        val lines = output.lines()
        for (line in lines) {
            // Skip compilation error lines that start with package name
            if (line.contains(": undefined:") || line.contains(": cannot use") || line.contains(": undeclared name")) {
                MtlogLogger.debug("Skipping compilation error: $line", project)
                continue
            }
            
            if (!line.contains(".go:")) continue
            
            MtlogLogger.debug("Processing line: $line", project)
            
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
                MtlogLogger.warn("Failed to parse diagnostic file path: $cleanDiagnosticFile - ${e.message}", project)
                continue
            }
            
            val targetFileName = Paths.get(filePath).fileName.toString()
            
            MtlogLogger.debug("Comparing files: diagnostic='$diagnosticFileName' target='$targetFileName'", project)
            
            // Match by filename since the paths might be in different formats
            if (diagnosticFileName == targetFileName) {
                val lineNum = lineStr.toIntOrNull() ?: 1
                val columnNum = columnStr.toIntOrNull() ?: 1
                
                MtlogLogger.debug("Found diagnostic: $message at line $lineNum, col $columnNum, diagnosticId=$diagnosticId", project)
                
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
                    MtlogLogger.debug("Skipping suppressed diagnostic: $diagnosticId from suppressed list: ${state.suppressedDiagnostics}", project)
                    continue
                }
                MtlogLogger.debug("Diagnostic $diagnosticId is not suppressed, adding to results", project)
                
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
        
        MtlogLogger.debug("parseStderrOutput returning ${diagnostics.size} diagnostics", project)
        return diagnostics
    }
    
    private fun parseStdinOutput(output: String): List<AnalyzerDiagnostic> {
        val diagnostics = mutableListOf<AnalyzerDiagnostic>()
        MtlogLogger.debug("parseStdinOutput called, output length: ${output.length}", project)
        
        try {
            // Parse JSON array of diagnostics
            val jsonArray = Gson().fromJson(output, com.google.gson.JsonArray::class.java)
            
            jsonArray.forEach { element ->
                val diag = element.asJsonObject
                val lineNum = diag.get("line").asInt
                val columnNum = diag.get("column").asInt
                val message = diag.get("message").asString
                val severity = diag.get("severity").asString
                val diagnosticId = diag.get("diagnostic_id")?.asString
                
                // Extract property name from PascalCase suggestions
                val propertyName = if (message.contains("property '")) {
                    message.substringAfter("property '").substringBefore("'")
                } else null
                
                // Parse suggested fixes
                val suggestedFixes = mutableListOf<AnalyzerSuggestedFix>()
                val fixesArray = diag.get("suggestedFixes")?.asJsonArray
                if (fixesArray != null) {
                    fixesArray.forEach { fixElement ->
                        val fixObj = fixElement.asJsonObject
                        val fixMessage = fixObj.get("message").asString
                        val textEdits = mutableListOf<AnalyzerTextEdit>()
                        
                        val editsArray = fixObj.get("textEdits")?.asJsonArray
                        if (editsArray != null) {
                            editsArray.forEach { editElement ->
                                val editObj = editElement.asJsonObject
                                textEdits.add(AnalyzerTextEdit(
                                    pos = editObj.get("pos").asString,
                                    end = editObj.get("end").asString,
                                    newText = editObj.get("newText").asString
                                ))
                            }
                        }
                        
                        suggestedFixes.add(AnalyzerSuggestedFix(fixMessage, textEdits))
                    }
                }
                
                diagnostics.add(AnalyzerDiagnostic(
                    lineNumber = lineNum,
                    columnNumber = columnNum,
                    message = message,
                    severity = severity,
                    propertyName = propertyName,
                    diagnosticId = diagnosticId,
                    suggestedFixes = suggestedFixes
                ))
            }
            
            MtlogLogger.debug("parseStdinOutput returning ${diagnostics.size} diagnostics", project)
        } catch (e: Exception) {
            MtlogLogger.error("Failed to parse stdin analyzer output", project, e)
            MtlogLogger.error("Output was: ${truncateForLog(output)}", project)
        }
        
        return diagnostics
    }
    
    /**
     * Shows a notification when mtlog-analyzer is not found.
     */
    private fun showAnalyzerNotFoundNotification() {
        val notification = NotificationGroupManager.getInstance()
            .getNotificationGroup("mtlog.analyzer")
            .createNotification(
                MtlogBundle.message("notification.analyzer.not.found"),
                MtlogBundle.message("notification.analyzer.not.found.content"),
                NotificationType.WARNING
            )
        
        // Add Install action
        notification.addAction(object : NotificationAction("Install") {
            override fun actionPerformed(e: AnActionEvent, notification: com.intellij.notification.Notification) {
                InstallAnalyzerAction().actionPerformed(e)
                notification.expire()
            }
        })
        
        // Add Settings action
        notification.addAction(object : NotificationAction("Settings") {
            override fun actionPerformed(e: AnActionEvent, notification: com.intellij.notification.Notification) {
                com.intellij.openapi.options.ShowSettingsUtil.getInstance()
                    .showSettingsDialog(project, com.mtlog.analyzer.settings.MtlogSettingsConfigurable::class.java)
                notification.expire()
            }
        })
        
        notification.notify(project)
    }
}
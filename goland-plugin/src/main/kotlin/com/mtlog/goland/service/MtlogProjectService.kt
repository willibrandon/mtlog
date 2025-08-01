package com.mtlog.goland.service

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.*
import com.intellij.openapi.Disposable
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Key
import com.mtlog.goland.settings.MtlogSettingsState
import java.nio.file.Path
import java.nio.file.Paths
import java.util.concurrent.ConcurrentHashMap
import kotlin.io.path.absolutePathString
import kotlin.io.path.exists
import com.google.gson.Gson
import com.google.gson.JsonParser
import java.io.BufferedReader

data class AnalyzerDiagnostic(
    val lineNumber: Int,
    val columnNumber: Int,
    val message: String,
    val severity: String,
    val propertyName: String? = null
)

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
        private const val ANALYZER_VERSION_PREFIX = "mtlog-analyzer version:"
    }
    
    private val processPool = ConcurrentHashMap<String, OSProcessHandler>()
    private var cachedVersion: String? = null
    private var state = MtlogSettingsState()
    
    override fun getState(): MtlogSettingsState = state
    
    override fun loadState(state: MtlogSettingsState) {
        this.state = state
    }
    
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
                    handler.addProcessListener(object : ProcessAdapter() {
                        override fun onTextAvailable(event: ProcessEvent, outputType: Key<*>) {
                            if (outputType == ProcessOutputTypes.STDERR) {
                                checkVersionMismatch(event.text)
                            }
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
        if (stderr.contains(ANALYZER_VERSION_PREFIX)) {
            val versionLine = stderr.lines().find { it.contains(ANALYZER_VERSION_PREFIX) }
            val version = versionLine?.substringAfter(ANALYZER_VERSION_PREFIX)?.trim()
            
            if (version != null && cachedVersion != null && cachedVersion != version) {
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
        processPool.values.forEach { 
            try {
                it.destroyProcess()
            } catch (e: Exception) {
                LOG.error("Error destroying process", e)
            }
        }
        processPool.clear()
    }
    
    fun clearCache() {
        cachedVersion = null
        disposeProcessPool()
    }
    
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
            state.analyzerFlags.forEach { addParameter(it) }
            addParameter(filePath)
            workDirectory = Paths.get(goModPath).toFile()
            withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
        }
        
        return try {
            LOG.debug("Running command: ${commandLine.commandLineString}")
            val process = commandLine.createProcess()
            val output = process.inputStream.bufferedReader().use { it.readText() }
            val errors = process.errorStream.bufferedReader().use { it.readText() }
            
            val exitCode = process.waitFor()
            LOG.debug("Analyzer exit code: $exitCode")
            
            if (errors.isNotEmpty()) {
                LOG.warn("Analyzer stderr: $errors")
            }
            
            // mtlog-analyzer outputs to stderr
            if (errors.isNotEmpty()) {
                parseAnalyzerOutput(errors, filePath)
            } else if (output.isNotEmpty()) {
                LOG.debug("Analyzer stdout: $output")
                parseAnalyzerOutput(output, filePath)
            } else {
                LOG.debug("No analyzer output")
                emptyList()
            }
        } catch (e: Exception) {
            LOG.error("Failed to run analyzer", e)
            null
        }
    }
    
    private fun parseAnalyzerOutput(output: String, filePath: String): List<AnalyzerDiagnostic> {
        val diagnostics = mutableListOf<AnalyzerDiagnostic>()
        LOG.debug("parseAnalyzerOutput called for file: $filePath")
        
        try {
            // Find the JSON object in the output
            val jsonStart = output.indexOf('{')
            if (jsonStart == -1) {
                LOG.warn("No JSON found in analyzer output")
                return emptyList()
            }
            
            val jsonString = output.substring(jsonStart)
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
                        
                        // Normalize paths for comparison (Windows path issue - case insensitive)
                        val diagnosticPath = diagnosticFilePath.replace('\\', '/').lowercase()
                        val normalizedFilePath = filePath.replace('\\', '/').lowercase()
                LOG.debug("Normalized paths - diagnostic: $diagnosticPath, file: $normalizedFilePath")
                        
                        if (diagnosticPath == normalizedFilePath) {
                            val message = diagnostic.get("message").asString
                            LOG.debug("Found matching diagnostic: $message at line $lineNum, col $columnNum")
                            
                            val severity = when {
                                message.startsWith("suggestion:") -> "suggestion"
                                message.contains("error") -> "error"
                                message.contains("template has") && message.contains("but") && message.contains("provided") -> "error"
                                message.contains("properties but") && message.contains("argument") -> "error"
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
                                propertyName = propertyName
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
}
import * as vscode from 'vscode';
import { spawn, ChildProcess, execSync } from 'child_process';
import * as path from 'path';
import * as os from 'os';
import * as crypto from 'crypto';
import * as fs from 'fs';

// Constants
const LOG_TRUNCATION_LENGTH = 200;  // Maximum length for log output in debug messages
const STATUS_BAR_WIDTH = 23;         // Fixed width for status bar to prevent jumping
const ENV_MTLOG_SUPPRESS = 'MTLOG_SUPPRESS';  // Environment variable for diagnostic suppression
const ENV_MTLOG_DISABLE_ALL = 'MTLOG_DISABLE_ALL';  // Environment variable to disable all diagnostics

// Logging utility for consistent timestamp formatting
function logWithTimestamp(channel: vscode.OutputChannel, message: string): void {
    channel.appendLine(`[${new Date().toLocaleTimeString()}] ${message}`);
}

// Enhanced logger similar to GoLand's MtlogLogger
class VSCodeMtlogLogger {
    private static instance: VSCodeMtlogLogger;
    private outputChannel: vscode.OutputChannel;
    
    private constructor(outputChannel: vscode.OutputChannel) {
        this.outputChannel = outputChannel;
    }
    
    static initialize(outputChannel: vscode.OutputChannel): void {
        VSCodeMtlogLogger.instance = new VSCodeMtlogLogger(outputChannel);
    }
    
    static debug(message: string, showInUI: boolean = false): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] DEBUG: ${message}`);
            if (showInUI) {
                vscode.window.showInformationMessage(`[DEBUG] ${message}`);
            }
        }
    }
    
    static info(message: string, showInUI: boolean = false): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] INFO: ${message}`);
            if (showInUI) {
                vscode.window.showInformationMessage(message);
            }
        }
    }
    
    static warn(message: string, showInUI: boolean = true): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] WARN: ${message}`);
            if (showInUI) {
                vscode.window.showWarningMessage(message);
            }
        }
    }
    
    static error(message: string, error?: Error, showInUI: boolean = true): void {
        if (VSCodeMtlogLogger.instance) {
            const errorDetails = error ? ` | Error: ${error.message}\nStack: ${error.stack}` : '';
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] ERROR: ${message}${errorDetails}`);
            if (showInUI) {
                vscode.window.showErrorMessage(message);
            }
        }
    }
    
    static show(): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.show();
        }
    }
}

// Extended diagnostic interface to store fix data
interface MtlogDiagnostic extends vscode.Diagnostic {
    mtlogData?: {
        type: 'pascalCase' | 'argumentMismatch' | 'missingError';
        // For PascalCase fixes
        oldName?: string;
        newName?: string;
        // For argument mismatch
        expectedArgs?: number;
        actualArgs?: number;
        // missingError doesn't need additional data
    };
    // Store suggested fixes from analyzer
    suggestedFixes?: Array<{
        message: string;
        textEdits: Array<{
            pos: string;
            end: string;
            newText: string;
        }>;
    }>;
}

let diagnosticCollection: vscode.DiagnosticCollection;
let outputChannel: vscode.OutputChannel;
let statusBarItem: vscode.StatusBarItem;
let activeProcesses = new Map<string, ChildProcess>();
let analysisVersions = new Map<string, number>();
let analysisQueue: string[] = [];
let runningAnalyses = 0;
const maxConcurrentAnalyses = Math.max(1, os.cpus().length - 1);
let totalIssueCount = 0;

// Cache removed - was causing stale diagnostics to persist

export function activate(context: vscode.ExtensionContext) {
    // Create output channel for error logging and diagnostics
    outputChannel = vscode.window.createOutputChannel('mtlog-analyzer');
    context.subscriptions.push(outputChannel);
    
    // Initialize enhanced logger
    VSCodeMtlogLogger.initialize(outputChannel);
    VSCodeMtlogLogger.info('mtlog-analyzer extension activated');
    
    diagnosticCollection = vscode.languages.createDiagnosticCollection('mtlog');
    context.subscriptions.push(diagnosticCollection);
    
    // Create status bar item - clicking toggles Problems panel
    statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    statusBarItem.command = 'workbench.actions.view.toggleProblems';
    context.subscriptions.push(statusBarItem);
    updateStatusBar();
    
    // Register command to show output
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.showOutput', () => {
            showDiagnosticsSummary();
            outputChannel.show();
        })
    );
    
    // Register manual analysis command
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.analyzeNow', async () => {
            const document = vscode.window.activeTextEditor?.document;
            if (document && document.languageId === 'go') {
                logWithTimestamp(outputChannel, `Manual analysis triggered for ${document.fileName}`);
                diagnosticCollection.delete(document.uri);
                await analyzeDocument(document);
            }
        })
    );
    
    // Register save and analyze command (for quick fixes)
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.saveAndAnalyze', async () => {
            const document = vscode.window.activeTextEditor?.document;
            if (document && document.languageId === 'go') {
                logWithTimestamp(outputChannel, `Save and analyze triggered for ${document.fileName}`);
                await document.save();
                // Small delay to ensure file is saved
                await new Promise(resolve => setTimeout(resolve, 100));
                diagnosticCollection.delete(document.uri);
                await analyzeDocument(document);
            }
        })
    );
    
    // Register toggle diagnostics command
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.toggleDiagnostics', async () => {
            const config = vscode.workspace.getConfiguration('mtlog');
            const currentState = config.get<boolean>('diagnosticsEnabled', true);
            await config.update('diagnosticsEnabled', !currentState, vscode.ConfigurationTarget.Workspace);
            
            const newState = !currentState ? 'enabled' : 'disabled';
            vscode.window.showInformationMessage(`mtlog analyzer ${newState}`);
            logWithTimestamp(outputChannel, `Diagnostics ${newState}`);
            
            // Re-analyze all open files
            diagnosticCollection.clear();
            vscode.workspace.textDocuments.forEach(document => {
                if (document.languageId === 'go') {
                    queueAnalysis(document);
                }
            });
        })
    );
    
    // Register suppress diagnostic command
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.suppressDiagnostic', async (diagnosticId?: string) => {
            logWithTimestamp(outputChannel, `Suppress diagnostic command called with: ${diagnosticId || 'no ID'}`);
            
            // If we got a URI instead of a diagnostic ID, ignore it
            if (diagnosticId && (diagnosticId.startsWith('file://') || diagnosticId.includes('://'))) {
                logWithTimestamp(outputChannel, `Ignoring URI passed as diagnostic ID: ${diagnosticId}`);
                diagnosticId = undefined;
            }
            
            if (!diagnosticId) {
                // Try to get diagnostic ID from current cursor position
                const editor = vscode.window.activeTextEditor;
                if (editor) {
                    const diagnostics = diagnosticCollection.get(editor.document.uri);
                    logWithTimestamp(outputChannel, `Found ${diagnostics?.length || 0} diagnostics in current file`);
                    
                    if (diagnostics) {
                        const position = editor.selection.active;
                        const diagnostic = diagnostics.find(d => d.range.contains(position));
                        
                        if (diagnostic) {
                            logWithTimestamp(outputChannel, `Found diagnostic at cursor: ${diagnostic.message}`);
                            logWithTimestamp(outputChannel, `Diagnostic code: ${(diagnostic as any).code}`);
                            logWithTimestamp(outputChannel, `Diagnostic full object: ${JSON.stringify(diagnostic)}`);
                            
                            // First try to use the code property
                            if ((diagnostic as any).code) {
                                diagnosticId = String((diagnostic as any).code);
                                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Using code property: ${diagnosticId}`);
                            } else if (diagnostic.message) {
                                // Fallback to extracting from message
                                const match = diagnostic.message.match(/\[?(MTLOG\d{3})\]?/);
                                if (match) {
                                    diagnosticId = match[1];
                                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Extracted from message: ${diagnosticId}`);
                                }
                            }
                        } else {
                            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] No diagnostic found at cursor position`);
                        }
                    }
                }
                
                if (!diagnosticId) {
                    // Show output to help debug
                    outputChannel.show();
                    diagnosticId = await vscode.window.showInputBox({
                        prompt: 'Enter diagnostic ID to suppress (e.g., MTLOG001)',
                        placeHolder: 'MTLOG001'
                    });
                }
            }
            
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Will suppress diagnostic ID: ${diagnosticId || 'none'}`);
            
            if (diagnosticId) {
                const config = vscode.workspace.getConfiguration('mtlog');
                
                // Get current suppressed list (or empty array)
                const currentValue = config.inspect('suppressedDiagnostics');
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Current config value: ${JSON.stringify(currentValue)}`);
                
                // Try to get existing valid IDs
                const existing = config.get('suppressedDiagnostics');
                let suppressed: string[] = [];
                
                if (existing === undefined || existing === null) {
                    // No config exists yet, start with empty array
                    suppressed = [];
                } else if (Array.isArray(existing)) {
                    // Valid array, filter for valid IDs
                    for (const item of existing) {
                        if (typeof item === 'string' && item.startsWith('MTLOG')) {
                            suppressed.push(item);
                        }
                    }
                } else {
                    // Config is corrupted, abort to avoid data loss
                    vscode.window.showErrorMessage('Failed to parse suppressed diagnostics configuration. Suppression aborted to avoid losing existing suppressed diagnostics.');
                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] ERROR: suppressedDiagnostics config is not a valid array: ${typeof existing}. Aborting suppression.`);
                    outputChannel.show();
                    return;
                }
                
                if (!suppressed.includes(diagnosticId)) {
                    suppressed.push(diagnosticId);
                    
                    // Clear first, then set
                    await config.update('suppressedDiagnostics', undefined, vscode.ConfigurationTarget.Workspace);
                    await config.update('suppressedDiagnostics', suppressed, vscode.ConfigurationTarget.Workspace);
                    
                    vscode.window.showInformationMessage(`Suppressed diagnostic ${diagnosticId}`);
                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Saved suppressed list: [${suppressed.join(', ')}]`);
                    
                    // Re-analyze to apply suppression
                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Clearing all diagnostics and re-analyzing...`);
                    diagnosticCollection.clear();
                    
                    const goDocuments = vscode.workspace.textDocuments.filter(d => d.languageId === 'go');
                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Found ${goDocuments.length} Go files to re-analyze`);
                    
                    goDocuments.forEach(document => {
                        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Queueing re-analysis for ${document.fileName}`);
                        queueAnalysis(document);
                    });
                } else {
                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] ${diagnosticId} already in suppressed list`);
                }
            }
        })
    );
    
    // Register suppression manager command
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.showSuppressionManager', async () => {
            const config = vscode.workspace.getConfiguration('mtlog');
            const suppressed = config.get<string[]>('suppressedDiagnostics', []);
            
            if (suppressed.length === 0) {
                vscode.window.showInformationMessage('No diagnostics are currently suppressed');
                return;
            }
            
            const items = suppressed.map(id => ({
                label: id,
                description: getDiagnosticDescription(id),
                picked: true
            }));
            
            const selected = await vscode.window.showQuickPick(items, {
                canPickMany: true,
                placeHolder: 'Select diagnostics to keep suppressed (uncheck to unsuppress)'
            });
            
            if (selected !== undefined) {
                const newSuppressed = selected.map(item => item.label);
                await config.update('suppressedDiagnostics', newSuppressed, vscode.ConfigurationTarget.Workspace);
                
                const removed = suppressed.filter(id => !newSuppressed.includes(id));
                if (removed.length > 0) {
                    vscode.window.showInformationMessage(`Unsuppressed: ${removed.join(', ')}`);
                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Unsuppressed: ${removed.join(', ')}`);
                    
                    // Re-analyze to apply changes
                    diagnosticCollection.clear();
                    vscode.workspace.textDocuments.forEach(document => {
                        if (document.languageId === 'go') {
                            queueAnalysis(document);
                        }
                    });
                }
            }
        })
    );

    // Register save handler for immediate analysis
    context.subscriptions.push(
        vscode.workspace.onDidSaveTextDocument(document => {
            if (document.languageId === 'go') {
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Save triggered analysis for ${document.fileName}`);
                queueAnalysis(document);
            }
        })
    );
    
    // Register change handler with debounce
    let changeTimeout: NodeJS.Timeout | undefined;
    context.subscriptions.push(
        vscode.workspace.onDidChangeTextDocument(event => {
            if (event.document.languageId === 'go') {
                // Don't clear diagnostics if the document is clean (no dirty changes)
                if (event.document.isDirty) {
                    // Clear diagnostics immediately to prevent stale errors
                    diagnosticCollection.delete(event.document.uri);
                }
                
                // Clear existing timeout
                if (changeTimeout) {
                    clearTimeout(changeTimeout);
                }
                
                // Set new timeout for 500ms debounce
                changeTimeout = setTimeout(() => {
                    queueAnalysis(event.document);
                }, 500);
            }
        })
    );
    
    // Check if analyzer is available on activation
    checkAnalyzerAvailable();
    
    // Analyze all open Go files on activation
    vscode.workspace.textDocuments.forEach(document => {
        if (document.languageId === 'go') {
            queueAnalysis(document);
        }
    });
    
    // Listen for configuration changes
    context.subscriptions.push(
        vscode.workspace.onDidChangeConfiguration(event => {
            if (event.affectsConfiguration('mtlog')) {
                // Cache removed - no longer needed
                
                // Cancel all active processes
                for (const [file, proc] of activeProcesses) {
                    proc.kill();
                }
                activeProcesses.clear();
                
                // Re-analyze all open Go files
                vscode.workspace.textDocuments.forEach(document => {
                    if (document.languageId === 'go') {
                        queueAnalysis(document);
                    }
                });
            }
        })
    );
    
    // Register code action provider for quick fixes
    context.subscriptions.push(
        vscode.languages.registerCodeActionsProvider(
            'go',
            new MtlogCodeActionProvider(),
            {
                providedCodeActionKinds: MtlogCodeActionProvider.providedCodeActionKinds
            }
        )
    );
}

export function deactivate() {
    // Kill all active processes
    for (const [file, proc] of activeProcesses) {
        proc.kill();
    }
    activeProcesses.clear();
    analysisQueue = [];
    
    if (statusBarItem) {
        statusBarItem.hide();
    }
}

function queueAnalysis(document: vscode.TextDocument) {
    const filePath = document.fileName;
    
    // Remove from queue if already present
    const index = analysisQueue.indexOf(filePath);
    if (index !== -1) {
        analysisQueue.splice(index, 1);
    }
    
    // Add to end of queue
    analysisQueue.push(filePath);
    
    // Process queue if we have capacity
    processQueue();
}

async function processQueue() {
    while (analysisQueue.length > 0 && runningAnalyses < maxConcurrentAnalyses) {
        const filePath = analysisQueue.shift();
        if (!filePath) continue;
        
        const document = vscode.workspace.textDocuments.find(d => d.fileName === filePath);
        if (document) {
            runningAnalyses++;
            updateStatusBar();
            analyzeDocument(document).finally(() => {
                runningAnalyses--;
                updateStatusBar();
                processQueue(); // Process next in queue
            });
        }
    }
}

async function analyzeDocument(document: vscode.TextDocument) {
    const filePath = document.fileName;
    
    // Increment analysis version for this file
    const currentVersion = (analysisVersions.get(filePath) || 0) + 1;
    analysisVersions.set(filePath, currentVersion);
    
    // Clear existing diagnostics immediately to prevent phantom diagnostics
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] CLEARING diagnostics for ${document.uri.fsPath} (version ${currentVersion})`);
    diagnosticCollection.delete(document.uri);
    
    // Cancel any existing analysis for this file
    const existingProcess = activeProcesses.get(filePath);
    if (existingProcess) {
        existingProcess.kill();
        activeProcesses.delete(filePath);
    }
    
    // We need to always re-analyze to detect when problems are fixed
    // Don't use cache - it causes stale diagnostics to persist after fixes
    const fileContent = document.getText();
    
    const diagnostics: vscode.Diagnostic[] = [];
    const fileUri = document.uri;
    let fileHash = '';
    
    // Find the Go module root
    let workingDir = path.dirname(document.fileName);
    let packagePath = './...';
    try {
        const goModPath = execSync('go env GOMOD', { 
            cwd: workingDir, 
            encoding: 'utf8' 
        }).trim();
        
        if (goModPath && goModPath !== 'NUL' && goModPath !== '/dev/null') {
            workingDir = path.dirname(goModPath);
            // Get relative path from module root to current file's directory
            const relPath = path.relative(workingDir, path.dirname(document.fileName));
            packagePath = relPath ? `./${relPath.replace(/\\/g, '/')}/...` : './...';
        }
    } catch (e) {
        // Fall back to single file if not in a module
    }
    
    // Not using cache anymore - causes stale diagnostics
    
    // Get the analyzer path and flags from config
    const config = vscode.workspace.getConfiguration('mtlog');
    const analyzerPath = config.get<string>('analyzerPath', 'mtlog-analyzer');
    const analyzerFlags = config.get<string[]>('analyzerFlags', []);
    
    // Add kill switch flags based on configuration
    const diagnosticsEnabled = config.get<boolean>('diagnosticsEnabled', true);
    let suppressedDiagnostics = config.get('suppressedDiagnostics', []);
    
    // Debug: Log raw config value
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Raw suppressedDiagnostics from config: ${JSON.stringify(suppressedDiagnostics)}`);
    
    // Fix: Ensure suppressed diagnostics are strings, not objects or URIs
    // Handle both array and single value cases  
    let suppressedArray: string[] = [];
    if (Array.isArray(suppressedDiagnostics)) {
        suppressedArray = suppressedDiagnostics
            .map((d: any) => {
                if (typeof d === 'string') {
                    // Filter out URIs that got saved by mistake
                    if (!d.includes('://') && d.startsWith('MTLOG')) {
                        return d;
                    }
                }
                // Ignore objects (like URI objects)
                return null;
            })
            .filter((d): d is string => d !== null);
    }
    
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Diagnostics enabled: ${diagnosticsEnabled}`);
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Processed suppressed diagnostics: [${suppressedArray.join(', ')}]`);
    
    // Use analyzer stdin mode to get suggested fixes
    const args = ['-stdin'];
    
    // Add analyzer flags
    args.push(...analyzerFlags);
    
    // Add suppression flags
    if (!diagnosticsEnabled) {
        args.push('-disable-all');
    } else if (suppressedArray.length > 0) {
        args.push(`-suppress=${suppressedArray.join(',')}`);
    }
    
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Running analyzer: ${analyzerPath} ${args.join(' ')}`);
    if (suppressedArray.length > 0) {
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Suppressing: ${suppressedArray.join(',')}`);
    }
    
    const proc = spawn(analyzerPath, args, {
        cwd: workingDir
    });
    
    // Track this process
    activeProcesses.set(filePath, proc);
    
    // Send file content to stdin
    const fileDocument = vscode.workspace.textDocuments.find(doc => doc.uri.fsPath === filePath);
    if (fileDocument) {
        const stdinRequest = {
            filename: filePath,
            content: fileDocument.getText(),
            go_module: workingDir
        };
        
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Sending ${stdinRequest.content.length} chars to analyzer stdin`);
        proc.stdin?.write(JSON.stringify(stdinRequest));
        proc.stdin?.end();
    }
    
    // Handle stdout - analyzer outputs JSON array of diagnostics
    let stdoutBuffer = '';
    proc.stdout.on('data', data => {
        stdoutBuffer += data.toString();
    });
    
    proc.stderr.on('data', (data) => {
        const text = data.toString();
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] stderr: ${text.substring(0, LOG_TRUNCATION_LENGTH)}`);
    });
    
    proc.on('close', (code) => {
        // Clean up process tracking
        activeProcesses.delete(filePath);
        
        // Only apply diagnostics if this is the latest analysis version
        const latestVersion = analysisVersions.get(filePath) || 0;
        if (currentVersion < latestVersion) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Discarding stale analysis results (version ${currentVersion} < ${latestVersion})`);
            return;
        }
        
        // Parse JSON array from stdout
        if (stdoutBuffer.trim()) {
            try {
                const stdinDiagnostics = JSON.parse(stdoutBuffer);
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Parsed ${stdinDiagnostics.length} diagnostics from analyzer stdout`);
                
                // Convert analyzer diagnostics to VS Code diagnostics
                for (const stdinDiag of stdinDiagnostics) {
                    // Call pushDiag directly with the analyzer data
                    const posn = `${stdinDiag.filename}:${stdinDiag.line}:${stdinDiag.column}`;
                    pushDiagInternal(posn, stdinDiag.message, stdinDiag.severity, stdinDiag.suggestedFixes, fileUri, diagnostics);
                }
            } catch (error) {
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Failed to parse analyzer output: ${error}`);
            }
        }
        
        // Update diagnostics for this file
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] SETTING ${diagnostics.length} diagnostics for ${fileUri.fsPath} (version ${currentVersion})`);
        for (const diag of diagnostics) {
            outputChannel.appendLine(`  - Line ${diag.range.start.line + 1}: ${diag.message}`);
        }
        diagnosticCollection.set(fileUri, diagnostics);
        
        // Not caching anymore - it causes stale diagnostics to persist
        
        // Update total issue count
        updateTotalIssueCount();
        
        // Log analysis completion
        const relPath = vscode.workspace.asRelativePath(fileUri.fsPath);
        if (diagnostics.length > 0) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Found ${diagnostics.length} issue${diagnostics.length !== 1 ? 's' : ''} in ${relPath}`);
        } else {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] No issues found in ${relPath} (exit code: ${code})`);
        }
    });
    
    proc.on('error', (err) => {
        // Clean up process tracking
        activeProcesses.delete(filePath);
        
        // Cache removed - no longer needed
        
        vscode.window.showErrorMessage(`mtlog-analyzer: ${err.message}`);
    });
}

function pushDiagInternal(posn: string, message: string, category?: string, suggestedFixes?: any[], uri?: vscode.Uri, diagnostics?: vscode.Diagnostic[]): void {
    if (!uri || !diagnostics) return;
    
    const posnParts = posn.split(':');
    if (posnParts.length < 3) return;
    
    // Extract file, line, col
    const file = posnParts.slice(0, -2).join(':');
    const line = parseInt(posnParts[posnParts.length - 2]);
    const col = parseInt(posnParts[posnParts.length - 1]);
    
    if (path.resolve(file) !== path.resolve(uri.fsPath)) return;
    
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Raw diagnostic message: "${message}"`);
    
    // Parse severity from category or message prefix
    let severity = vscode.DiagnosticSeverity.Error;
    let cleanMessage = message;
    
    if (message.startsWith('warning:')) {
        severity = vscode.DiagnosticSeverity.Warning;
        cleanMessage = message.substring(8).trim();
    } else if (message.startsWith('suggestion:')) {
        severity = vscode.DiagnosticSeverity.Information;
        cleanMessage = message.substring(11).trim();
    } else if (message.startsWith('error:')) {
        cleanMessage = message.substring(6).trim();
    } else if (category) {
        const cat = category.toLowerCase();
        if (cat.includes('warn')) severity = vscode.DiagnosticSeverity.Warning;
        else if (cat.includes('suggest') || cat.includes('info')) severity = vscode.DiagnosticSeverity.Information;
    }
    
    // Validate line number exists in document
    const document = vscode.workspace.textDocuments.find(d => d.uri.fsPath === uri.fsPath);
    if (!document || line - 1 >= document.lineCount) {
        // Log filtered diagnostic for debugging
        if (outputChannel) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Filtered out-of-range diagnostic: line ${line} (doc has ${document?.lineCount || 0} lines)`);
        }
        return;
    }
    
    // Use actual line length for end column
    const lineText = document.lineAt(line - 1).text;
    const diag = new vscode.Diagnostic(
        new vscode.Range(line - 1, col - 1, line - 1, lineText.length),
        cleanMessage,
        severity
    ) as MtlogDiagnostic;
    
    diag.mtlogData = extractFixData(cleanMessage);
    diag.source = 'mtlog';
    
    // Store suggested fixes from analyzer if available
    if (suggestedFixes && suggestedFixes.length > 0) {
        diag.suggestedFixes = suggestedFixes;
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Diagnostic has ${suggestedFixes.length} suggested fixes from analyzer`);
    }
    
    // Extract diagnostic ID from message OR determine from content
    const idMatch = cleanMessage.match(/\[?(MTLOG\d{3})\]?/);
    let diagnosticId: string | undefined;
    
    if (idMatch) {
        diagnosticId = idMatch[1];
    } else {
        // Determine ID from message content since go vet strips it
        const msgLower = cleanMessage.toLowerCase();
        if (msgLower.includes('template has') && msgLower.includes('properties') && msgLower.includes('arguments')) {
            diagnosticId = 'MTLOG001';
        } else if (msgLower.includes('invalid format specifier')) {
            diagnosticId = 'MTLOG002';
        } else if (msgLower.includes('duplicate property')) {
            diagnosticId = 'MTLOG003';
        } else if (msgLower.includes('pascalcase')) {
            diagnosticId = 'MTLOG004';
        } else if (msgLower.includes('capturing')) {
            diagnosticId = 'MTLOG005';
        } else if (msgLower.includes('error level log without error value') || msgLower.includes('error logging without error')) {
            diagnosticId = 'MTLOG006';
        } else if (msgLower.includes('context key')) {
            diagnosticId = 'MTLOG007';
        } else if (msgLower.includes('dynamic template')) {
            diagnosticId = 'MTLOG008';
        }
    }
    
    if (diagnosticId) {
        (diag as any).code = diagnosticId;
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Set diagnostic code: ${diagnosticId} for message: ${cleanMessage.substring(0, 50)}...`);
    }
    
    diagnostics.push(diag);
}

function parseDiagnostic(line: string, uri: vscode.Uri, diagnostics: vscode.Diagnostic[]) {
    if (!line.trim()) return;
    
    let obj: any;
    try { 
        obj = JSON.parse(line); 
    } catch (e) {
        // Log malformed JSON to output channel
        if (line.trim() && outputChannel) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Failed to parse JSON: ${line}`);
            outputChannel.appendLine(`Error: ${e}`);
        }
        return;
    }
    
    // --- NEWER go vet / analysis output -------------------------------
    // Flat object: {file, line, column, message, category, suggestedFixes, ...}
    if (obj.message && (obj.posn || obj.file)) {
        const posn = obj.posn ?? `${obj.file}:${obj.line}:${obj.column ?? 1}`;
        pushDiag(posn, obj.message, obj.category, obj.suggestedFixes);
        return;
    }
    
    // Envelope form: {diagnostic:{posn,message,category,suggestedFixes,...}, analysis:"mtlog"}
    if (obj.diagnostic?.posn) {
        pushDiag(obj.diagnostic.posn, obj.diagnostic.message, obj.diagnostic.category, obj.diagnostic.suggestedFixes);
        return;
    }
    
    // --- Legacy nested object (kept for backward-compat) --------------
    for (const pkg in obj) {
        const byAnalyzer = obj[pkg];
        if (typeof byAnalyzer === 'object' && byAnalyzer !== null) {
            for (const analyzer in byAnalyzer) {
                const issues = byAnalyzer[analyzer];
                if (Array.isArray(issues)) {
                    for (const d of issues) {
                        if (d.posn && d.message) {
                            pushDiag(d.posn, d.message, d.category, d.suggestedFixes);
                        }
                    }
                }
            }
        }
    }
    
    function pushDiag(posn: string, message: string, category?: string, suggestedFixes?: any[]) {
        const posnParts = posn.split(':');
        if (posnParts.length < 3) return;
        
        // Extract file, line, col
        const file = posnParts.slice(0, -2).join(':');
        const line = parseInt(posnParts[posnParts.length - 2]);
        const col = parseInt(posnParts[posnParts.length - 1]);
        
        if (path.resolve(file) !== path.resolve(uri.fsPath)) return;
        
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Raw diagnostic message: "${message}"`);
        
        // Parse severity from category or message prefix
        let severity = vscode.DiagnosticSeverity.Error;
        let cleanMessage = message;
        
        if (message.startsWith('warning:')) {
            severity = vscode.DiagnosticSeverity.Warning;
            cleanMessage = message.substring(8).trim();
        } else if (message.startsWith('suggestion:')) {
            severity = vscode.DiagnosticSeverity.Information;
            cleanMessage = message.substring(11).trim();
        } else if (message.startsWith('error:')) {
            cleanMessage = message.substring(6).trim();
        } else if (category) {
            const cat = category.toLowerCase();
            if (cat.includes('warn')) severity = vscode.DiagnosticSeverity.Warning;
            else if (cat.includes('suggest') || cat.includes('info')) severity = vscode.DiagnosticSeverity.Information;
        }
        
        // Validate line number exists in document
        const document = vscode.workspace.textDocuments.find(d => d.uri.fsPath === uri.fsPath);
        if (!document || line - 1 >= document.lineCount) {
            // Log filtered diagnostic for debugging
            if (outputChannel) {
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Filtered out-of-range diagnostic: line ${line} (doc has ${document?.lineCount || 0} lines)`);
            }
            return;
        }
        
        // Use actual line length for end column
        const lineText = document.lineAt(line - 1).text;
        const diag = new vscode.Diagnostic(
            new vscode.Range(line - 1, col - 1, line - 1, lineText.length),
            cleanMessage,
            severity
        ) as MtlogDiagnostic;
        
        diag.mtlogData = extractFixData(cleanMessage);
        diag.source = 'mtlog';
        
        // Store suggested fixes from analyzer if available
        if (suggestedFixes && suggestedFixes.length > 0) {
            diag.suggestedFixes = suggestedFixes;
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Diagnostic has ${suggestedFixes.length} suggested fixes from analyzer`);
        }
        
        // Extract diagnostic ID from message OR determine from content
        const idMatch = cleanMessage.match(/\[?(MTLOG\d{3})\]?/);
        let diagnosticId: string | undefined;
        
        if (idMatch) {
            diagnosticId = idMatch[1];
        } else {
            // Determine ID from message content since go vet strips it
            const msgLower = cleanMessage.toLowerCase();
            if (msgLower.includes('template has') && msgLower.includes('properties') && msgLower.includes('arguments')) {
                diagnosticId = 'MTLOG001';
            } else if (msgLower.includes('invalid format specifier')) {
                diagnosticId = 'MTLOG002';
            } else if (msgLower.includes('duplicate property')) {
                diagnosticId = 'MTLOG003';
            } else if (msgLower.includes('pascalcase')) {
                diagnosticId = 'MTLOG004';
            } else if (msgLower.includes('capturing')) {
                diagnosticId = 'MTLOG005';
            } else if (msgLower.includes('error level log without error value') || msgLower.includes('error logging without error')) {
                diagnosticId = 'MTLOG006';
            } else if (msgLower.includes('context key')) {
                diagnosticId = 'MTLOG007';
            } else if (msgLower.includes('dynamic template')) {
                diagnosticId = 'MTLOG008';
            }
        }
        
        if (diagnosticId) {
            (diag as any).code = diagnosticId;
            
            // Check if this diagnostic should be suppressed
            const config = vscode.workspace.getConfiguration('mtlog');
            const suppressedDiagnostics = config.get<string[]>('suppressedDiagnostics', []);
            
            if (suppressedDiagnostics.includes(diagnosticId)) {
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Filtering out suppressed diagnostic ${diagnosticId}: ${cleanMessage.substring(0, 50)}...`);
                return; // Skip this diagnostic
            }
            
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Set diagnostic code: ${diagnosticId} for message: ${cleanMessage.substring(0, 50)}...`);
        } else {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] No diagnostic ID found in message: ${cleanMessage.substring(0, 50)}...`);
        }
        
        diagnostics.push(diag);
    }
}

function extractFixData(message: string): MtlogDiagnostic['mtlogData'] {
    // PascalCase fix: "consider using PascalCase for property 'userId'"
    const pascalMatch = message.match(/consider using PascalCase for property '(.+?)'/);
    if (pascalMatch) {
        const oldName = pascalMatch[1];
        // Convert to PascalCase
        const newName = oldName.split(/[_-]/).map(part => 
            part.charAt(0).toUpperCase() + part.slice(1).toLowerCase()
        ).join('');
        return {
            type: 'pascalCase',
            oldName: oldName,
            newName: newName
        };
    }
    
    // Argument mismatch: "template has 2 properties but 3 arguments provided"
    const argMatch = message.match(/template has (\d+) (?:property|properties) but (\d+) (?:argument|arguments) provided/);
    if (argMatch) {
        return {
            type: 'argumentMismatch',
            expectedArgs: parseInt(argMatch[1]),
            actualArgs: parseInt(argMatch[2])
        };
    }
    
    // Alternative format: "expected 2 arguments, got 3"
    const altMatch = message.match(/expected (\d+) arguments?, got (\d+)/);
    if (altMatch) {
        return {
            type: 'argumentMismatch',
            expectedArgs: parseInt(altMatch[1]),
            actualArgs: parseInt(altMatch[2])
        };
    }
    
    // Missing error parameter: "error level log without error value"
    const errorMatch = message.match(/error (?:level )?log(?:ging)? without error (?:value|parameter)/i);
    if (errorMatch) {
        return {
            type: 'missingError'
        };
    }
    
    return undefined;
}

function updateStatusBar() {
    if (!statusBarItem) return;
    
    const config = vscode.workspace.getConfiguration('mtlog');
    const diagnosticsEnabled = config.get<boolean>('diagnosticsEnabled', true);
    const suppressedDiagnostics = config.get<string[]>('suppressedDiagnostics', []);
    
    const errorCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Error);
    const warningCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Warning);
    const infoCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Information);
    
    // Always use the same structure with non-breaking spaces (U+00A0) for consistent width
    const e = errorCount || 0;
    const w = warningCount || 0; 
    const i = infoCount || 0;
    const s = suppressedDiagnostics.length || 0;
    
    let tooltip = '';
    let text = '';
    
    if (!diagnosticsEnabled) {
        // When disabled, just show the disabled icon
        text = '$(circle-slash)        '; // 17 chars + 6 spaces = STATUS_BAR_WIDTH total
        tooltip = 'mtlog disabled';
    } else if (runningAnalyses > 0 || analysisQueue.length > 0) {
        // Show spinning icon during analysis
        text = '$(sync~spin)           '; // 15 chars + 8 spaces = STATUS_BAR_WIDTH total  
        tooltip = `mtlog: Analyzing ${runningAnalyses} file(s)`;
    } else {
        // Always show the 3 main icons with counts
        const parts = [];
        parts.push(`$(error) ${e}`);
        parts.push(`$(warning) ${w}`);
        parts.push(`$(lightbulb) ${i}`);
        if (s > 0) parts.push(`$(eye-closed) ${s}`);
        
        const content = parts.join(' ');
        text = content.padEnd(STATUS_BAR_WIDTH, ' '); // Always exactly STATUS_BAR_WIDTH characters
        
        // Build tooltip
        const tooltipParts = [];
        if (e > 0) tooltipParts.push(`${e} error${e !== 1 ? 's' : ''}`);
        if (w > 0) tooltipParts.push(`${w} warning${w !== 1 ? 's' : ''}`);
        if (i > 0) tooltipParts.push(`${i} suggestion${i !== 1 ? 's' : ''}`);
        if (s > 0) tooltipParts.push(`${s} suppressed`);
        
        tooltip = tooltipParts.length > 0 
            ? `mtlog: ${tooltipParts.join(', ')}`
            : 'mtlog: No issues found';
    }
    
    statusBarItem.text = text;
    statusBarItem.tooltip = tooltip;
    statusBarItem.show();
}

function updateTotalIssueCount() {
    totalIssueCount = 0;
    diagnosticCollection.forEach((uri, diagnostics) => {
        totalIssueCount += diagnostics.length;
    });
    updateStatusBar();
}

function countIssuesBySeverity(severity: vscode.DiagnosticSeverity): number {
    let count = 0;
    diagnosticCollection.forEach((uri, diagnostics) => {
        count += diagnostics.filter(d => d.severity === severity).length;
    });
    return count;
}

function checkAnalyzerAvailable() {
    const config = vscode.workspace.getConfiguration('mtlog');
    let analyzerPath = config.get<string>('analyzerPath', 'mtlog-analyzer');

    if (analyzerPath.includes(path.sep) || analyzerPath.includes('/')) {
        if (fs.existsSync(analyzerPath)) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Analyzer found at ${analyzerPath}`);
            return;
        }
    }

    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Analyzer not found`);
    vscode.window.showInformationMessage(
        'mtlog-analyzer not found. Install it to enable real-time template validation.',
        'Install Now',
        'Not Now'
    ).then(selection => {
        if (selection === 'Install Now') {
            installAnalyzer();
        }
    });
}

async function installAnalyzer() {
    const installMethod = await vscode.window.showQuickPick(
        ['Install with Go (recommended)', 'Download pre-built binary', 'Cancel'],
        { placeHolder: 'How would you like to install mtlog-analyzer?' }
    );
    
    if (!installMethod || installMethod === 'Cancel') {
        return;
    }
    
    if (installMethod === 'Install with Go (recommended)') {
        const terminal = vscode.window.createTerminal('Install mtlog-analyzer');
        terminal.show();
        terminal.sendText('go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest');
        
        // Show message about what's happening
        vscode.window.showInformationMessage(
            'Installing mtlog-analyzer... Please wait for the installation to complete, then reload VS Code.',
            'Reload Window'
        ).then(selection => {
            if (selection === 'Reload Window') {
                vscode.commands.executeCommand('workbench.action.reloadWindow');
            }
        });
    } else {
        // Download pre-built binary
        const platform = os.platform();
        const arch = os.arch();
        let binaryUrl = '';
        let binaryName = 'mtlog-analyzer';
        
        // Check if we have a pre-built binary for this architecture
        const supportedArch = (arch === 'x64' || arch === 'amd64') ? 'amd64' : null;
        
        if (!supportedArch) {
            vscode.window.showWarningMessage(
                `Pre-built binaries are only available for amd64/x64 architecture. Your system is ${arch}.\n\nPlease install via Go instead: go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest`,
                'Install with Go'
            ).then(selection => {
                if (selection === 'Install with Go') {
                    const terminal = vscode.window.createTerminal('Install mtlog-analyzer');
                    terminal.show();
                    terminal.sendText('go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest');
                }
            });
            return;
        }
        
        if (platform === 'win32') {
            binaryName = 'mtlog-analyzer.exe';
            binaryUrl = 'https://github.com/willibrandon/mtlog/releases/latest/download/mtlog-analyzer-windows-amd64.exe';
        } else if (platform === 'darwin') {
            binaryUrl = 'https://github.com/willibrandon/mtlog/releases/latest/download/mtlog-analyzer-darwin-amd64';
        } else {
            binaryUrl = 'https://github.com/willibrandon/mtlog/releases/latest/download/mtlog-analyzer-linux-amd64';
        }
        
        vscode.window.showInformationMessage(
            `Please download mtlog-analyzer from: ${binaryUrl}\n\nThen update your VS Code settings with the path to the binary.`,
            'Open Settings'
        ).then(selection => {
            if (selection === 'Open Settings') {
                vscode.commands.executeCommand('workbench.action.openSettings', 'mtlog.analyzerPath');
            }
        });
    }
}

function showDiagnosticsSummary() {
    outputChannel.clear();
    outputChannel.appendLine('=== mtlog analyzer summary ===');
    outputChannel.appendLine('');
    
    const errorCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Error);
    const warningCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Warning);
    const infoCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Information);
    
    if (totalIssueCount === 0) {
        outputChannel.appendLine('✓ No issues found');
    } else {
        outputChannel.appendLine(`Found ${totalIssueCount} issue${totalIssueCount !== 1 ? 's' : ''}:`);
        if (errorCount > 0) outputChannel.appendLine(`  • ${errorCount} error${errorCount !== 1 ? 's' : ''}`);
        if (warningCount > 0) outputChannel.appendLine(`  • ${warningCount} warning${warningCount !== 1 ? 's' : ''}`);
        if (infoCount > 0) outputChannel.appendLine(`  • ${infoCount} suggestion${infoCount !== 1 ? 's' : ''}`);
        
        outputChannel.appendLine('');
        outputChannel.appendLine('Issues by file:');
        
        const fileIssues: Map<string, vscode.Diagnostic[]> = new Map();
        diagnosticCollection.forEach((uri, diagnostics) => {
            if (diagnostics.length > 0) {
                fileIssues.set(uri.fsPath, [...diagnostics]);
            }
        });
        
        Array.from(fileIssues.entries())
            .sort(([a], [b]) => a.localeCompare(b))
            .forEach(([file, diagnostics]) => {
                const relPath = vscode.workspace.asRelativePath(file);
                outputChannel.appendLine(`  ${relPath}: ${diagnostics.length} issue${diagnostics.length !== 1 ? 's' : ''}`);
            });
    }
    
    outputChannel.appendLine('');
    outputChannel.appendLine('View all issues in the Problems panel (Ctrl+Shift+M)');
}

// Code action provider for quick fixes
class MtlogCodeActionProvider implements vscode.CodeActionProvider {
    public static readonly providedCodeActionKinds = [
        vscode.CodeActionKind.QuickFix
    ];

    provideCodeActions(
        document: vscode.TextDocument,
        range: vscode.Range | vscode.Selection,
        context: vscode.CodeActionContext,
        token: vscode.CancellationToken
    ): vscode.CodeAction[] {
        const actions: vscode.CodeAction[] = [];
        
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Code actions requested for ${context.diagnostics.length} diagnostics`);
        
        // Collect unique diagnostic IDs to avoid duplicates
        const uniqueDiagnosticIds = new Set<string>();
        const mtlogDiagnostics: MtlogDiagnostic[] = [];
        
        // Process each diagnostic in the current range
        for (const diagnostic of context.diagnostics) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Diagnostic: source=${diagnostic.source}, code=${(diagnostic as any).code}, message=${diagnostic.message}`);
            
            if (diagnostic.source !== 'mtlog') continue;
            
            const mtlogDiag = diagnostic as MtlogDiagnostic;
            mtlogDiagnostics.push(mtlogDiag);
            
            // Map messages to diagnostic IDs
            let diagnosticId: string | undefined;
            const msg = diagnostic.message.toLowerCase();
            
            if (msg.includes('template has') && msg.includes('but') && msg.includes('provided')) {
                diagnosticId = 'MTLOG001';
            } else if (msg.includes('format specifier')) {
                diagnosticId = 'MTLOG002';
            } else if (msg.includes('duplicate property')) {
                diagnosticId = 'MTLOG003';
            } else if (msg.includes('pascalcase')) {
                diagnosticId = 'MTLOG004';
            } else if (msg.includes('capturing')) {
                diagnosticId = 'MTLOG005';
            } else if (msg.includes('error level log without error value') || msg.includes('error logging without error')) {
                diagnosticId = 'MTLOG006';
            } else if (msg.includes('context key')) {
                diagnosticId = 'MTLOG007';
            } else if (msg.includes('dynamic template')) {
                diagnosticId = 'MTLOG008';
            }
            
            if (diagnosticId) {
                uniqueDiagnosticIds.add(diagnosticId);
            }
        }
        
        // Add suppress actions for unique diagnostic IDs
        for (const diagnosticId of uniqueDiagnosticIds) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Adding suppress action for ${diagnosticId}`);
            const suppressAction = new vscode.CodeAction(
                `Suppress ${diagnosticId} diagnostic`,
                vscode.CodeActionKind.QuickFix
            );
            suppressAction.command = {
                command: 'mtlog.suppressDiagnostic',
                title: 'Suppress diagnostic',
                arguments: [diagnosticId]
            };
            suppressAction.isPreferred = true;
            actions.push(suppressAction);
        }
        
        // Add analyzer-provided quick fixes for each diagnostic
        for (const mtlogDiag of mtlogDiagnostics) {
            // Only use analyzer provided suggested fixes
            if (mtlogDiag.suggestedFixes && mtlogDiag.suggestedFixes.length > 0) {
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Using ${mtlogDiag.suggestedFixes.length} suggested fixes from analyzer`);
                for (const suggestedFix of mtlogDiag.suggestedFixes) {
                    const action = this.createActionFromAnalyzerFix(document, mtlogDiag, suggestedFix);
                    if (action) {
                        actions.push(action);
                    }
                }
            }
        }
        
        return actions;
    }
    
    private createPascalCaseFix(
        document: vscode.TextDocument,
        diagnostic: MtlogDiagnostic
    ): vscode.CodeAction {
        const { oldName, newName } = diagnostic.mtlogData!;
        
        const fix = new vscode.CodeAction(
            `Change '${oldName}' to '${newName}'`,
            vscode.CodeActionKind.QuickFix
        );
        
        fix.edit = new vscode.WorkspaceEdit();
        fix.diagnostics = [diagnostic];
        
        // Get the line containing the template
        const line = document.lineAt(diagnostic.range.start.line);
        const lineText = line.text;
        
        // Find all occurrences of {oldName} in the line
        const pattern = new RegExp(`\\{${escapeRegExp(oldName!)}\\}`, 'g');
        let match;
        
        while ((match = pattern.exec(lineText)) !== null) {
            const startPos = new vscode.Position(line.lineNumber, match.index + 1); // +1 to skip '{'
            const endPos = new vscode.Position(line.lineNumber, match.index + 1 + oldName!.length);
            fix.edit.replace(document.uri, new vscode.Range(startPos, endPos), newName!);
        }
        
        // Add command to save and reanalyze after applying the fix
        fix.command = {
            command: 'mtlog.saveAndAnalyze',
            title: 'Save and reanalyze'
        };
        
        return fix;
    }
    
    private createArgumentFix(
        document: vscode.TextDocument,
        diagnostic: MtlogDiagnostic
    ): vscode.CodeAction {
        const { expectedArgs, actualArgs } = diagnostic.mtlogData!;
        const diff = expectedArgs! - actualArgs!;
        
        const fix = new vscode.CodeAction(
            diff > 0 
                ? `Add ${diff} missing argument${diff > 1 ? 's' : ''}`
                : `Remove ${-diff} extra argument${-diff > 1 ? 's' : ''}`,
            vscode.CodeActionKind.QuickFix
        );
        
        fix.edit = new vscode.WorkspaceEdit();
        fix.diagnostics = [diagnostic];
        
        // Find the function call on this line
        const line = document.lineAt(diagnostic.range.start.line);
        const lineText = line.text;
        
        // Find closing parenthesis of the function call
        const closeParenIndex = this.findClosingParen(lineText, diagnostic.range.start.character);
        if (closeParenIndex === -1) return fix;
        
        if (diff > 0) {
            // Add missing arguments
            const insertPos = new vscode.Position(line.lineNumber, closeParenIndex);
            
            // Check if template contains {Error} or {Err} to add 'err' instead of 'nil'
            const hasErrorPlaceholder = /\{(?:Error|Err)\}/.test(lineText);
            const isErrorMethod = /\.(?:Error|E)\s*\(/.test(lineText);
            
            let args: string;
            if (hasErrorPlaceholder && isErrorMethod) {
                // If there's an {Error} placeholder and it's an Error method, add 'nil'
                // For multiple args, add nil for all including the error placeholder
                args = Array(diff).fill('nil').join(', ');
            } else {
                args = Array(diff).fill('nil').join(', ');
            }
            
            fix.edit.insert(document.uri, insertPos, `, ${args}`);
        } else {
            // Remove extra arguments
            const extraCount = -diff;
            const argsToRemove = this.findLastNArguments(lineText, diagnostic.range.start.character, extraCount);
            
            if (argsToRemove && argsToRemove.length > 0) {
                const firstArg = argsToRemove[0];
                const lastArg = argsToRemove[argsToRemove.length - 1];
                
                // Find the comma before the first argument to remove
                let deleteStart = firstArg.start;
                let searchPos = deleteStart - 1;
                const text = lineText;
                
                while (searchPos >= 0 && /\s/.test(text[searchPos])) {
                    searchPos--;
                }
                
                if (searchPos >= 0 && text[searchPos] === ',') {
                    deleteStart = searchPos;
                }
                
                const deleteRange = new vscode.Range(
                    line.lineNumber, 
                    deleteStart,
                    line.lineNumber,
                    lastArg.end
                );
                
                fix.edit.delete(document.uri, deleteRange);
            }
        }
        
        // Add command to save and reanalyze after applying the fix
        fix.command = {
            command: 'mtlog.saveAndAnalyze',
            title: 'Save and reanalyze'
        };
        
        return fix;
    }
    
    private findLastNArguments(text: string, startPos: number, count: number): Array<{start: number, end: number}> | null {
        // Parse the function call to find argument positions
        const args: Array<{start: number, end: number}> = [];
        let parenCount = 0;
        let currentArgStart = -1;
        let inString = false;
        let stringChar = '';
        
        for (let i = startPos; i < text.length; i++) {
            const char = text[i];
            
            // Handle string literals
            if ((char === '"' || char === '\'' || char === '`') && (i === 0 || text[i-1] !== '\\')) {
                if (!inString) {
                    inString = true;
                    stringChar = char;
                } else if (char === stringChar) {
                    inString = false;
                }
                continue;
            }
            
            if (!inString) {
                if (char === '(') {
                    parenCount++;
                    if (parenCount === 1 && currentArgStart === -1) {
                        currentArgStart = i + 1;
                    }
                } else if (char === ')') {
                    parenCount--;
                    if (parenCount === 0 && currentArgStart !== -1) {
                        // End of last argument
                        const argEnd = i;
                        // Trim whitespace from start
                        while (currentArgStart < argEnd && /\s/.test(text[currentArgStart])) {
                            currentArgStart++;
                        }
                        if (currentArgStart < argEnd) {
                            args.push({start: currentArgStart, end: argEnd});
                        }
                        break;
                    }
                } else if (char === ',' && parenCount === 1) {
                    // End of current argument
                    if (currentArgStart !== -1) {
                        const argEnd = i;
                        // Trim whitespace
                        while (currentArgStart < argEnd && /\s/.test(text[currentArgStart])) {
                            currentArgStart++;
                        }
                        if (currentArgStart < argEnd) {
                            args.push({start: currentArgStart, end: argEnd});
                        }
                    }
                    currentArgStart = i + 1;
                }
            }
        }
        
        // Return the last N arguments
        if (args.length >= count) {
            return args.slice(-count);
        }
        
        return null;
    }
    
    private createMissingErrorFix(
        document: vscode.TextDocument,
        diagnostic: MtlogDiagnostic
    ): vscode.CodeAction {
        // For MTLOG006, we need to add an error parameter without modifying the template
        // This matches the GoLand plugin behavior
        const fix = new vscode.CodeAction(
            'Add error parameter',
            vscode.CodeActionKind.QuickFix
        );
        
        fix.edit = new vscode.WorkspaceEdit();
        fix.diagnostics = [diagnostic];
        
        // Find the function call on this line
        const line = document.lineAt(diagnostic.range.start.line);
        const lineText = line.text;
        
        // Find closing parenthesis of the function call
        const closeParenIndex = this.findClosingParen(lineText, diagnostic.range.start.character);
        if (closeParenIndex === -1) return fix;
        
        // Find error variable in scope or use nil
        const errorVar = this.findErrorVariableInScope(document, diagnostic.range.start.line);
        const errorParam = errorVar || 'nil';
        
        // Add error parameter at the end
        const insertPos = new vscode.Position(line.lineNumber, closeParenIndex);
        const insertion = `, ${errorParam}`;
        
        fix.edit.insert(document.uri, insertPos, insertion);
        
        // If we used nil, add a TODO comment
        if (errorParam === 'nil') {
            // Check if there's already a comment on this line
            const hasComment = lineText.includes('//');
            
            if (hasComment) {
                // Put the TODO on the next line with proper indentation
                const indentMatch = lineText.match(/^(\s*)/);
                const indent = indentMatch ? indentMatch[1] : '';
                const lineEndPos = new vscode.Position(line.lineNumber, lineText.length);
                fix.edit.insert(document.uri, lineEndPos, `\n${indent}// TODO: replace nil with actual error`);
            } else {
                // No existing comment, add at end of line
                const lineEndPos = new vscode.Position(line.lineNumber, lineText.length);
                fix.edit.insert(document.uri, lineEndPos, ' // TODO: replace nil with actual error');
            }
        }
        
        // Add command to save and reanalyze after applying the fix
        fix.command = {
            command: 'mtlog.saveAndAnalyze',
            title: 'Save and reanalyze'
        };
        
        return fix;
    }
    
    private findErrorVariableInScope(document: vscode.TextDocument, lineNumber: number): string | null {
        // Work backwards from the log line to find the closest error variable in scope
        
        // First check: is the log line inside an if block that checks an error?
        const currentBlock = this.findContainingIfBlock(document, lineNumber);
        if (currentBlock) {
            const errorVar = this.extractErrorVariableFromIfCondition(document, currentBlock.ifLine);
            if (errorVar) {
                return errorVar;
            }
        }
        
        // Second check: look for recent error assignments in the same block scope
        for (let i = lineNumber - 1; i >= 0; i--) {
            const line = document.lineAt(i).text;
            
            // Stop at function boundaries
            if (line.match(/^\s*func\s+/)) {
                break;
            }
            
            // Stop if we encounter a closing brace at the same or lesser indentation
            // This indicates we've left the current scope
            const currentIndent = this.getIndentLevel(document.lineAt(lineNumber).text);
            const lineIndent = this.getIndentLevel(line);
            if (line.trim() === '}' && lineIndent <= currentIndent) {
                break;
            }
            
            // Check for ignored errors - if found, use nil
            if (line.match(/,\s*_\s*=/)) {
                return null;
            }
            
            // Look for error variable declarations
            const errorMatch = line.match(/,\s*(err|e|error|\w*[Ee]rr\w*)\s*:=/) || 
                              line.match(/^\s*(err|e|error|\w*[Ee]rr\w*)\s*:=/);
            
            if (errorMatch) {
                const varName = errorMatch[1];
                
                // Check if this error is in an if statement that would limit scope
                if (line.includes('if') && line.includes(':=')) {
                    // Only use this error if we're inside the same if block
                    const ifBlockEnd = this.findIfBlockEnd(document, i);
                    if (ifBlockEnd >= lineNumber) {
                        return varName;
                    }
                } else {
                    // Regular assignment - it should be in scope
                    return varName;
                }
            }
        }
        
        return null;
    }
    
    private findContainingIfBlock(document: vscode.TextDocument, lineNumber: number): {ifLine: number, blockEnd: number} | null {
        // Work backwards to find if we're inside an if block
        let braceCount = 0;
        
        for (let i = lineNumber; i >= 0; i--) {
            const line = document.lineAt(i).text;
            
            // Count braces backwards
            for (let j = line.length - 1; j >= 0; j--) {
                if (line[j] === '}') braceCount++;
                else if (line[j] === '{') braceCount--;
            }
            
            // If we find an if statement and we're inside its block
            if (line.includes('if') && braceCount <= 0) {
                const blockEnd = this.findIfBlockEnd(document, i);
                if (blockEnd >= lineNumber) {
                    return {ifLine: i, blockEnd};
                }
            }
            
            // Stop at function boundaries
            if (line.match(/^\s*func\s+/)) {
                break;
            }
        }
        
        return null;
    }
    
    private extractErrorVariableFromIfCondition(document: vscode.TextDocument, ifLine: number): string | null {
        const line = document.lineAt(ifLine).text;
        
        // Look for patterns like "if err != nil" or "if err := foo(); err != nil"
        const nilCheckMatch = line.match(/(\w+)\s*!=\s*nil/) || line.match(/(\w+)\s*==\s*nil/);
        if (nilCheckMatch) {
            const varName = nilCheckMatch[1];
            // Only return if it's actually an error variable name
            if (varName.toLowerCase().includes('err') || varName === 'e' || varName === 'error') {
                return varName;
            }
        }
        
        // Look for assignment in if condition - but only error variables
        const assignMatch = line.match(/if.*?(\w*[Ee]rr\w*|err|e|error)\s*:=.*?;/);
        if (assignMatch) {
            return assignMatch[1];
        }
        
        return null;
    }
    
    private findIfBlockEnd(document: vscode.TextDocument, ifLine: number): number {
        let braceCount = 0;
        let foundBrace = false;
        
        for (let i = ifLine; i < document.lineCount; i++) {
            const line = document.lineAt(i).text;
            
            for (const char of line) {
                if (char === '{') {
                    braceCount++;
                    foundBrace = true;
                } else if (char === '}') {
                    braceCount--;
                    if (braceCount === 0 && foundBrace) {
                        return i;
                    }
                }
            }
        }
        
        return document.lineCount - 1;
    }
    
    private getIndentLevel(line: string): number {
        const match = line.match(/^(\s*)/);
        return match ? match[1].length : 0;
    }
    
    private getFunctionRange(document: vscode.TextDocument, lineNumber: number): vscode.Range | null {
        // Find the function containing this line
        let funcStart = -1;
        let braceCount = 0;
        
        // Search backwards for function declaration
        for (let i = lineNumber; i >= 0; i--) {
            const line = document.lineAt(i).text;
            if (line.match(/^\s*func\s+/)) {
                funcStart = i;
                break;
            }
        }
        
        if (funcStart === -1) return null;
        
        // Find the end of the function
        let funcEnd = -1;
        for (let i = funcStart; i < document.lineCount; i++) {
            const line = document.lineAt(i).text;
            for (const char of line) {
                if (char === '{') braceCount++;
                else if (char === '}') {
                    braceCount--;
                    if (braceCount === 0) {
                        funcEnd = i;
                        break;
                    }
                }
            }
            if (funcEnd !== -1) break;
        }
        
        if (funcEnd === -1) funcEnd = document.lineCount - 1;
        
        return new vscode.Range(
            new vscode.Position(funcStart, 0),
            new vscode.Position(funcEnd, document.lineAt(funcEnd).text.length)
        );
    }
    
    private isVariableInScope(document: vscode.TextDocument, declLine: number, usageLine: number, varName: string): boolean {
        // Simple scope check - make sure the variable is still accessible
        // This is a simplified version - a full implementation would need proper scope analysis
        
        // Check if the declaration is in a nested block that doesn't contain usage
        const declLineText = document.lineAt(declLine).text;
        const declIndent = declLineText.match(/^(\s*)/)?.[1].length || 0;
        
        // If the declaration is in an if statement init, check if we're inside that if
        if (declLineText.includes('if') && declLineText.includes(':=')) {
            // Find the end of this if block
            let braceCount = 0;
            let inIf = false;
            for (let i = declLine; i <= usageLine; i++) {
                const line = document.lineAt(i).text;
                for (const char of line) {
                    if (char === '{') {
                        braceCount++;
                        inIf = true;
                    } else if (char === '}') {
                        braceCount--;
                        if (braceCount === 0 && inIf) {
                            // We've exited the if block
                            if (i < usageLine) {
                                // Usage is outside the if block
                                return false;
                            }
                        }
                    }
                }
            }
        }
        
        return true;
    }
    
    private createActionFromAnalyzerFix(
        document: vscode.TextDocument,
        diagnostic: MtlogDiagnostic,
        suggestedFix: any
    ): vscode.CodeAction | null {
        if (!suggestedFix.textEdits || suggestedFix.textEdits.length === 0) {
            return null;
        }
        
        const action = new vscode.CodeAction(
            suggestedFix.message || 'Apply fix',
            vscode.CodeActionKind.QuickFix
        );
        
        action.edit = new vscode.WorkspaceEdit();
        action.diagnostics = [diagnostic];
        
        try {
            // Apply each text edit from the analyzer
            for (const edit of suggestedFix.textEdits) {
                // Parse position from analyzer format (file:line:col)
                const startParts = edit.pos.split(':');
                const endParts = edit.end.split(':');
                
                if (startParts.length < 3 || endParts.length < 3) {
                    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Invalid position format in text edit: ${edit.pos} -> ${edit.end}`);
                    continue;
                }
                
                const startLine = parseInt(startParts[startParts.length - 2]) - 1; // Convert to 0-based
                const startCol = parseInt(startParts[startParts.length - 1]) - 1;
                const endLine = parseInt(endParts[endParts.length - 2]) - 1;
                const endCol = parseInt(endParts[endParts.length - 1]) - 1;
                
                const range = new vscode.Range(
                    new vscode.Position(startLine, startCol),
                    new vscode.Position(endLine, endCol)
                );
                
                action.edit.replace(document.uri, range, edit.newText);
                
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Text edit: ${startLine}:${startCol} -> ${endLine}:${endCol} = "${edit.newText}"`);
            }
            
            // Add command to save and reanalyze after applying the fix
            action.command = {
                command: 'mtlog.saveAndAnalyze',
                title: 'Save and reanalyze'
            };
            
            return action;
        } catch (e) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Error creating action from analyzer fix: ${e}`);
            return null;
        }
    }
    
    private findClosingParen(text: string, startPos: number): number {
        let parenCount = 0;
        let inString = false;
        let stringChar = '';
        
        for (let i = startPos; i < text.length; i++) {
            const char = text[i];
            
            // Handle string literals
            if ((char === '"' || char === '\'' || char === '`') && (i === 0 || text[i-1] !== '\\')) {
                if (!inString) {
                    inString = true;
                    stringChar = char;
                } else if (char === stringChar) {
                    inString = false;
                }
                continue;
            }
            
            if (!inString) {
                if (char === '(') parenCount++;
                else if (char === ')') {
                    parenCount--;
                    if (parenCount === 0) return i;
                }
            }
        }
        
        return -1;
    }
}

function escapeRegExp(string: string): string {
    return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function getDiagnosticDescription(id: string): string {
    const descriptions: Record<string, string> = {
        'MTLOG001': 'Template/argument count mismatch',
        'MTLOG002': 'Invalid format specifier',
        'MTLOG003': 'Duplicate property names',
        'MTLOG004': 'Property naming (PascalCase)',
        'MTLOG005': 'Missing capturing hints',
        'MTLOG006': 'Error logging without error value',
        'MTLOG007': 'Context key constant suggestion',
        'MTLOG008': 'Dynamic template warning'
    };
    return descriptions[id] || 'Unknown diagnostic';
}
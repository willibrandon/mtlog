import * as vscode from 'vscode';
import { spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import * as os from 'os';
import * as crypto from 'crypto';
import * as fs from 'fs';

let diagnosticCollection: vscode.DiagnosticCollection;
let outputChannel: vscode.OutputChannel;
let statusBarItem: vscode.StatusBarItem;
let activeProcesses = new Map<string, ChildProcess>();
let analysisQueue: string[] = [];
let runningAnalyses = 0;
const maxConcurrentAnalyses = Math.max(1, os.cpus().length - 1);
let totalIssueCount = 0;

// Cache for file hashes and their diagnostics
interface CacheEntry {
    hash: string;
    diagnostics: vscode.Diagnostic[];
    timestamp: number;
}
const diagnosticsCache = new Map<string, CacheEntry>();
const CACHE_TTL = 60 * 60 * 1000; // 1 hour

export function activate(context: vscode.ExtensionContext) {
    // Create output channel for error logging and diagnostics
    outputChannel = vscode.window.createOutputChannel('mtlog-analyzer');
    context.subscriptions.push(outputChannel);
    
    diagnosticCollection = vscode.languages.createDiagnosticCollection('mtlog');
    context.subscriptions.push(diagnosticCollection);
    
    // Create status bar item
    statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    statusBarItem.command = 'mtlog.showOutput';
    context.subscriptions.push(statusBarItem);
    updateStatusBar();
    
    // Register command to show output
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.showOutput', () => {
            showDiagnosticsSummary();
            outputChannel.show();
        })
    );
    
    // Register save handler for immediate analysis
    context.subscriptions.push(
        vscode.workspace.onDidSaveTextDocument(document => {
            if (document.languageId === 'go') {
                queueAnalysis(document);
            }
        })
    );
    
    // Register change handler with debounce
    let changeTimeout: NodeJS.Timeout | undefined;
    context.subscriptions.push(
        vscode.workspace.onDidChangeTextDocument(event => {
            if (event.document.languageId === 'go') {
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
                // Clear cache when configuration changes
                diagnosticsCache.clear();
                
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
    
    // Clear cache
    diagnosticsCache.clear();
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
    
    // Cancel any existing analysis for this file
    const existingProcess = activeProcesses.get(filePath);
    if (existingProcess) {
        existingProcess.kill();
        activeProcesses.delete(filePath);
    }
    
    // Check cache first
    const fileContent = document.getText();
    const currentHash = crypto.createHash('sha256').update(fileContent).digest('hex');
    const cacheEntry = diagnosticsCache.get(filePath);
    
    if (cacheEntry && cacheEntry.hash === currentHash) {
        const age = Date.now() - cacheEntry.timestamp;
        if (age < CACHE_TTL) {
            // Use cached diagnostics
            diagnosticCollection.set(document.uri, cacheEntry.diagnostics);
            updateTotalIssueCount();
            
            // Log cache hit
            const relPath = vscode.workspace.asRelativePath(filePath);
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Using cached results for ${relPath}`);
            return;
        }
    }
    
    const config = vscode.workspace.getConfiguration('mtlog');
    let analyzerPath = config.get<string>('analyzerPath', 'mtlog-analyzer');
    const analyzerFlags = config.get<string[]>('analyzerFlags', []);
    
    // If analyzer path is just the name, find it using 'where' command
    if (!analyzerPath.includes(path.sep) && !analyzerPath.includes('/')) {
        try {
            const { execSync } = require('child_process');
            analyzerPath = execSync(`where ${analyzerPath}`, { encoding: 'utf8' }).trim().split('\n')[0];
        } catch (e) {
            // Analyzer not found, offer to install
            const message = 'mtlog-analyzer not found. Would you like to install it?';
            vscode.window.showErrorMessage(message, 'Install mtlog-analyzer').then(selection => {
                if (selection === 'Install mtlog-analyzer') {
                    installAnalyzer();
                }
            });
            return;
        }
    }
    
    const diagnostics: vscode.Diagnostic[] = [];
    const fileUri = document.uri;
    let fileHash = '';
    
    // Find the Go module root
    let workingDir = path.dirname(document.fileName);
    let packagePath = './...';
    try {
        const { execSync } = require('child_process');
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
    
    // Store file hash for cache comparison
    fileHash = crypto.createHash('sha256').update(fileContent).digest('hex');
    
    // Analyze package or single file
    const args = ['vet', '-json', `-vettool=${analyzerPath}`, ...analyzerFlags, packagePath];
    
    
    const proc = spawn('go', args, {
        cwd: workingDir
    });
    
    // Track this process
    activeProcesses.set(filePath, proc);
    
    let buffer = '';
    let stderrBuffer = '';
    let inJson = false;
    let braceCount = 0;
    
    proc.stdout.on('data', (data) => {
        buffer += data.toString();
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        
        for (const line of lines) {
            if (line.trim()) {
                parseDiagnostic(line, fileUri, diagnostics);
            }
        }
    });
    
    proc.stderr.on('data', (data) => {
        const text = data.toString();
        
        // go vet outputs JSON to stderr, need to collect full JSON object
        for (const char of text) {
            if (char === '{') {
                if (!inJson) {
                    inJson = true;
                    stderrBuffer = '';
                }
                braceCount++;
            }
            
            if (inJson) {
                stderrBuffer += char;
                
                if (char === '}') {
                    braceCount--;
                    if (braceCount === 0) {
                        // Complete JSON object
                        parseDiagnostic(stderrBuffer, fileUri, diagnostics);
                        stderrBuffer = '';
                        inJson = false;
                    }
                }
            }
        }
    });
    
    proc.on('close', (code) => {
        // Clean up process tracking
        activeProcesses.delete(filePath);
        
        // Parse any remaining buffer
        if (buffer.trim()) {
            parseDiagnostic(buffer, fileUri, diagnostics);
        }
        
        // Update diagnostics for this file
        diagnosticCollection.set(fileUri, diagnostics);
        
        // Update cache
        diagnosticsCache.set(filePath, {
            hash: fileHash,
            diagnostics: [...diagnostics],
            timestamp: Date.now()
        });
        
        // Update total issue count
        updateTotalIssueCount();
        
        // Log analysis completion
        const relPath = vscode.workspace.asRelativePath(fileUri.fsPath);
        if (diagnostics.length > 0) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Found ${diagnostics.length} issue${diagnostics.length !== 1 ? 's' : ''} in ${relPath}`);
        } else {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] No issues found in ${relPath}`);
        }
    });
    
    proc.on('error', (err) => {
        // Clean up process tracking
        activeProcesses.delete(filePath);
        
        // Clear cache entry on error
        diagnosticsCache.delete(filePath);
        
        vscode.window.showErrorMessage(`mtlog-analyzer: ${err.message}`);
    });
}

function parseDiagnostic(line: string, uri: vscode.Uri, diagnostics: vscode.Diagnostic[]) {
    if (!line.trim()) return;
    
    try {
        const obj = JSON.parse(line);
        
        // Handle go vet JSON format
        // Format: { "packageName": { "analyzerName": [ { "posn": "file:line:col", "message": "..." } ] } }
        for (const packageName in obj) {
            const packageData = obj[packageName];
            if (packageData.mtlog) {
                for (const issue of packageData.mtlog) {
                    // Parse position format: "file:line:col"
                    const posnParts = issue.posn.split(':');
                    if (posnParts.length < 3) continue;
                    
                    // Only show diagnostics for the current file
                    const issueFile = posnParts.slice(0, -2).join(':');
                    if (path.resolve(issueFile) !== path.resolve(uri.fsPath)) continue;
                    
                    const lineNum = parseInt(posnParts[posnParts.length - 2]) - 1; // VS Code uses 0-based lines
                    const col = parseInt(posnParts[posnParts.length - 1]) - 1;      // VS Code uses 0-based columns
                    
                    // Extract severity from message prefix
                    let severity = vscode.DiagnosticSeverity.Error;
                    let message = issue.message || '';
                    
                    if (message.startsWith('warning:')) {
                        severity = vscode.DiagnosticSeverity.Warning;
                        message = message.substring(8).trim();
                    } else if (message.startsWith('suggestion:')) {
                        severity = vscode.DiagnosticSeverity.Information;
                        message = message.substring(11).trim();
                    } else if (message.startsWith('error:')) {
                        message = message.substring(6).trim();
                    }
                    
                    const diagnostic = new vscode.Diagnostic(
                        new vscode.Range(lineNum, col, lineNum, 999),
                        message,
                        severity
                    );
                    
                    diagnostic.source = 'mtlog';
                    diagnostics.push(diagnostic);
                }
            }
        }
    } catch (e) {
        // Log malformed JSON to output channel
        if (line.trim() && outputChannel) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Failed to parse JSON: ${line}`);
            outputChannel.appendLine(`Error: ${e}`);
        }
    }
}

function updateStatusBar() {
    if (!statusBarItem) return;
    
    if (runningAnalyses > 0 || analysisQueue.length > 0) {
        const queueText = analysisQueue.length > 0 ? ` (${analysisQueue.length} queued)` : '';
        statusBarItem.text = `$(sync~spin) mtlog: analyzing${queueText}`;
        statusBarItem.tooltip = `Analyzing ${runningAnalyses} file(s)${queueText}\nClick to show output`;
        statusBarItem.show();
    } else if (totalIssueCount > 0) {
        const errorCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Error);
        const warningCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Warning);
        const infoCount = countIssuesBySeverity(vscode.DiagnosticSeverity.Information);
        
        const parts = [];
        if (errorCount > 0) parts.push(`${errorCount} error${errorCount !== 1 ? 's' : ''}`);
        if (warningCount > 0) parts.push(`${warningCount} warning${warningCount !== 1 ? 's' : ''}`);
        if (infoCount > 0) parts.push(`${infoCount} suggestion${infoCount !== 1 ? 's' : ''}`);
        
        statusBarItem.text = `$(warning) mtlog: ${parts.join(', ')}`;
        statusBarItem.tooltip = 'mtlog analyzer found issues\nClick to show output';
        statusBarItem.show();
    } else {
        statusBarItem.text = '$(check) mtlog: no issues';
        statusBarItem.tooltip = 'mtlog analyzer - no issues found\nClick to show output';
        statusBarItem.show();
    }
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
    const analyzerPath = config.get<string>('analyzerPath', 'mtlog-analyzer');
    
    // If it's a full path, assume user knows what they're doing
    if (analyzerPath.includes(path.sep) || analyzerPath.includes('/')) {
        return;
    }
    
    // Check if analyzer is in PATH
    try {
        const { execSync } = require('child_process');
        execSync(`where ${analyzerPath}`, { encoding: 'utf8' });
    } catch (e) {
        // Not found, show notification
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
        
        if (platform === 'win32') {
            binaryName = 'mtlog-analyzer.exe';
            binaryUrl = `https://github.com/willibrandon/mtlog/releases/latest/download/mtlog-analyzer-windows-${arch}.exe`;
        } else if (platform === 'darwin') {
            binaryUrl = `https://github.com/willibrandon/mtlog/releases/latest/download/mtlog-analyzer-darwin-${arch}`;
        } else {
            binaryUrl = `https://github.com/willibrandon/mtlog/releases/latest/download/mtlog-analyzer-linux-${arch}`;
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
    
    // Show cache statistics
    const cacheSize = diagnosticsCache.size;
    if (cacheSize > 0) {
        outputChannel.appendLine('');
        outputChannel.appendLine(`Cache: ${cacheSize} file${cacheSize !== 1 ? 's' : ''} cached`);
    }
}
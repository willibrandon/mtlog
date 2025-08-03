import * as vscode from 'vscode';
import { spawn, ChildProcess, execSync } from 'child_process';
import * as path from 'path';
import * as os from 'os';
import * as crypto from 'crypto';
import * as fs from 'fs';

// Extended diagnostic interface to store fix data
interface MtlogDiagnostic extends vscode.Diagnostic {
    mtlogData?: {
        type: 'pascalCase' | 'argumentMismatch';
        // For PascalCase fixes
        oldName?: string;
        newName?: string;
        // For argument mismatch
        expectedArgs?: number;
        actualArgs?: number;
    };
}

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
    
    // Register manual analysis command
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.analyzeNow', async () => {
            const document = vscode.window.activeTextEditor?.document;
            if (document && document.languageId === 'go') {
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Manual analysis triggered for ${document.fileName}`);
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
                outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Save and analyze triggered for ${document.fileName}`);
                await document.save();
                // Small delay to ensure file is saved
                await new Promise(resolve => setTimeout(resolve, 100));
                diagnosticCollection.delete(document.uri);
                await analyzeDocument(document);
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
    
    // Resolve analyzer path dynamically
    if (!analyzerPath.includes(path.sep) && !analyzerPath.includes('/')) {
        const resolvedPath = resolveAnalyzerPath(analyzerPath);
        if (!resolvedPath) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Error: mtlog-analyzer not found`);
            vscode.window.showErrorMessage('mtlog-analyzer not found. Please install it or update mtlog.analyzerPath setting.');
            return;
        }
        analyzerPath = resolvedPath;
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Resolved analyzer path to ${analyzerPath}`);
    }

    if (!fs.existsSync(analyzerPath)) {
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Error: Analyzer not found at ${analyzerPath}`);
        vscode.window.showErrorMessage(`mtlog-analyzer not found at ${analyzerPath}. Please check mtlog.analyzerPath setting.`);
        return;
    }
    
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
    
    // Store file hash for cache comparison
    fileHash = crypto.createHash('sha256').update(fileContent).digest('hex');
    
    // Analyze package or single file
    // If analyzerPath is just a binary name (found in PATH), get the full path
    let fullAnalyzerPath = analyzerPath;
    if (!analyzerPath.includes('/') && !analyzerPath.includes('\\')) {
        try {
            const whichCmd = process.platform === 'win32' ? 'where' : 'which';
            fullAnalyzerPath = execSync(`${whichCmd} ${analyzerPath}`, { encoding: 'utf8' }).trim().split('\n')[0];
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Resolved ${analyzerPath} to ${fullAnalyzerPath}`);
        } catch (e) {
            // If which/where fails, stick with the binary name
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Could not resolve full path for ${analyzerPath}, using as-is`);
        }
    }
    
    const args = ['vet', '-json', `-vettool=${fullAnalyzerPath}`, ...analyzerFlags, packagePath];
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Running go vet: go ${args.join(' ')} in ${workingDir}`);
    
    const proc = spawn('go', args, {
        cwd: workingDir
    });
    
    // Track this process
    activeProcesses.set(filePath, proc);
    
    let stderrBuffer = '';
    let inJson = false;
    let braceCount = 0;
    
    // Handle stdout with brace counting for multi-line JSON
    let outJson = '', outBrace = 0, outIn = false;
    proc.stdout.on('data', data => {
        const text = data.toString();
        for (const ch of text) {
            if (ch === '{') { outBrace++; outIn = true; }
            if (outIn) outJson += ch;
            if (ch === '}') {
                if (--outBrace === 0) {           // complete object
                    parseDiagnostic(outJson, fileUri, diagnostics);
                    outJson = ''; outIn = false;
                }
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
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] No issues found in ${relPath} (exit code: ${code})`);
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
    // Flat object: {file, line, column, message, category, ...}
    if (obj.message && (obj.posn || obj.file)) {
        const posn = obj.posn ?? `${obj.file}:${obj.line}:${obj.column ?? 1}`;
        pushDiag(posn, obj.message, obj.category);
        return;
    }
    
    // Envelope form: {diagnostic:{posn,message,category,...}, analysis:"mtlog"}
    if (obj.diagnostic?.posn) {
        pushDiag(obj.diagnostic.posn, obj.diagnostic.message, obj.diagnostic.category);
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
                            pushDiag(d.posn, d.message, d.category);
                        }
                    }
                }
            }
        }
    }
    
    function pushDiag(posn: string, message: string, category?: string) {
        const posnParts = posn.split(':');
        if (posnParts.length < 3) return;
        
        // Extract file, line, col
        const file = posnParts.slice(0, -2).join(':');
        const line = parseInt(posnParts[posnParts.length - 2]);
        const col = parseInt(posnParts[posnParts.length - 1]);
        
        if (path.resolve(file) !== path.resolve(uri.fsPath)) return;
        
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
        
        const diag = new vscode.Diagnostic(
            new vscode.Range(line - 1, col - 1, line - 1, 999),
            cleanMessage,
            severity
        ) as MtlogDiagnostic;
        
        diag.mtlogData = extractFixData(cleanMessage);
        diag.source = 'mtlog';
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
    
    return undefined;
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
    let analyzerPath = config.get<string>('analyzerPath', 'mtlog-analyzer');

    if (analyzerPath.includes(path.sep) || analyzerPath.includes('/')) {
        if (fs.existsSync(analyzerPath)) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Analyzer found at ${analyzerPath}`);
            return;
        }
    }

    const resolvedPath = resolveAnalyzerPath(analyzerPath);
    if (resolvedPath) {
        config.update('analyzerPath', resolvedPath, vscode.ConfigurationTarget.Global);
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Updated analyzerPath to ${resolvedPath}`);
        return;
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

function resolveAnalyzerPath(analyzerName: string): string | null {
    const isWindows = process.platform === 'win32';
    const binaryName = isWindows ? `${analyzerName}.exe` : analyzerName;

    // Check PATH first
    try {
        execSync(`${binaryName} -V=full`, { encoding: 'utf8', stdio: 'pipe', windowsHide: true });
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Found ${binaryName} in PATH`);
        return binaryName;
    } catch (e) {
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] ${binaryName} not found in PATH: ${e}`);
    }

    // Check common Go binary locations
    const goBinPaths = [
        process.env.GOBIN,
        process.env.GOPATH && path.join(process.env.GOPATH, 'bin'),
        path.join(os.homedir(), 'go', 'bin')
    ].filter(Boolean);

    for (const binPath of goBinPaths) {
        if (!binPath) continue;
        const fullPath = path.join(binPath, binaryName);
        if (fs.existsSync(fullPath)) {
            outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Found ${binaryName} at ${fullPath}`);
            return fullPath;
        }
    }

    return null;
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
        
        // Process each diagnostic in the current range
        for (const diagnostic of context.diagnostics) {
            if (diagnostic.source !== 'mtlog') continue;
            
            const mtlogDiag = diagnostic as MtlogDiagnostic;
            if (!mtlogDiag.mtlogData) continue;
            
            switch (mtlogDiag.mtlogData.type) {
                case 'pascalCase':
                    actions.push(this.createPascalCaseFix(document, mtlogDiag));
                    break;
                case 'argumentMismatch':
                    actions.push(this.createArgumentFix(document, mtlogDiag));
                    break;
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
            const args = Array(diff).fill('nil').join(', ');
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
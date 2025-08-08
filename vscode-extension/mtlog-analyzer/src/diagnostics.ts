import * as vscode from 'vscode';
import { spawn, ChildProcess, execSync } from 'child_process';
import * as path from 'path';
import * as os from 'os';
import { MtlogDiagnostic, LOG_TRUNCATION_LENGTH } from './types';
import { findAnalyzer, checkAnalyzerAvailable } from './analyzer';
import { updateStatusBar, updateTotalIssueCount } from './statusBar';
import { VSCodeMtlogLogger } from './logger';

/**
 * Core diagnostic processing module for the mtlog analyzer.
 * Manages analysis queue, spawns analyzer processes, and parses results.
 */

export let diagnosticCollection: vscode.DiagnosticCollection;
export let outputChannel: vscode.OutputChannel;

/** Active analyzer processes by file path */
let activeProcesses = new Map<string, ChildProcess>();
/** Version tracking to discard stale results */
let analysisVersions = new Map<string, number>();
/** Queue of files waiting for analysis */
let analysisQueue: string[] = [];
/** Number of analyses currently running */
let runningAnalyses = 0;
/** Max parallel analyses (CPU cores - 1) */
const maxConcurrentAnalyses = Math.max(1, os.cpus().length - 1);

/**
 * Initialize the diagnostics module with VS Code collections.
 */
export function initializeDiagnostics(
    collection: vscode.DiagnosticCollection,
    channel: vscode.OutputChannel
): void {
    diagnosticCollection = collection;
    outputChannel = channel;
}

/**
 * Add a document to the analysis queue.
 * Removes any existing entry and adds to end for FIFO processing.
 */
export function queueAnalysis(document: vscode.TextDocument): void {
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

/**
 * Process queued analyses up to the concurrency limit.
 * Automatically called when capacity is available.
 */
async function processQueue(): Promise<void> {
    while (analysisQueue.length > 0 && runningAnalyses < maxConcurrentAnalyses) {
        const filePath = analysisQueue.shift();
        if (!filePath) continue;
        
        const document = vscode.workspace.textDocuments.find(d => d.fileName === filePath);
        if (document) {
            runningAnalyses++;
            updateStatusBar(diagnosticCollection, runningAnalyses, analysisQueue.length);
            analyzeDocument(document).finally(() => {
                runningAnalyses--;
                updateStatusBar(diagnosticCollection, runningAnalyses, analysisQueue.length);
                processQueue(); // Process next in queue
            });
        }
    }
}

/**
 * Analyze a Go file using the mtlog-analyzer binary.
 * Spawns analyzer in stdin mode, sends file content, and parses JSON diagnostics.
 * 
 * @param document - The Go document to analyze
 */
export async function analyzeDocument(document: vscode.TextDocument): Promise<void> {
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
    
    const fileContent = document.getText();
    const diagnostics: vscode.Diagnostic[] = [];
    const fileUri = document.uri;
    
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
            const relPath = path.relative(workingDir, path.dirname(document.fileName));
            packagePath = relPath ? `./${relPath.replace(/\\/g, '/')}/...` : './...';
        }
    } catch (e) {
        // Fall back to single file if not in a module
    }
    
    // Get the analyzer path using smart detection
    const analyzerPath = await findAnalyzer();
    if (!analyzerPath) {
        VSCodeMtlogLogger.error('Analyzer not found - cannot run analysis', undefined, false);
        await checkAnalyzerAvailable();
        return;
    }
    
    // Get flags from config
    const config = vscode.workspace.getConfiguration('mtlog');
    const analyzerFlags = config.get<string[]>('analyzerFlags', []);
    
    // Add kill switch flags based on configuration
    const diagnosticsEnabled = config.get<boolean>('diagnosticsEnabled', true);
    let suppressedDiagnostics = config.get('suppressedDiagnostics', []);
    
    outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Raw suppressedDiagnostics from config: ${JSON.stringify(suppressedDiagnostics)}`);
    
    // Fix: Ensure suppressed diagnostics are strings, not objects or URIs
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
                    const posn = `${stdinDiag.filename}:${stdinDiag.line}:${stdinDiag.column}`;
                    pushDiagInternal(posn, stdinDiag.message, stdinDiag.severity, stdinDiag.suggestedFixes, stdinDiag.diagnostic_id, fileUri, diagnostics);
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
        
        // Update total issue count
        updateTotalIssueCount(diagnosticCollection);
        updateStatusBar(diagnosticCollection, runningAnalyses, analysisQueue.length);
        
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
        vscode.window.showErrorMessage(`mtlog-analyzer: ${err.message}`);
    });
}

/**
 * Convert analyzer diagnostic to VS Code diagnostic.
 * Handles severity mapping and stores suggested fixes.
 * 
 * @internal
 */
function pushDiagInternal(
    posn: string,
    message: string,
    category?: string,
    suggestedFixes?: any[],
    diagnosticId?: string,
    uri?: vscode.Uri,
    diagnostics?: vscode.Diagnostic[]
): void {
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
    
    diag.source = 'mtlog';
    
    // Store suggested fixes from analyzer if available
    if (suggestedFixes && suggestedFixes.length > 0) {
        diag.suggestedFixes = suggestedFixes;
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Diagnostic has ${suggestedFixes.length} suggested fixes from analyzer`);
    }
    
    // Use diagnostic ID from analyzer if provided
    if (diagnosticId) {
        (diag as any).code = diagnosticId;
        outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] Set diagnostic code: ${diagnosticId} for message: ${cleanMessage.substring(0, 50)}...`);
    }
    
    diagnostics.push(diag);
}

/**
 * Kill all active analyzer processes.
 * Called on deactivation or configuration changes.
 */
export function cleanupProcesses(): void {
    // Kill all active processes
    for (const [file, proc] of activeProcesses) {
        proc.kill();
    }
    activeProcesses.clear();
    analysisQueue = [];
}

/** Get count of currently running analyses */
export function getRunningAnalyses(): number {
    return runningAnalyses;
}

/** Get count of queued analyses */
export function getAnalysisQueueLength(): number {
    return analysisQueue.length;
}
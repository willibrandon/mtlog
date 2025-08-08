import * as vscode from 'vscode';
import { VSCodeMtlogLogger, logWithTimestamp } from './logger';
import { registerCommands } from './commands';
import { checkAnalyzerAvailable } from './analyzer';
import { 
    initializeDiagnostics, 
    queueAnalysis, 
    cleanupProcesses,
    getRunningAnalyses,
    getAnalysisQueueLength
} from './diagnostics';
import { initializeStatusBar, updateStatusBar } from './statusBar';
import { MtlogCodeActionProvider } from './codeActions';

/**
 * Extension activation point.
 * Sets up all modules, registers handlers, and starts initial analysis.
 */
export function activate(context: vscode.ExtensionContext) {
    // Create output channel for error logging and diagnostics
    const outputChannel = vscode.window.createOutputChannel('mtlog-analyzer');
    context.subscriptions.push(outputChannel);
    
    // Initialize enhanced logger
    VSCodeMtlogLogger.initialize(outputChannel);
    VSCodeMtlogLogger.info('mtlog-analyzer extension activated');
    
    // Initialize diagnostic collection
    const diagnosticCollection = vscode.languages.createDiagnosticCollection('mtlog');
    context.subscriptions.push(diagnosticCollection);
    
    // Initialize diagnostics module
    initializeDiagnostics(diagnosticCollection, outputChannel);
    
    // Initialize status bar
    initializeStatusBar(context);
    
    // Register all commands
    registerCommands(context);

    // Register save handler for immediate analysis
    context.subscriptions.push(
        vscode.workspace.onDidSaveTextDocument(document => {
            if (document.languageId === 'go') {
                logWithTimestamp(outputChannel, `Save triggered analysis for ${document.fileName}`);
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
                // Cancel all active processes
                cleanupProcesses();
                
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
    
    // Update status bar periodically
    const statusBarUpdateInterval = setInterval(() => {
        updateStatusBar(diagnosticCollection, getRunningAnalyses(), getAnalysisQueueLength());
    }, 1000);
    
    context.subscriptions.push({
        dispose: () => clearInterval(statusBarUpdateInterval)
    });
}

/**
 * Extension deactivation point.
 * Cleans up running analyzer processes.
 */
export function deactivate() {
    cleanupProcesses();
}
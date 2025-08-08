import * as vscode from 'vscode';
import { logWithTimestamp } from './logger';
import { getDiagnosticDescription } from './utils';
import { showDiagnosticsSummary } from './statusBar';
import { 
    diagnosticCollection, 
    outputChannel, 
    analyzeDocument, 
    queueAnalysis 
} from './diagnostics';

/**
 * Register all extension commands with VS Code.
 * Commands handle manual analysis, suppression, and diagnostics toggling.
 */
export function registerCommands(context: vscode.ExtensionContext): void {
    // Register command to show output
    context.subscriptions.push(
        vscode.commands.registerCommand('mtlog.showOutput', () => {
            showDiagnosticsSummary(outputChannel, diagnosticCollection);
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
            vscode.window.showInformationMessage(`mtlog-analyzer ${newState}`);
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
}
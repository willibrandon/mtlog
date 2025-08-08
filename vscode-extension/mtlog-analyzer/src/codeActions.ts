import * as vscode from 'vscode';
import { MtlogDiagnostic } from './types';
import { outputChannel } from './diagnostics';

/**
 * Provides quick fixes for mtlog diagnostics.
 * Converts analyzer-provided fixes into VS Code code actions.
 */
export class MtlogCodeActionProvider implements vscode.CodeActionProvider {
    public static readonly providedCodeActionKinds = [
        vscode.CodeActionKind.QuickFix
    ];

    /**
     * Generate code actions for diagnostics in the given range.
     * Includes suppression actions and analyzer-provided fixes.
     */
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
            
            // Get diagnostic ID from code property (set by the analyzer)
            const diagnosticId = (diagnostic as any).code;
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
    
    /**
     * Convert an analyzer suggested fix to a VS Code code action.
     * Parses position format and creates workspace edits.
     * 
     * @returns Code action or null if fix is invalid
     */
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
}
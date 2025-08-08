import * as vscode from 'vscode';
import { STATUS_BAR_WIDTH } from './types';

let statusBarItem: vscode.StatusBarItem;
let totalIssueCount = 0;

/**
 * Create and configure the status bar item.
 * Shows diagnostic counts and analysis status.
 * 
 * @returns The created status bar item
 */
export function initializeStatusBar(context: vscode.ExtensionContext): vscode.StatusBarItem {
    statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    statusBarItem.command = 'workbench.actions.view.toggleProblems';
    context.subscriptions.push(statusBarItem);
    updateStatusBar(undefined, 0, 0);
    return statusBarItem;
}

/**
 * Update the status bar with current diagnostic counts and analysis status.
 * Shows different icons based on state: disabled, analyzing, or issue counts.
 * 
 * @param diagnosticCollection - Collection of current diagnostics (optional during init)
 * @param runningAnalyses - Number of files currently being analyzed
 * @param analysisQueueLength - Number of files waiting for analysis
 */
export function updateStatusBar(
    diagnosticCollection: vscode.DiagnosticCollection | undefined,
    runningAnalyses: number,
    analysisQueueLength: number
): void {
    if (!statusBarItem) return;
    
    const config = vscode.workspace.getConfiguration('mtlog');
    const diagnosticsEnabled = config.get<boolean>('diagnosticsEnabled', true);
    const suppressedDiagnostics = config.get<string[]>('suppressedDiagnostics', []);
    
    const errorCount = diagnosticCollection ? countIssuesBySeverity(diagnosticCollection, vscode.DiagnosticSeverity.Error) : 0;
    const warningCount = diagnosticCollection ? countIssuesBySeverity(diagnosticCollection, vscode.DiagnosticSeverity.Warning) : 0;
    const infoCount = diagnosticCollection ? countIssuesBySeverity(diagnosticCollection, vscode.DiagnosticSeverity.Information) : 0;
    
    const e = errorCount || 0;
    const w = warningCount || 0; 
    const i = infoCount || 0;
    const s = suppressedDiagnostics.length || 0;
    
    let tooltip = '';
    let text = '';
    
    if (!diagnosticsEnabled) {
        text = '$(circle-slash)        ';
        tooltip = 'mtlog disabled';
    } else if (runningAnalyses > 0 || analysisQueueLength > 0) {
        text = '$(sync~spin)           ';
        tooltip = `mtlog: Analyzing ${runningAnalyses} file(s)`;
    } else {
        const parts = [];
        parts.push(`$(error) ${e}`);
        parts.push(`$(warning) ${w}`);
        parts.push(`$(lightbulb) ${i}`);
        if (s > 0) parts.push(`$(eye-closed) ${s}`);
        
        const content = parts.join(' ');
        text = content.padEnd(STATUS_BAR_WIDTH, ' ');
        
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

/**
 * Recalculate the total issue count across all files.
 */
export function updateTotalIssueCount(diagnosticCollection: vscode.DiagnosticCollection): void {
    totalIssueCount = 0;
    diagnosticCollection.forEach((uri, diagnostics) => {
        totalIssueCount += diagnostics.length;
    });
}

/**
 * Count diagnostics matching a specific severity level.
 * 
 * @param severity - VS Code diagnostic severity (Error, Warning, Information)
 * @returns Total count across all files
 */
export function countIssuesBySeverity(
    diagnosticCollection: vscode.DiagnosticCollection,
    severity: vscode.DiagnosticSeverity
): number {
    let count = 0;
    diagnosticCollection.forEach((uri, diagnostics) => {
        count += diagnostics.filter(d => d.severity === severity).length;
    });
    return count;
}

/**
 * Get the cached total issue count.
 */
export function getTotalIssueCount(): number {
    return totalIssueCount;
}

/**
 * Display a formatted summary of all diagnostics in the output channel.
 * Groups issues by file and severity.
 */
export function showDiagnosticsSummary(
    outputChannel: vscode.OutputChannel,
    diagnosticCollection: vscode.DiagnosticCollection
): void {
    outputChannel.clear();
    outputChannel.appendLine('=== mtlog-analyzer summary ===');
    outputChannel.appendLine('');
    
    const errorCount = countIssuesBySeverity(diagnosticCollection, vscode.DiagnosticSeverity.Error);
    const warningCount = countIssuesBySeverity(diagnosticCollection, vscode.DiagnosticSeverity.Warning);
    const infoCount = countIssuesBySeverity(diagnosticCollection, vscode.DiagnosticSeverity.Information);
    
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
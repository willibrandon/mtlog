import * as vscode from 'vscode';

export interface DiagnosticParser {
    parse(line: string): vscode.Diagnostic | null;
}

export function parseDiagnosticLine(line: string, diagnostics?: vscode.Diagnostic[]): vscode.Diagnostic | null {
    if (!line.trim()) return null;
    
    try {
        const obj = JSON.parse(line);
        
        // Handle go vet JSON format
        // Format: { "packageName": { "analyzerName": [ { "posn": "file:line:col", "message": "..." } ] } }
        for (const packageName in obj) {
            const packageData = obj[packageName];
            if (packageData.mtlog) {
                for (const issue of packageData.mtlog) {
                    // Parse position format: "file:line:col"
                    const posnMatch = issue.posn.match(/:(\d+):(\d+)$/);
                    if (!posnMatch) continue;
                    
                    const lineNum = parseInt(posnMatch[1]) - 1; // VS Code uses 0-based lines
                    const col = parseInt(posnMatch[2]) - 1;      // VS Code uses 0-based columns
                    
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
                    
                    if (diagnostics) {
                        diagnostics.push(diagnostic);
                    } else {
                        return diagnostic; // For backward compatibility
                    }
                }
            }
        }
        
        return diagnostics && diagnostics.length > 0 ? diagnostics[0] : null;
    } catch (e) {
        return null;
    }
}
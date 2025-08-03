import * as assert from 'assert';
import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import { execSync } from 'child_process';
import * as os from 'os';

function ensureGoModExists(workspaceRoot: string) {
    // DO NOT MODIFY go.mod - it should already exist in the repository
    const goModPath = path.join(workspaceRoot, 'go.mod');
    if (!fs.existsSync(goModPath)) {
        throw new Error('go.mod is missing from test directory - it should be committed to the repository');
    }
}

// Resolve the path to the mtlog-analyzer binary
// This will check the PATH, GOBIN, GOPATH/bin and home directory for the binary
// If not found, it will return null
function resolveAnalyzerPath(analyzerName: string): string | null {
    const isWindows = process.platform === 'win32';
    const binaryName = isWindows ? `${analyzerName}.exe` : analyzerName;

    try {
        execSync(`${binaryName} -V=full`, { encoding: 'utf8', stdio: 'pipe', windowsHide: true });
        console.log(`Found ${binaryName} in PATH`);
        return binaryName;
    } catch (e) {
        console.log(`Could not find ${binaryName} in PATH: ${e}`);
    }

    const goBinPaths = [
        process.env.GOBIN,
        process.env.GOPATH && path.join(process.env.GOPATH, 'bin'),
        path.join(os.homedir(), 'go', 'bin')
    ].filter(Boolean);

    for (const binPath of goBinPaths) {
        if (!binPath) continue;
        const fullPath = path.join(binPath, binaryName);
        if (fs.existsSync(fullPath)) {
            console.log(`Found ${binaryName} at ${fullPath}`);
            return fullPath;
        }
    }

    console.log(`Failed to resolve ${binaryName} in PATH, GOBIN or GOPATH/bin`);
    return null;
}

suite('Quick Fix Integration Tests', () => {
    let testDir: string;
    
    suiteSetup(async function() {
        this.timeout(30000);
        const workspaceFolders = vscode.workspace.workspaceFolders;
        if (!workspaceFolders || workspaceFolders.length === 0) {
            throw new Error('No workspace folder found');
        }
        
        const workspaceRoot = workspaceFolders[0].uri.fsPath;
        
        ensureGoModExists(workspaceRoot);
        
        // Wait a bit for the extension to do initial analysis
        await new Promise(resolve => setTimeout(resolve, 3000));
        
        // Ensure analyzer path is set dynamically
        const config = vscode.workspace.getConfiguration('mtlog');
        let analyzerPath = config.get<string>('analyzerPath', 'mtlog-analyzer');
        
        // If it's already a full path, check if we need to resolve it
        if (analyzerPath.includes('\\') || analyzerPath.includes('/')) {
            if (fs.existsSync(analyzerPath)) {
                console.log(`Using existing analyzer path: ${analyzerPath}`);
            } else {
                throw new Error(`Analyzer not found at configured path: ${analyzerPath}`);
            }
        } else {
            const resolvedPath = resolveAnalyzerPath(analyzerPath);
            if (resolvedPath) {
                await config.update('analyzerPath', resolvedPath, vscode.ConfigurationTarget.Workspace);
                console.log(`Set mtlog.analyzerPath to ${resolvedPath}`);
            } else {
                throw new Error('Could not resolve mtlog-analyzer path');
            }
        }
    });
    
    suiteTeardown(() => {
        // Don't delete or modify any files
    });
    
    test('PascalCase quick fix should be available', async function() {
        this.timeout(20000);
        
        try {
            // Use URI to open the document (like the working test)
            const fileUri = vscode.Uri.file(testFile);
            const doc = await vscode.workspace.openTextDocument(fileUri);
            const editor = await vscode.window.showTextDocument(doc, { preview: false });
            
            // Force clear any cache by making a small edit and reverting it
            await editor.edit(editBuilder => {
                editBuilder.insert(new vscode.Position(0, 0), ' ');
            });
            await editor.edit(editBuilder => {
                editBuilder.delete(new vscode.Range(0, 0, 0, 1));
            });
            
            await doc.save();
            
            // Trigger analysis by re-activating the extension or forcing a workspace scan
            await vscode.commands.executeCommand('mtlog.analyzeNow');
            
            // Wait for analysis to complete
            await new Promise(resolve => setTimeout(resolve, 3000));
            
            // Debug: Check if the extension is analyzing properly
            console.log(`Test file created at: ${testFile}`);
            console.log(`Document URI: ${doc.uri.toString()}`);
            
            // Debug: Try manual analysis with go vet
            const workspaceRoot = vscode.workspace.workspaceFolders![0].uri.fsPath;
            console.log(`Workspace root: ${workspaceRoot}`);
            
            try {
                // Get the full path to mtlog-analyzer
                let analyzerPath = resolveAnalyzerPath('mtlog-analyzer') || 'mtlog-analyzer';
                
                // If it's just the binary name (found in PATH), get the full path
                if (!analyzerPath.includes('/') && !analyzerPath.includes('\\')) {
                    try {
                        const whichCmd = process.platform === 'win32' ? 'where' : 'which';
                        const fullPath = execSync(`${whichCmd} ${analyzerPath}`, { encoding: 'utf8' }).trim().split('\n')[0];
                        if (fullPath) {
                            analyzerPath = fullPath;
                        }
                    } catch (e) {
                        // which/where failed, stick with the binary name
                    }
                }
                
                console.log(`Running manual test: ${testCmd}`);
                const testOutput = execSync(testCmd, {
                    cwd: workspaceRoot,
                    encoding: 'utf8',
                    stdio: ['pipe', 'pipe', 'pipe']
                });
                console.log('Manual go vet stdout:', testOutput);
            } catch (e: any) {
                if (e.stderr) {
                    console.log('Manual go vet stderr:', e.stderr);
                }
                if (e.stdout) {
                    console.log('Manual go vet stdout:', e.stdout);
                }
            }
            
            let diagnostics: vscode.Diagnostic[] = [];
            let pascalCaseDiag: vscode.Diagnostic | undefined;
            const maxWaitTime = 15000;
            const pollInterval = 500;
            const startTime = Date.now();
            
            while (Date.now() - startTime < maxWaitTime) {
                diagnostics = vscode.languages.getDiagnostics(doc.uri);
                pascalCaseDiag = diagnostics.find(d => 
                    d.message.includes('PascalCase') && d.message.includes('user_id')
                );
                
                if (pascalCaseDiag) {
                    console.log(`Found PascalCase diagnostic after ${Date.now() - startTime}ms: ${pascalCaseDiag.message}`);
                    break;
                }
                
                console.log(`Diagnostics after ${Date.now() - startTime}ms: ${diagnostics.length}`);
                diagnostics.forEach(d => console.log(`  - ${d.message}`));
                await new Promise(resolve => setTimeout(resolve, pollInterval));
            }
            
            console.log(`Final diagnostics for ${doc.uri.toString()}: ${diagnostics.length}`);
            diagnostics.forEach(d => console.log(`  - ${d.message}`));
            
            assert.ok(pascalCaseDiag, `Should have PascalCase diagnostic, got: ${diagnostics.map(d => d.message).join(', ')}`);
            
            const codeActions = await vscode.commands.executeCommand<vscode.CodeAction[]>(
                'vscode.executeCodeActionProvider',
                doc.uri,
                pascalCaseDiag.range
            );
            
            console.log(`Found ${codeActions.length} code actions`);
            codeActions.forEach(a => console.log(`  - ${a.title}`));
            
            const quickFix = codeActions?.find(action => 
                action.title.includes("Change 'user_id' to 'UserId'")
            );
            
            assert.ok(quickFix, `Should have PascalCase quick fix, got: ${codeActions.map(a => a.title).join(', ')}`);
        } finally {
            await vscode.commands.executeCommand('workbench.action.closeActiveEditor');
        }
    });
    
    test('Argument mismatch quick fix should be available', async function() {
        this.timeout(20000);
        
        try {
            // Use URI to open the document (like the working test)
            const fileUri = vscode.Uri.file(testFile);
            const doc = await vscode.workspace.openTextDocument(fileUri);
            const editor = await vscode.window.showTextDocument(doc, { preview: false });
            
            // Force clear any cache by making a small edit and reverting it
            await editor.edit(editBuilder => {
                editBuilder.insert(new vscode.Position(0, 0), ' ');
            });
            await editor.edit(editBuilder => {
                editBuilder.delete(new vscode.Range(0, 0, 0, 1));
            });
            
            await doc.save();
            
            // Trigger analysis by re-activating the extension or forcing a workspace scan
            await vscode.commands.executeCommand('mtlog.analyzeNow');
            
            // Wait for analysis to complete
            await new Promise(resolve => setTimeout(resolve, 3000));
            
            // Debug: Check if the extension is analyzing properly
            console.log(`Test file created at: ${testFile}`);
            console.log(`Document URI: ${doc.uri.toString()}`);
            
            let diagnostics: vscode.Diagnostic[] = [];
            let argMismatchDiag: vscode.Diagnostic | undefined;
            const maxWaitTime = 15000;
            const pollInterval = 500;
            const startTime = Date.now();
            
            while (Date.now() - startTime < maxWaitTime) {
                diagnostics = vscode.languages.getDiagnostics(doc.uri);
                argMismatchDiag = diagnostics.find(d => 
                    d.message.includes('template has 2 properties but 1 arguments provided')
                );
                
                if (argMismatchDiag) {
                    console.log(`Found argument mismatch diagnostic after ${Date.now() - startTime}ms: ${argMismatchDiag.message}`);
                    break;
                }
                
                console.log(`Diagnostics after ${Date.now() - startTime}ms: ${diagnostics.length}`);
                diagnostics.forEach(d => console.log(`  - ${d.message}`));
                await new Promise(resolve => setTimeout(resolve, pollInterval));
            }
            
            console.log(`Final diagnostics for ${doc.uri.toString()}: ${diagnostics.length}`);
            diagnostics.forEach(d => console.log(`  - ${d.message}`));
            
            assert.ok(argMismatchDiag, `Should have argument mismatch diagnostic, got: ${diagnostics.map(d => d.message).join(', ')}`);
            
            const codeActions = await vscode.commands.executeCommand<vscode.CodeAction[]>(
                'vscode.executeCodeActionProvider',
                doc.uri,
                argMismatchDiag.range
            );
            
            console.log(`Found ${codeActions.length} code actions`);
            codeActions.forEach(a => console.log(`  - ${a.title}`));
            
            const quickFix = codeActions?.find(action => 
                action.title.includes('Add 1 missing argument')
            );
            
            assert.ok(quickFix, `Should have argument mismatch quick fix, got: ${codeActions.map(a => a.title).join(', ')}`);
        } finally {
            await vscode.commands.executeCommand('workbench.action.closeActiveEditor');
        }
    });
});
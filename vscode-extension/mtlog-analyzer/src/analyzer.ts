import * as vscode from 'vscode';
import { execSync } from 'child_process';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs';
import { VSCodeMtlogLogger } from './logger';

/**
 * Locate the mtlog-analyzer binary using Go's standard installation paths.
 * Checks in order: configured path, PATH, GOBIN, GOPATH/bin, platform-specific locations.
 * 
 * @returns Path to analyzer or undefined if not found
 */
export async function findAnalyzer(): Promise<string | undefined> {
    const config = vscode.workspace.getConfiguration('mtlog');
    const configuredPath = config.get<string>('analyzerPath');
    
    // 1. Check explicit setting first
    if (configuredPath && configuredPath !== 'mtlog-analyzer') {
        if (fs.existsSync(configuredPath)) {
            VSCodeMtlogLogger.debug(`Analyzer found at configured path: ${configuredPath}`);
            return configuredPath;
        }
        VSCodeMtlogLogger.warn(`Configured analyzer path not found: ${configuredPath}`, false);
    }
    
    // 2. Try to find in PATH
    try {
        const pathResult = execSync('which mtlog-analyzer', { encoding: 'utf8' }).trim();
        if (pathResult) {
            VSCodeMtlogLogger.debug(`Analyzer found in PATH: ${pathResult}`);
            return pathResult;
        }
    } catch (e) {
        // which command failed, continue searching
    }
    
    // For Windows, try where command
    if (os.platform() === 'win32') {
        try {
            const whereResult = execSync('where mtlog-analyzer', { encoding: 'utf8' }).trim().split('\n')[0];
            if (whereResult) {
                VSCodeMtlogLogger.debug(`Analyzer found via where: ${whereResult}`);
                return whereResult;
            }
        } catch (e) {
            // where command failed, continue searching
        }
    }
    
    // 3. Check Go installation locations following Go's precedence
    const goLocations: string[] = [];
    const binaryName = os.platform() === 'win32' ? 'mtlog-analyzer.exe' : 'mtlog-analyzer';
    
    // Check GOBIN first
    const gobin = process.env.GOBIN;
    if (gobin) {
        goLocations.push(path.join(gobin, binaryName));
    }
    
    // Check GOPATH/bin
    const gopath = process.env.GOPATH || path.join(os.homedir(), 'go');
    goLocations.push(path.join(gopath, 'bin', binaryName));
    
    // Check default $HOME/go/bin if different from GOPATH
    const defaultGoPath = path.join(os.homedir(), 'go', 'bin', binaryName);
    if (!goLocations.includes(defaultGoPath)) {
        goLocations.push(defaultGoPath);
    }
    
    // Platform-specific locations
    if (os.platform() === 'darwin') {
        // Homebrew locations
        goLocations.push('/usr/local/bin/mtlog-analyzer');
        goLocations.push('/opt/homebrew/bin/mtlog-analyzer');
    }
    
    // Check each location
    for (const location of goLocations) {
        if (fs.existsSync(location)) {
            VSCodeMtlogLogger.debug(`Analyzer found at: ${location}`);
            return location;
        }
    }
    
    // Log all checked locations for debugging
    VSCodeMtlogLogger.debug(`Analyzer not found. Checked locations:\n${goLocations.join('\n')}`);
    return undefined;
}

/**
 * Check if analyzer is available and prompt for installation if missing.
 * Updates the config with the found path for performance.
 */
export async function checkAnalyzerAvailable(): Promise<void> {
    const analyzerPath = await findAnalyzer();
    
    if (analyzerPath) {
        // Update the configuration with the found path for performance
        const config = vscode.workspace.getConfiguration('mtlog');
        const currentPath = config.get<string>('analyzerPath');
        if (currentPath === 'mtlog-analyzer') {
            // Only update if using default value
            await config.update('analyzerPath', analyzerPath, vscode.ConfigurationTarget.Global);
            VSCodeMtlogLogger.info(`Updated analyzer path to: ${analyzerPath}`);
        }
        return;
    }
    
    // Show detailed error message with specific paths
    const gopath = process.env.GOPATH || path.join(os.homedir(), 'go');
    const expectedPath = path.join(gopath, 'bin', os.platform() === 'win32' ? 'mtlog-analyzer.exe' : 'mtlog-analyzer');
    
    const message = `mtlog-analyzer not found in PATH or standard Go locations.\n\nExpected location: ${expectedPath}`;
    
    const selection = await vscode.window.showErrorMessage(
        message,
        'Install Now',
        'Configure Path',
        'Dismiss'
    );
    
    if (selection === 'Install Now') {
        await installAnalyzer();
    } else if (selection === 'Configure Path') {
        await vscode.commands.executeCommand('workbench.action.openSettings', 'mtlog.analyzerPath');
    }
}

/**
 * Guide the user through analyzer installation.
 * Offers Go installation, manual config, or direct installation via go install.
 */
export async function installAnalyzer(): Promise<void> {
    // Check if Go is installed first
    let goInstalled = false;
    try {
        execSync('go version', { encoding: 'utf8' });
        goInstalled = true;
    } catch (e) {
        // Go not found
    }
    
    const options = goInstalled 
        ? ['Install with Go (recommended)', 'Configure Path Manually', 'Cancel']
        : ['Configure Path Manually', 'Install Go First', 'Cancel'];
    
    const installMethod = await vscode.window.showQuickPick(
        options,
        { 
            placeHolder: goInstalled 
                ? 'How would you like to install mtlog-analyzer?' 
                : 'Go is not installed. Please install Go first or configure the path manually.'
        }
    );
    
    if (!installMethod || installMethod === 'Cancel') {
        return;
    }
    
    if (installMethod === 'Install Go First') {
        vscode.env.openExternal(vscode.Uri.parse('https://go.dev/doc/install'));
        return;
    }
    
    if (installMethod === 'Configure Path Manually') {
        await vscode.commands.executeCommand('workbench.action.openSettings', 'mtlog.analyzerPath');
        vscode.window.showInformationMessage(
            'Please set the full path to mtlog-analyzer in the settings.',
            'Show Instructions'
        ).then(selection => {
            if (selection === 'Show Instructions') {
                const gopath = process.env.GOPATH || path.join(os.homedir(), 'go');
                const expectedPath = path.join(gopath, 'bin', os.platform() === 'win32' ? 'mtlog-analyzer.exe' : 'mtlog-analyzer');
                vscode.window.showInformationMessage(
                    `After installing with 'go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest', the analyzer will be at:\n\n${expectedPath}`
                );
            }
        });
        return;
    }
    
    if (installMethod === 'Install with Go (recommended)') {
        // Show progress notification
        vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: "Installing mtlog-analyzer",
            cancellable: false
        }, async (progress) => {
            progress.report({ increment: 0, message: "Running go install..." });
            
            try {
                // Run go install
                execSync('go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest', {
                    encoding: 'utf8',
                    env: { ...process.env }
                });
                
                progress.report({ increment: 100, message: "Installation complete!" });
                
                // Try to find the installed analyzer
                const installedPath = await findAnalyzer();
                if (installedPath) {
                    const config = vscode.workspace.getConfiguration('mtlog');
                    await config.update('analyzerPath', installedPath, vscode.ConfigurationTarget.Global);
                    
                    vscode.window.showInformationMessage(
                        `mtlog-analyzer installed successfully at: ${installedPath}`,
                        'Reload Window'
                    ).then(selection => {
                        if (selection === 'Reload Window') {
                            vscode.commands.executeCommand('workbench.action.reloadWindow');
                        }
                    });
                } else {
                    vscode.window.showWarningMessage(
                        'Installation completed but analyzer not found in expected locations. You may need to reload VS Code.',
                        'Reload Window'
                    ).then(selection => {
                        if (selection === 'Reload Window') {
                            vscode.commands.executeCommand('workbench.action.reloadWindow');
                        }
                    });
                }
            } catch (error) {
                vscode.window.showErrorMessage(
                    `Failed to install mtlog-analyzer: ${error}. Please check the output for details.`
                );
                VSCodeMtlogLogger.error('Installation failed', error as Error);
            }
        });
    }
}
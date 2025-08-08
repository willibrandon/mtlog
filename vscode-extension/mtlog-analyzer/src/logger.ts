import * as vscode from 'vscode';

/**
 * Writes a timestamped message to the output channel.
 * Used for diagnostic logging throughout the extension.
 * 
 * @param channel - VS Code output channel to write to
 * @param message - Message to log
 */
export function logWithTimestamp(channel: vscode.OutputChannel, message: string): void {
    channel.appendLine(`[${new Date().toLocaleTimeString()}] ${message}`);
}

/**
 * Singleton logger for the mtlog analyzer extension.
 * Provides structured logging with optional UI notifications.
 * 
 * @example
 * ```ts
 * VSCodeMtlogLogger.initialize(outputChannel);
 * VSCodeMtlogLogger.info('Extension activated');
 * VSCodeMtlogLogger.error('Analysis failed', err, false); // Log only, no UI
 * ```
 */
export class VSCodeMtlogLogger {
    private static instance: VSCodeMtlogLogger;
    private outputChannel: vscode.OutputChannel;
    
    private constructor(outputChannel: vscode.OutputChannel) {
        this.outputChannel = outputChannel;
    }
    
    /**
     * Initialize the logger singleton. Must be called before using static methods.
     */
    static initialize(outputChannel: vscode.OutputChannel): void {
        VSCodeMtlogLogger.instance = new VSCodeMtlogLogger(outputChannel);
    }
    
    /**
     * Log debug information. Hidden from users unless showInUI is true.
     * 
     * @param showInUI - If true, also shows an info notification (default: false)
     */
    static debug(message: string, showInUI: boolean = false): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] DEBUG: ${message}`);
            if (showInUI) {
                vscode.window.showInformationMessage(`[DEBUG] ${message}`);
            }
        }
    }
    
    /**
     * Log informational messages. Useful for tracking extension flow.
     * 
     * @param showInUI - If true, also shows an info notification (default: false)
     */
    static info(message: string, showInUI: boolean = false): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] INFO: ${message}`);
            if (showInUI) {
                vscode.window.showInformationMessage(message);
            }
        }
    }
    
    /**
     * Log warnings. Shows UI notification by default.
     * 
     * @param showInUI - If true, also shows a warning notification (default: true)
     */
    static warn(message: string, showInUI: boolean = true): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] WARN: ${message}`);
            if (showInUI) {
                vscode.window.showWarningMessage(message);
            }
        }
    }
    
    /**
     * Log errors with optional stack trace. Shows UI notification by default.
     * 
     * @param error - Optional Error object for stack trace
     * @param showInUI - If true, also shows an error notification (default: true)
     */
    static error(message: string, error?: Error, showInUI: boolean = true): void {
        if (VSCodeMtlogLogger.instance) {
            const errorDetails = error ? ` | Error: ${error.message}\nStack: ${error.stack}` : '';
            VSCodeMtlogLogger.instance.outputChannel.appendLine(`[${new Date().toLocaleTimeString()}] ERROR: ${message}${errorDetails}`);
            if (showInUI) {
                vscode.window.showErrorMessage(message);
            }
        }
    }
    
    /**
     * Show the output channel to the user.
     * Useful when debugging or after errors.
     */
    static show(): void {
        if (VSCodeMtlogLogger.instance) {
            VSCodeMtlogLogger.instance.outputChannel.show();
        }
    }
}
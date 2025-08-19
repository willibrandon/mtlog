import * as vscode from 'vscode';

/**
 * Configuration constants for the mtlog analyzer extension.
 */

/** Max characters to show in debug output to prevent log flooding */
export const LOG_TRUNCATION_LENGTH = 200;

/** Fixed width for status bar to prevent UI jumping */
export const STATUS_BAR_WIDTH = 23;

/** Environment variable for suppressing diagnostics (unused but reserved) */
export const ENV_MTLOG_SUPPRESS = 'MTLOG_SUPPRESS';

/** Environment variable to disable all diagnostics (unused but reserved) */
export const ENV_MTLOG_DISABLE_ALL = 'MTLOG_DISABLE_ALL';

/**
 * Extended VS Code diagnostic that includes suggested fixes from the analyzer.
 * 
 * @example
 * ```ts
 * const diagnostic = new vscode.Diagnostic(...) as MtlogDiagnostic;
 * diagnostic.suggestedFixes = [{
 *   message: "Add missing argument",
 *   textEdits: [{ pos: "file.go:10:5", end: "file.go:10:5", newText: ", nil" }]
 * }];
 * ```
 */
export interface MtlogDiagnostic extends vscode.Diagnostic {
    /** Quick fixes provided by the Go analyzer */
    suggestedFixes?: Array<{
        /** Human-readable description of the fix */
        message: string;
        /** Text replacements to apply */
        textEdits: Array<{
            /** Start position in "file:line:col" format */
            pos: string;
            /** End position in "file:line:col" format */
            end: string;
            /** Replacement text */
            newText: string;
        }>;
    }>;
}

/**
 * Human-readable descriptions for each diagnostic code.
 * Used in the suppression manager UI.
 */
export const DIAGNOSTIC_DESCRIPTIONS: Record<string, string> = {
    'MTLOG001': 'Template/argument count mismatch',
    'MTLOG002': 'Invalid format specifier',
    'MTLOG003': 'Duplicate property names',
    'MTLOG004': 'Property naming (PascalCase)',
    'MTLOG005': 'Missing capturing hints',
    'MTLOG006': 'Error logging without error value',
    'MTLOG007': 'Context key constant suggestion',
    'MTLOG008': 'Dynamic template warning',
    // With() method diagnostics
    'MTLOG009': 'With() odd argument count',
    'MTLOG010': 'With() non-string key',
    'MTLOG011': 'With() cross-call duplicate keys',
    'MTLOG012': 'With() reserved property name',
    'MTLOG013': 'With() empty key'
};
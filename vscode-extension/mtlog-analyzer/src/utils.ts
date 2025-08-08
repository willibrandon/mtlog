import { DIAGNOSTIC_DESCRIPTIONS } from './types';

/**
 * Get a human-readable description for a diagnostic ID.
 * 
 * @param id - Diagnostic ID like "MTLOG001"
 * @returns Description or "Unknown diagnostic" if not found
 */
export function getDiagnosticDescription(id: string): string {
    return DIAGNOSTIC_DESCRIPTIONS[id] || 'Unknown diagnostic';
}

/**
 * Find the matching closing parenthesis in a string.
 * Handles nested parens and ignores parens inside string literals.
 * 
 * @param text - The text to search
 * @param startPos - Position after the opening paren
 * @returns Index of closing paren or -1 if not found
 * 
 * @example
 * ```ts
 * const text = 'log.Info("msg", foo(bar()))';
 * const closePos = findClosingParen(text, 9); // Returns 27
 * ```
 */
export function findClosingParen(text: string, startPos: number): number {
    let parenCount = 0;
    let inString = false;
    let stringChar = '';
    
    for (let i = startPos; i < text.length; i++) {
        const char = text[i];
        
        // Handle string literals (", ', `)
        if (char === '"' || char === '\'' || char === '`') {
            // Count preceding backslashes to determine if quote is escaped
            let backslashCount = 0;
            for (let j = i - 1; j >= 0 && text[j] === '\\'; j--) {
                backslashCount++;
            }
            // If even number of backslashes (including 0), quote is not escaped
            if (backslashCount % 2 === 0) {
                if (!inString) {
                    inString = true;
                    stringChar = char;
                } else if (char === stringChar) {
                    inString = false;
                }
            }
            continue;
        }
        
        if (!inString) {
            if (char === '(') parenCount++;
            else if (char === ')') {
                parenCount--;
                if (parenCount === 0) return i;
            }
        }
    }
    
    return -1;
}
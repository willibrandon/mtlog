import * as assert from 'assert';
import * as vscode from 'vscode';

// Since extractFixData is not exported, we'll need to test it through the full flow
// or duplicate the function here for unit testing
function extractFixData(message: string): any {
    // PascalCase fix: "consider using PascalCase for property 'userId'"
    const pascalMatch = message.match(/consider using PascalCase for property '(.+?)'/);
    if (pascalMatch) {
        const oldName = pascalMatch[1];
        // Convert to PascalCase
        const newName = oldName.split(/[_-]/).map(part => 
            part.charAt(0).toUpperCase() + part.slice(1).toLowerCase()
        ).join('');
        return {
            type: 'pascalCase',
            oldName: oldName,
            newName: newName
        };
    }
    
    // Argument mismatch: "template has 2 properties but 3 arguments provided"
    const argMatch = message.match(/template has (\d+) (?:property|properties) but (\d+) (?:argument|arguments) provided/);
    if (argMatch) {
        return {
            type: 'argumentMismatch',
            expectedArgs: parseInt(argMatch[1]),
            actualArgs: parseInt(argMatch[2])
        };
    }
    
    // Alternative format: "expected 2 arguments, got 3"
    const altMatch = message.match(/expected (\d+) arguments?, got (\d+)/);
    if (altMatch) {
        return {
            type: 'argumentMismatch',
            expectedArgs: parseInt(altMatch[1]),
            actualArgs: parseInt(altMatch[2])
        };
    }
    
    return undefined;
}

suite('mtlog Quick Fix Tests', () => {
    test('Extract PascalCase fix data', () => {
        const testCases = [
            {
                message: "consider using PascalCase for property 'userId'",
                expected: { type: 'pascalCase', oldName: 'userId', newName: 'Userid' }
            },
            {
                message: "consider using PascalCase for property 'user_id'",
                expected: { type: 'pascalCase', oldName: 'user_id', newName: 'UserId' }
            },
            {
                message: "consider using PascalCase for property 'user-id'",
                expected: { type: 'pascalCase', oldName: 'user-id', newName: 'UserId' }
            }
        ];

        for (const testCase of testCases) {
            const result = extractFixData(testCase.message);
            assert.deepStrictEqual(result, testCase.expected, `Failed for message: ${testCase.message}`);
        }
    });

    test('Extract argument mismatch fix data', () => {
        const testCases = [
            {
                message: "template has 2 properties but 3 arguments provided",
                expected: { type: 'argumentMismatch', expectedArgs: 2, actualArgs: 3 }
            },
            {
                message: "template has 1 property but 3 arguments provided",
                expected: { type: 'argumentMismatch', expectedArgs: 1, actualArgs: 3 }
            },
            {
                message: "template has 3 properties but 1 argument provided",
                expected: { type: 'argumentMismatch', expectedArgs: 3, actualArgs: 1 }
            },
            {
                message: "expected 2 arguments, got 3",
                expected: { type: 'argumentMismatch', expectedArgs: 2, actualArgs: 3 }
            },
            {
                message: "expected 1 argument, got 0",
                expected: { type: 'argumentMismatch', expectedArgs: 1, actualArgs: 0 }
            }
        ];

        for (const testCase of testCases) {
            const result = extractFixData(testCase.message);
            assert.deepStrictEqual(result, testCase.expected, `Failed for message: ${testCase.message}`);
        }
    });

    test('Return undefined for non-matching messages', () => {
        const testCases = [
            "some random error message",
            "template syntax error",
            "property name is fine",
            ""
        ];

        for (const message of testCases) {
            const result = extractFixData(message);
            assert.strictEqual(result, undefined, `Should return undefined for: ${message}`);
        }
    });

    test('escapeRegExp function', () => {
        function escapeRegExp(string: string): string {
            return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        }

        const testCases = [
            { input: 'hello.world', expected: 'hello\\.world' },
            { input: 'test*', expected: 'test\\*' },
            { input: 'a+b', expected: 'a\\+b' },
            { input: '[abc]', expected: '\\[abc\\]' },
            { input: 'test|other', expected: 'test\\|other' },
            { input: 'normal_text', expected: 'normal_text' }
        ];

        for (const testCase of testCases) {
            const result = escapeRegExp(testCase.input);
            assert.strictEqual(result, testCase.expected, `Failed for input: ${testCase.input}`);
        }
    });
});
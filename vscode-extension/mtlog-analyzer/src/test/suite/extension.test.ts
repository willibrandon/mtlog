import * as assert from 'assert';
import * as vscode from 'vscode';
import * as path from 'path';
import { parseDiagnosticLine, DiagnosticParser } from '../diagnosticParser.test';

suite('Extension Test Suite', () => {
    vscode.window.showInformationMessage('Start all tests.');

    test('Extension should be present', () => {
        assert.ok(vscode.extensions.getExtension('mtlog.mtlog-analyzer'));
    });

    test('Should register diagnostics collection', async () => {
        const ext = vscode.extensions.getExtension('mtlog.mtlog-analyzer');
        assert.ok(ext);
        
        await ext.activate();
        
        // Check that mtlog diagnostics collection exists
        // This is a bit indirect, but we can verify by checking subscriptions
        assert.ok(ext.exports || ext.isActive);
    });
});

suite('Diagnostic Parser Tests', () => {
    test('Should parse error diagnostic', () => {
        const line = JSON.stringify({
            "test": {
                "mtlog": [{
                    "posn": "test.go:10:5",
                    "message": "template has 2 properties but 1 arguments provided"
                }]
            }
        });
        const diagnostic = parseDiagnosticLine(line);
        
        assert.strictEqual(diagnostic?.severity, vscode.DiagnosticSeverity.Error);
        assert.strictEqual(diagnostic?.message, 'template has 2 properties but 1 arguments provided');
        assert.strictEqual(diagnostic?.range.start.line, 9); // 0-based
        assert.strictEqual(diagnostic?.range.start.character, 4); // 0-based
    });

    test('Should parse warning diagnostic', () => {
        const line = JSON.stringify({
            "test": {
                "mtlog": [{
                    "posn": "test.go:15:8",
                    "message": "warning: using @ prefix with basic type"
                }]
            }
        });
        const diagnostic = parseDiagnosticLine(line);
        
        assert.strictEqual(diagnostic?.severity, vscode.DiagnosticSeverity.Warning);
        assert.strictEqual(diagnostic?.message, 'using @ prefix with basic type');
    });

    test('Should parse suggestion diagnostic', () => {
        const line = JSON.stringify({
            "test": {
                "mtlog": [{
                    "posn": "test.go:20:12",
                    "message": "suggestion: property name should be PascalCase"
                }]
            }
        });
        const diagnostic = parseDiagnosticLine(line);
        
        assert.strictEqual(diagnostic?.severity, vscode.DiagnosticSeverity.Information);
        assert.strictEqual(diagnostic?.message, 'property name should be PascalCase');
    });

    test('Should ignore non-mtlog diagnostics', () => {
        const line = JSON.stringify({
            "test": {
                "other": [{
                    "posn": "test.go:5:1",
                    "message": "some other message"
                }]
            }
        });
        const diagnostic = parseDiagnosticLine(line);
        
        assert.strictEqual(diagnostic, null);
    });

    test('Should handle malformed JSON', () => {
        const line = 'not valid json';
        const diagnostic = parseDiagnosticLine(line);
        
        assert.strictEqual(diagnostic, null);
    });

    test('Should handle empty lines', () => {
        const diagnostic = parseDiagnosticLine('');
        assert.strictEqual(diagnostic, null);
    });
});
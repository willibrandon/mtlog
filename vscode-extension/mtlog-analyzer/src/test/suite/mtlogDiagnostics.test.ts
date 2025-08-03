import * as vscode from "vscode";
import * as assert from "assert";
import * as path from "path";
import * as fs from "fs";
import * as os from "os";
import { execSync } from "child_process";

/**
 * Integration tests for the mtlog‑analyzer VS Code extension.
 *
 * The test suite opens a Go fixture that is known to produce three
 * diagnostics, waits for the extension to publish them, and verifies
 * message text as well as DiagnosticSeverity for each entry.
 */
suite("mtlog‑analyzer diagnostics", () => {
  // Replace with the real ID from your package.json (publisher.name)
  const EXTENSION_ID = "mtlog.mtlog-analyzer";

  // Polling parameters – tweak if your CI is slow
  const POLL_INTERVAL_MS = 250;
  const TIMEOUT_MS = 10_000;

  let document: vscode.TextDocument;

  /**
   * Helper that polls vscode.languages.getDiagnostics until the expected
   * number of diagnostics are available or the timeout expires.
   */
  async function waitForDiagnostics(
    uri: vscode.Uri,
    expected: number,
  ): Promise<vscode.Diagnostic[]> {
    const start = Date.now();
    while (Date.now() - start < TIMEOUT_MS) {
      const diags = vscode.languages.getDiagnostics(uri);
      if (diags.length >= expected) {
        return diags;
      }
      await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
    }

    throw new Error(
      `Timed out after ${TIMEOUT_MS} ms waiting for ${expected} diagnostics (found ${vscode.languages.getDiagnostics(uri).length}).`,
    );
  }

  suiteSetup(async function() {
    this.timeout(30000); // Increase timeout
    // Ensure the extension is loaded before we open the file
    const extension = vscode.extensions.getExtension(EXTENSION_ID);
    assert.ok(extension, `Extension ${EXTENSION_ID} not found`);
    await extension!.activate();
    
    // Find the actual analyzer path dynamically
    let analyzerPath = 'mtlog-analyzer';
    const platform = process.platform;
    const isWindows = platform === 'win32';
    const binaryName = isWindows ? 'mtlog-analyzer.exe' : 'mtlog-analyzer';
    
    // Try to find in PATH first
    const whereCmd = isWindows ? 'where' : 'which';
    try {
      analyzerPath = execSync(`${whereCmd} ${binaryName}`, { encoding: 'utf8' }).trim().split('\n')[0];
    } catch (e) {
      // If not in PATH, check common Go binary locations
      const goPaths = [
        process.env.GOPATH ? path.join(process.env.GOPATH, 'bin') : null,
        path.join(os.homedir(), 'go', 'bin'),
        isWindows ? 'C:\\Go\\bin' : '/usr/local/go/bin'
      ].filter(Boolean);
      
      for (const goPath of goPaths) {
        if (goPath) {
          const fullPath = path.join(goPath, binaryName);
          if (fs.existsSync(fullPath)) {
            analyzerPath = fullPath;
            break;
          }
        }
      }
    }
    
    // Set the analyzer path explicitly to the full path
    const config = vscode.workspace.getConfiguration('mtlog');
    await config.update('analyzerPath', analyzerPath, vscode.ConfigurationTarget.Workspace);
    
    // Wait for the extension to react to configuration change
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Open and display the fixture using absolute path
    const fixturePath = path.join(__dirname, '..', '..', '..', FIXTURE_RELATIVE);
    console.log('Opening fixture at:', fixturePath);
    
    // Check workspace folders
    console.log('Workspace folders:', vscode.workspace.workspaceFolders?.map(f => f.uri.toString()));
    
    // Use URI to open the document
    const fileUri = vscode.Uri.file(fixturePath);
    document = await vscode.workspace.openTextDocument(fileUri);
    console.log('Document language:', document.languageId);
    console.log('Document URI:', document.uri.toString());
    
    // Make sure it's the active editor
    const editor = await vscode.window.showTextDocument(document, { preview: false });
    
    // Force clear any cache by making a small edit and reverting it
    await editor.edit(editBuilder => {
      editBuilder.insert(new vscode.Position(0, 0), ' ');
    });
    await editor.edit(editBuilder => {
      editBuilder.delete(new vscode.Range(0, 0, 0, 1));
    });

    // Save to trigger analysis
    await document.save();
    console.log('Document saved');
    
    // Wait for analysis
    await new Promise(resolve => setTimeout(resolve, 3000));
  });

  test("publishes three diagnostics with correct severities", async function() {
    this.timeout(30000); // Increase timeout to 30 seconds
    
    // Log initial diagnostics
    console.log('Initial diagnostics:', vscode.languages.getDiagnostics(document.uri).length);
    
    // Try to show the output channel to see what's happening
    try {
      await vscode.commands.executeCommand('mtlog.showOutput');
      
      // Get the output channel text
      const outputChannels = (vscode as any).window.outputChannels;
      console.log('Output channels:', outputChannels);
    } catch (e) {
      console.log('Could not show output:', e);
    }
    
    // Check extension configuration
    const config = vscode.workspace.getConfiguration('mtlog');
    console.log('Analyzer path config:', config.get('analyzerPath'));
    console.log('Analyzer flags config:', config.get('analyzerFlags'));
    
    // Check all diagnostics collections
    const allDiagnostics = vscode.languages.getDiagnostics();
    console.log('All diagnostics URIs:', allDiagnostics.map(([uri, diags]) => uri.toString()));
    
    // Trigger a save again to ensure analysis runs
    await document.save();
    
    // Wait a bit for analysis to start
    await new Promise(resolve => setTimeout(resolve, 2000));
    
    console.log('Diagnostics after save:', vscode.languages.getDiagnostics(document.uri).length);
    
    const diagnostics = await waitForDiagnostics(document.uri, 5);
    assert.strictEqual(
      diagnostics.length,
      5,
      `Expected 5 diagnostics but got ${diagnostics.length}`,
    );

    const templateError = diagnostics.find((d) =>
      /template has 2 properties/i.test(d.message)
    );
    const prefixWarning = diagnostics.find((d) =>
      /using @ prefix/i.test(d.message)
    );
    const pascalSuggestion = diagnostics.find((d) =>
      /PascalCase/i.test(d.message)
    );

    assert.ok(templateError, "Template mismatch error not reported");
    assert.ok(prefixWarning, "@‑prefix warning not reported");
    assert.ok(pascalSuggestion, "PascalCase suggestion not reported");

    assert.strictEqual(
      templateError!.severity,
      vscode.DiagnosticSeverity.Error,
      "Unexpected severity for template error",
    );
    assert.strictEqual(
      prefixWarning!.severity,
      vscode.DiagnosticSeverity.Warning,
      "Unexpected severity for @‑prefix warning",
    );
    assert.ok(
      pascalSuggestion!.severity === vscode.DiagnosticSeverity.Information ||
        pascalSuggestion!.severity === vscode.DiagnosticSeverity.Hint,
      `Unexpected severity for PascalCase suggestion: ${pascalSuggestion!.severity}`,
    );
  });

  suiteTeardown(async () => {
    await vscode.commands.executeCommand("workbench.action.closeAllEditors");
  });
});
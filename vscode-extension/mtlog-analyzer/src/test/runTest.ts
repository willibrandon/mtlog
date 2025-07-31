import * as path from 'path';
import { runTests } from '@vscode/test-electron';

export async function main() {
    const devPath = path.resolve(__dirname, '..', '..');
    const testsPath = path.resolve(__dirname, 'suite', 'index');

    // Convert to POSIX form so Windows back-slashes don't get stripped
    const extensionDevelopmentPath = devPath.replace(/\\/g, '/');
    const extensionTestsPath = testsPath.replace(/\\/g, '/');

    console.log('devPath', extensionDevelopmentPath);
    console.log('testsPath', extensionTestsPath);

    await runTests({ extensionDevelopmentPath, extensionTestsPath });
}

main().catch(console.error);
// VS Code Extension for AiBsCleaner
const vscode = require('vscode');
const { exec } = require('child_process');
const path = require('path');

function activate(context) {
    // Register command
    let disposable = vscode.commands.registerCommand('aibscleaner.analyze', function () {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showErrorMessage('No active editor');
            return;
        }

        const document = editor.document;
        if (document.languageId !== 'go') {
            vscode.window.showErrorMessage('AiBsCleaner only works with Go files');
            return;
        }

        const filePath = document.fileName;
        const workspaceFolder = vscode.workspace.getWorkspaceFolder(document.uri);
        
        // Run AiBsCleaner
        exec(`aibscleaner -path "${filePath}" -format json`, 
            { cwd: workspaceFolder.uri.fsPath },
            (error, stdout, stderr) => {
                if (error) {
                    vscode.window.showErrorMessage(`AiBsCleaner error: ${error.message}`);
                    return;
                }

                try {
                    const results = JSON.parse(stdout);
                    displayResults(results, editor);
                } catch (e) {
                    vscode.window.showErrorMessage(`Failed to parse results: ${e.message}`);
                }
            }
        );
    });

    context.subscriptions.push(disposable);

    // Register CodeLens provider
    const codeLensProvider = new AiBsCleanerCodeLensProvider();
    context.subscriptions.push(
        vscode.languages.registerCodeLensProvider('go', codeLensProvider)
    );

    // Register diagnostics
    const diagnosticCollection = vscode.languages.createDiagnosticCollection('aibscleaner');
    context.subscriptions.push(diagnosticCollection);

    // Auto-run on save
    vscode.workspace.onDidSaveTextDocument((document) => {
        if (document.languageId === 'go') {
            runAnalysis(document, diagnosticCollection);
        }
    });
}

function displayResults(results, editor) {
    const diagnostics = [];
    
    results.issues.forEach(issue => {
        const range = new vscode.Range(
            issue.line - 1, issue.column - 1,
            issue.line - 1, issue.column + 10
        );

        const severity = issue.severity === 'HIGH' 
            ? vscode.DiagnosticSeverity.Error
            : issue.severity === 'MEDIUM'
            ? vscode.DiagnosticSeverity.Warning
            : vscode.DiagnosticSeverity.Information;

        const diagnostic = new vscode.Diagnostic(
            range,
            `${issue.message}\n💡 ${issue.suggestion}`,
            severity
        );
        
        diagnostic.code = issue.type;
        diagnostic.source = 'AiBsCleaner';
        
        diagnostics.push(diagnostic);
    });

    const diagnosticCollection = vscode.languages.createDiagnosticCollection('aibscleaner');
    diagnosticCollection.set(editor.document.uri, diagnostics);
}

class AiBsCleanerCodeLensProvider {
    provideCodeLenses(document, token) {
        const codeLenses = [];
        
        // Add "Run AiBsCleaner" lens at the top of file
        const topOfDocument = new vscode.Range(0, 0, 0, 0);
        const command = {
            title: "🧹 Run AiBsCleaner",
            command: "aibscleaner.analyze"
        };
        
        codeLenses.push(new vscode.CodeLens(topOfDocument, command));
        
        return codeLenses;
    }
}

function deactivate() {}

module.exports = {
    activate,
    deactivate
}
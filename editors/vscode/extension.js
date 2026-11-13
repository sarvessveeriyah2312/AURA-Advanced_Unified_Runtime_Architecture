// Tread for VS Code (ADR-0015): starts `tread lsp` from the Tread SDK and
// connects it to the editor.
"use strict";

const vscode = require("vscode");
const { LanguageClient } = require("vscode-languageclient/node");

let client;

function activate(context) {
  const command = vscode.workspace.getConfiguration("tread").get("path") || "tread";
  client = new LanguageClient(
    "tread",
    "Tread",
    { command, args: ["lsp"] },
    { documentSelector: [{ scheme: "file", language: "tread" }] }
  );
  client.start().catch((err) => {
    vscode.window.showErrorMessage(
      `Tread: could not start "${command} lsp" (${err.message}). Install the Tread SDK 0.7.0 or later, or set "tread.path".`
    );
  });
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("tread.path")) {
        vscode.window.showInformationMessage("Tread: reload the window to use the new tread.path.");
      }
    })
  );
}

function deactivate() {
  return client ? client.stop() : undefined;
}

module.exports = { activate, deactivate };

# Tread for Visual Studio Code

Language support for [Tread](https://github.com/sarvessveeriyah2312/Tread)
`.tr` files:

-   syntax highlighting;
-   errors as you type, with Tread's error codes (also in imported files);
-   hover for types and signatures, go to definition, completion;
-   **Format Document** with `tread fmt`;
-   an outline of structs, enums, functions and tests.

Everything except highlighting comes from the language server in the
**Tread SDK** (`tread lsp`), so install the SDK 0.7.0 or later first:
<https://github.com/sarvessveeriyah2312/Tread#installation>. If `tread`
is not on your `PATH`, set **Tread: Path** (`tread.path`) to the
executable.

## Install

The extension is not on the Marketplace. Download
`tread-vscode-<version>.vsix` from a
[Tread release](https://github.com/sarvessveeriyah2312/Tread/releases),
then run **Extensions: Install from VSIX...** in VS Code, or:

```sh
code --install-extension tread-vscode-0.7.0.vsix
```

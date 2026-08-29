# LSP Refactor Specification

## Purpose

Enables GAIA's LSP client to perform active code refactoring via standard LSP JSON-RPC methods (`textDocument/rename`, `textDocument/references`, `textDocument/codeAction`) and safely apply multi-file `WorkspaceEdit` modifications without text corruption or partial file writes.

## Requirements

### Requirement: Symbol Renaming (textDocument/rename)

The LSP client MUST support the `textDocument/rename` JSON-RPC method. It SHALL send a `RenameParams` object containing document URI, symbol position (line and character, 0-indexed), and the new symbol name. It MUST parse the returned `WorkspaceEdit` containing all file modifications across the workspace.

#### Scenario: Rename symbol across multiple files

- GIVEN an active LSP client connected to a language server (e.g., gopls, pylsp, tsserver)
- WHEN `Rename` is invoked with document URI "file:///workspace/main.go", line 10, character 5, and newName "NewFunctionName"
- THEN the client sends a `textDocument/rename` JSON-RPC request and returns the resulting `WorkspaceEdit` mapping affected file URIs to text edits.

#### Scenario: Rename non-renamable position or invalid symbol

- GIVEN an active LSP client
- WHEN `Rename` is invoked at a position with no identifiable symbol or a language keyword
- THEN the client returns an error or an empty edit set indicating the symbol cannot be renamed.

### Requirement: Find References (textDocument/references)

The LSP client MUST support the `textDocument/references` JSON-RPC method. It SHALL accept document URI, position, and reference context flags (including `includeDeclaration: bool`), returning a slice of `Location` objects with URI and range for each occurrence.

#### Scenario: Find symbol references including declaration

- GIVEN an active LSP client
- WHEN `References` is invoked with URI "file:///workspace/service.go", position line 20, character 8, and `includeDeclaration: true`
- THEN the client returns a list of all locations referencing the symbol, including its declaration.

#### Scenario: Find references for local symbol with no external usages

- GIVEN an active LSP client
- WHEN `References` is invoked for an unreferenced local symbol with `includeDeclaration: false`
- THEN the client returns an empty location list without failing.

### Requirement: Code Actions (textDocument/codeAction)

The LSP client MUST support the `textDocument/codeAction` JSON-RPC method. It SHALL accept document URI, range, and code action context (diagnostics, kinds) and return available `CodeAction` or `Command` items.

#### Scenario: Retrieve quick-fix code actions

- GIVEN an active LSP client and a file with known diagnostics (e.g., unused import or missing error check)
- WHEN `CodeAction` is invoked with the diagnostic's range and context
- THEN the client returns the list of available code actions, each with a title, kind, and optional edit or command.

#### Scenario: Request code actions on clean code section

- GIVEN an active LSP client and a file range with no diagnostics
- WHEN `CodeAction` is invoked with an empty diagnostic slice
- THEN the client returns any available refactoring actions or an empty list without error.

### Requirement: Safe Multi-File WorkspaceEdit Application

The system MUST provide a WorkspaceEdit applier that safely applies text edits across single or multiple files on disk. The applier MUST sort edits in reverse document order (descending line and column offset) before applying them to prevent offset shifts and content corruption. It MUST perform atomic file writes and validate that edits do not contain conflicting or overlapping ranges.

#### Scenario: Apply multi-file edits in reverse offset order

- GIVEN a `WorkspaceEdit` containing multiple `TextEdit` entries across multiple files
- WHEN the workspace edit applier executes the edit plan
- THEN edits within each file are applied from bottom-to-top by range, file contents are updated atomically, and disk files match the refactored state.

#### Scenario: Fail fast on invalid file or write error

- GIVEN a `WorkspaceEdit` targeting multiple files where one target file cannot be read or written
- WHEN the workspace edit applier encounters a file error
- THEN the applier SHALL fail fast, report the failure details, and prevent silent partial corruption of workspace state.

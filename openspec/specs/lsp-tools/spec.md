# LSP Tools Specification

## Purpose

Exposes semantic LSP refactoring capabilities (`lsp_rename_symbol`, `lsp_find_references`, `lsp_code_actions`) as agent tools within the GAIA Tool Engine and LSPModule, enabling subagents (Implementer, Debugger) to perform accurate, syntax-aware refactoring.

## Requirements

### Requirement: Rename Symbol Tool (lsp_rename_symbol)

The LSPModule MUST provide the `lsp_rename_symbol` tool (or server-scoped equivalent `lsp_{server}_rename_symbol`). The tool SHALL accept file path, line (1-indexed or 0-indexed per tool schema convention), column/character, and new name. It MUST execute the rename via the underlying LSP client, apply the resulting `WorkspaceEdit`, and return a structured summary of modified files and changed line counts.

#### Scenario: Subagent executes symbol rename

- GIVEN the Implementer subagent invokes `lsp_rename_symbol` with path "src/app.ts", line 15, character 10, and new_name "processOrder"
- WHEN the tool executes successfully
- THEN the tool applies the edits to the target files and returns `success: true` with a summary listing each modified file and the number of edits applied.

#### Scenario: Rename symbol failure reporting

- GIVEN a subagent invokes `lsp_rename_symbol` on an invalid identifier position or with an invalid new name
- WHEN the LSP server returns an error or rejects the rename
- THEN the tool returns `success: false` and a descriptive error message without mutating any files on disk.

### Requirement: Find References Tool (lsp_find_references)

The LSPModule MUST provide the `lsp_find_references` tool. The tool SHALL accept file path, line, character, and optional `include_declaration` boolean. It MUST return a structured list of reference locations, including file path, start line/character, end line/character, and code line previews where available.

#### Scenario: Subagent finds all symbol references

- GIVEN the Debugger subagent invokes `lsp_find_references` with path "pkg/auth/token.go", line 42, character 6, and `include_declaration: true`
- WHEN the tool queries the LSP client
- THEN the tool returns `success: true` and an output listing all reference locations with file paths and line numbers.

#### Scenario: No references found

- GIVEN a subagent calls `lsp_find_references` on a position with no external usages
- WHEN the LSP client returns no references
- THEN the tool returns `success: true` with an empty references list and an informative message.

### Requirement: Code Actions Tool (lsp_code_actions)

The LSPModule MUST provide the `lsp_code_actions` tool. The tool SHALL accept file path, start line/character, end line/character, and an optional `apply` boolean or selected action title. When `apply` is false, it MUST list available actions; when `apply` is true or an action is chosen, it MUST execute the action's edits and return the result.

#### Scenario: Subagent inspects available code actions

- GIVEN a subagent invokes `lsp_code_actions` with path "main.py", start_line 5, start_char 1, end_line 5, end_char 20, and `apply: false`
- WHEN the tool queries the LSP server
- THEN the tool returns `success: true` with the titles, kinds, and descriptions of available code actions.

#### Scenario: Subagent applies a selected code action

- GIVEN a subagent invokes `lsp_code_actions` with an action title or `apply: true` for an available quick-fix
- WHEN the code action contains a `WorkspaceEdit`
- THEN the tool applies the `WorkspaceEdit` to disk and returns `success: true` with details of the applied changes.

### Requirement: Subagent Tool Registration and Access Control

The system MUST register the semantic refactoring tools in `LSPModule` and make them available to subagents configured for refactoring, implementation, and debugging (Implementer, Debugger).

#### Scenario: Tool availability in subagent tool registry

- GIVEN a configured LSP server (e.g., gopls) and an active subagent session
- WHEN the subagent inspects its available tools
- THEN `lsp_rename_symbol`, `lsp_find_references`, and `lsp_code_actions` are present with valid argument schemas.

#### Scenario: Tool execution with disconnected server

- GIVEN an LSP server that is stopped or failed to connect
- WHEN a subagent invokes any LSP refactoring tool
- THEN the tool returns `success: false` with an explicit error indicating the LSP server is not connected.

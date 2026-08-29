# Tasks: LSP Active Refactor & Subagent Tool Integration

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 350-450 |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Data Models, WorkspaceEdit Engine & Reverse Sorting (`internal/lsp/types.go`, `internal/lsp/edit.go`) → PR 2: JSON-RPC Client Refactoring Methods & Subagent Module Tools (`internal/lsp/client.go`, `internal/lsp/module.go`) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

## Implementation Tasks

- [x] Implement LSP 3.17 base data models (`Position`, `Range`, `Location`, `TextEdit`, `WorkspaceEdit`, `CodeActionContext`, `CodeAction`) in `internal/lsp/types.go` with JSON marshaling/unmarshaling unit tests. <!-- sdd-owner: implementation -->
- [x] Implement safe `WorkspaceEdit` application engine with atomic backup, rollback, and reverse-order sorting (descending by end line and character) in `internal/lsp/edit.go` with unit tests covering multi-file edits and error rollback. <!-- sdd-owner: implementation -->
- [x] Implement LSP client refactoring methods (`RenameSymbol`, `FindReferences`, `CodeActions`) in `internal/lsp/client.go` with mock server integration tests. <!-- sdd-owner: implementation -->
- [x] Implement subagent tool wrappers (`lsp_rename_symbol`, `lsp_find_references`, `lsp_code_actions`) and tool registration in `internal/lsp/module.go` with end-to-end module execution tests. <!-- sdd-owner: implementation -->
- [x] Perform integration verification testing against available language servers (gopls / mock) to ensure robust error handling and successful multi-file refactoring. <!-- sdd-owner: implementation -->
- [x] Conduct final review of code coverage, schema conformance, and documentation. <!-- sdd-owner: parent -->

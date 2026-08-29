# Proposal: LSP Active Refactor

## Intent
Transform GAIA's passive LSP client into an active refactoring engine to enable subagents (Implementer, Debugger) to perform semantic code refactoring directly via Language Server capabilities (rename, find references, code actions, and workspace edits).

## Scope
**In Scope:**
- Implement `textDocument/rename`, `textDocument/references`, and `textDocument/codeAction` in the LSP JSON-RPC client.
- Implement WorkspaceEdit application across files (`workspace/applyEdit`).
- Expose these capabilities as semantic refactoring tools (`lsp_rename_symbol`, `lsp_find_references`, `lsp_code_actions`).
- Support Go (gopls), Python (pylsp), and TypeScript (tsserver).

**Out of Scope:**
- Initializing or managing the lifecycle of Language Servers (assumes GAIA already connects/initializes them).
- Implementing new LSP servers.
- Complex refactoring operations not natively supported by the language servers (e.g., custom AST manipulations outside LSP).

## Capabilities
**New:**
- **Workspace Edit Application:** Ability to safely apply cross-file text changes returned by LSP.
- **Semantic Tools:** Tools specifically designed for LLM subagents to query references, rename symbols, and execute code actions.

**Modified:**
- **LSP Client:** Extended to support the new JSON-RPC methods (`textDocument/rename`, etc.).

## Approach
1. **Extend JSON-RPC Client:** Add robust request/response handling for the targeted LSP methods in the existing Go 1.26 LSP client.
2. **Implement WorkspaceEdit Applier:** Create a module to handle file modifications concurrently while respecting overlapping edits and file states.
3. **Tool Exposure:** Map the new client methods to subagent tool schemas (`lsp_rename_symbol`, `lsp_find_references`, `lsp_code_actions`).
4. **Integration Testing:** Test the new tools against `gopls`, `pylsp`, and `tsserver` using dummy projects to ensure correct edit applications.

## Affected Areas
- `lsp/client` (JSON-RPC method definitions and routing).
- `agent/tools` (schema definitions for new semantic tools).
- File handling/IO modules (for applying WorkspaceEdits).

## Risks
- **Data Loss/Corruption:** Incorrect application of WorkspaceEdits (e.g., overlapping text edits or off-by-one offsets) could break user code.
- **Language Server Quirks:** `gopls`, `pylsp`, and `tsserver` handle capabilities and edit formats slightly differently.

## Rollback Plan
- Revert the addition of the new tools in the agent tool registry.
- Roll back the LSP client extensions to the previous passive state via Git.
- (If data is corrupted) Rely on source control (Git) of the target repository.

## Success Criteria
- Subagents can successfully find references, rename variables across multiple files, and apply code actions.
- Workspace edits are applied safely without mangling files.
- Integration tests pass for Go, Python, and TypeScript servers.

## Proposal question round
**Assumptions needing review:**
1. **File State:** Should WorkspaceEdits be applied directly to the file system, or to an in-memory virtual file system first (for dry-runs)?
2. **Server Capabilities:** Do we need capability detection/fallback if a specific server (e.g. pylsp) does not support a specific code action?
3. **Review/Undo:** Do we need a mechanism to rollback/undo an applied WorkspaceEdit within GAIA, or do we rely purely on Git?

**Proposed Questions:**
1. What happens if a subagent triggers a rename that affects a file currently unsaved or modified by the user?
2. Should we support file creation/deletion/renaming (ResourceOperations) as part of WorkspaceEdits, or just TextEdits?
3. How should failures during a multi-file WorkspaceEdit application be handled (e.g., fail fast vs. best effort)?
# Proposal: codegraph-ast-index

## Intent
Build a high-performance, SQLite-backed CodeGraph indexer native to Go. By parsing the Abstract Syntax Tree (AST), this engine will index packages, structs, interfaces, functions, methods, and call references, empowering AI subagents to execute complex semantic and architectural queries in sub-millisecond times.

## Scope
**In Scope:**
- Parsing Go source code using standard library packages (`go/ast`, `go/parser`, `go/types`).
- Extracting and defining domain entities: packages, structs, interfaces, functions, methods, and call relationships.
- Persisting the graph (nodes and edges) into a local SQLite database.
- Building a querying engine with Hexagonal architecture ports (interfaces) and SQLite adapters.
- Providing specific semantic queries (e.g., "Find all callers of X", "List struct fields of Y").

**Out of Scope:**
- Multi-language support (Go only for this iteration).
- Full Language Server Protocol (LSP) implementation.
- Real-time file system watching (indexing will be triggered on-demand or via explicit hooks).

## Capabilities
**New:**
- **AST Indexing:** Ability to parse and store exact structural representations of Go codebases.
- **Semantic CodeGraph Queries:** Sub-millisecond SQL-based retrieval of architectural relationships and call trees.

**Modified:**
- **Agent Code Exploration:** Shifts AI agent discovery from text-based `grep`/`find` to highly accurate semantic AST queries.

## Approach
The solution will follow Hexagonal Architecture principles:
1.  **Domain:** Define structural entities (`Node`, `Edge`, `Symbol`) representing the Go AST.
2.  **Ports:** Define `Indexer` (for parsing and saving code) and `QueryEngine` (for retrieving relationships).
3.  **Adapters:** Implement an AST Parser Adapter (using `go/ast`) and a SQLite Repository Adapter (using `database/sql`).
4.  **Database:** Design normalized SQLite tables (e.g., `nodes`, `edges`) optimized with indices for hierarchical and relational lookups.

## Affected Areas
- Local workspace storage (addition of `.codegraph.db` or similar SQLite file).
- Subagent context generation and codebase exploration tooling.

## Risks
- **Performance Overhead:** Initial indexing on massive monolithic codebases might consume significant CPU and memory.
- **AST Resolution Complexity:** Handling edge cases like complex Go generics, type aliases, and cgo boundaries accurately.

## Rollback Plan
- Revert AI subagent configuration to use legacy file-based search tools (`grep`, `find`).
- Delete the generated SQLite index files and remove the CodeGraph adapter binaries/code.

## Success Criteria
- **Speed:** Indexing a standard 10k-50k LOC Go repository completes in under 2 seconds.
- **Latency:** Semantic queries (e.g., deep caller paths) return in under 1 millisecond.
- **Accuracy:** 100% precision in resolving struct implementations, method receivers, and direct function calls within the same module.
- **Integration:** Subagents can successfully interface with the engine using standard SQLite adapters without crashing.

---

## Proposal question round
*To refine this proposal, please review the following product/design assumptions:*
1. **Triggering:** Since real-time watching is out of scope, will the index be rebuilt from scratch on every agent execution, or should we design an incremental update mechanism using file hashes?
2. **Storage Location:** Should the SQLite index live in a global cache directory, or locally inside the target project repository (e.g., `.codegraph/index.db`)?
3. **External Dependencies:** Do we need to index external `go.mod` dependencies, or should we strictly bound the AST indexing to the local project module to save time and space?
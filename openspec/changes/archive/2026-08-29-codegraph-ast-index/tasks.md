# Tasks: CodeGraph AST Indexer

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 600-900 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Domain, Ports & SQLite Store -> PR 2: AST Parser Adapter -> PR 3: Query Engine & Subagent Tools |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Task Breakdown

### Slice 1: Domain, Ports, and SQLite Store Adapter

- [x] 1.1 Create domain package structure under `internal/codegraph/domain/` with `SymbolRef`, `SymbolNode`, `Edge`, and related filter/hierarchy types. <!-- sdd-owner: implementation -->
- [x] 1.2 Write unit tests for domain entities and validation rules in `internal/codegraph/domain/domain_test.go`. <!-- sdd-owner: implementation -->
- [x] 1.3 Create ports package under `internal/codegraph/ports/` defining `IndexerPort` and `QueryEnginePort`. <!-- sdd-owner: implementation -->
- [x] 1.4 Implement SQLite store adapter under `internal/codegraph/adapters/sqlite/` using `modernc.org/sqlite` with schema initialization for `files`, `nodes`, and `edges` tables plus performance indexes. <!-- sdd-owner: implementation -->
- [x] 1.5 Implement incremental hashing, file hash lookup, and stale node/edge deletion methods in the SQLite adapter. <!-- sdd-owner: implementation -->
- [x] 1.6 Write integration tests for SQLite store persistence, transactions, and incremental updates in `internal/codegraph/adapters/sqlite/store_test.go`. <!-- sdd-owner: implementation -->

### Slice 2: AST Parser Adapter

- [x] 2.1 Implement AST parser adapter under `internal/codegraph/adapters/ast/` utilizing `go/parser`, `go/ast`, and `go/token` for directory traversal and file parsing. <!-- sdd-owner: implementation -->
- [x] 2.2 Implement symbol extraction logic to capture packages, structs, interfaces, functions, methods, fields, signatures, docstrings, and export status. <!-- sdd-owner: implementation -->
- [x] 2.3 Implement edge resolution logic to extract structural (`CONTAINS`, `RECEIVER_OF`) and behavioral (`CALLS`, `IMPLEMENTS`, `IMPORTS`) relationships. <!-- sdd-owner: implementation -->
- [x] 2.4 Implement `IndexerPort` interface combining AST parsing and SQLite batch insertion transaction commit. <!-- sdd-owner: implementation -->
- [x] 2.5 Write unit and integration tests for AST parsing and relationship extraction across sample Go source files in `internal/codegraph/adapters/ast/parser_test.go`. <!-- sdd-owner: implementation -->

### Slice 3: Query Engine & Subagent Tools

- [x] 3.1 Implement query engine adapter under `internal/codegraph/adapters/sqlite/query.go` fulfilling `QueryEnginePort`. <!-- sdd-owner: implementation -->
- [x] 3.2 Implement `FindCallers` and `FindCallees` queries with optimized sub-millisecond SQL joins. <!-- sdd-owner: implementation -->
- [x] 3.3 Implement `FindImplementations` and `GetStructDetails` queries for interface and struct resolution. <!-- sdd-owner: implementation -->
- [x] 3.4 Implement `GetCallHierarchy` and `LookupSymbol` with multi-depth traversal support. <!-- sdd-owner: implementation -->
- [x] 3.5 Write comprehensive unit and benchmark tests for query performance (<1ms latency) in `internal/codegraph/adapters/sqlite/query_test.go`. <!-- sdd-owner: implementation -->
- [x] 3.6 Wire up query engine and indexer tools for subagent codebase exploration integration. <!-- sdd-owner: implementation -->

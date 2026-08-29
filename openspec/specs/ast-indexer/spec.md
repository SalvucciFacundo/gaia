# AST Indexer Specification

## Purpose

High-performance, AST-driven structural indexer for Go codebases. Parses Go source files recursively using standard Go AST tooling (`go/parser`, `go/ast`, `go/token`), extracts structural symbols (nodes) and architectural relationships (edges), and persists the resulting graph into a local SQLite database with sub-2-second indexing for standard modules.

## Requirements

### Requirement: Recursive Workspace Discovery and File Filtering

The indexer MUST recursively traverse the target workspace directory to discover Go source files (`*.go`). The indexer SHALL ignore vendor directories, test caches, hidden directories, and configured ignore patterns. The indexer SHOULD provide options to include or exclude `*_test.go` files.

#### Scenario: Recursive discovery in Go module

- GIVEN a Go module with multiple packages across nested subdirectories
- WHEN the indexer is triggered on the root directory
- THEN all non-ignored `.go` files across all packages are discovered and queued for parsing

#### Scenario: Ignore vendor and hidden directories

- GIVEN a workspace containing a `vendor/` directory and `.git/` folder
- WHEN the indexer traverses the workspace
- THEN files inside `vendor/` and `.git/` are skipped from AST parsing

---

### Requirement: AST Parsing and Symbol Node Extraction

The system MUST parse valid Go source files into Abstract Syntax Trees using standard Go packages (`go/parser`, `go/token`, `go/ast`). The indexer MUST extract structural code entities as typed nodes:
- Packages (`package_name`, import path)
- Types (`struct`, `interface`, `type alias`, basic type definitions)
- Functions (standalone functions with parameters, return types, and signatures)
- Methods (functions with explicit pointer or value receivers)
- Struct Fields and Interface Method Signatures

Every node MUST record unique identifier, kind, name, package identifier, file path, line/column start and end spans, export visibility (exported/unexported), and associated doc comments.

#### Scenario: Parsing struct and interface definitions

- GIVEN a Go source file declaring `type Reader interface { Read(p []byte) (n int, err error) }` and `type FileReader struct { path string }`
- WHEN the file is parsed by the indexer
- THEN a node of kind `interface` is created for `Reader` with method signature `Read`
- AND a node of kind `struct` is created for `FileReader` with field `path`
- AND accurate file positions (start/end lines) are saved for both types

#### Scenario: Parsing method with receiver

- GIVEN a Go source file declaring `func (r *FileReader) Read(p []byte) (int, error)`
- WHEN the file is parsed by the indexer
- THEN a node of kind `method` is created with name `Read`
- AND the node stores the receiver type `*FileReader` and full signature parameters

---

### Requirement: Relationship and Edge Extraction

The system MUST extract structural and behavioral relationships between nodes as directed edges with typed relationship semantics:
- `CONTAINS`: Package contains type/function; struct contains field; interface contains method signature
- `RECEIVER_OF`: Type is the receiver for a method
- `CALLS`: Function or method invokes another function or method within the indexed codebase
- `IMPLEMENTS`: Struct or type implements an interface by matching all interface method signatures
- `IMPORTS`: File or package imports another package

Edge records MUST include source node ID, target node ID, edge kind, and call location metadata (file, line number) when applicable.

#### Scenario: Direct function call reference

- GIVEN function `Service.Process()` calls `Repo.Save()` on line 42 of `service.go`
- WHEN the indexer analyzes the AST of `Service.Process()`
- THEN an edge of kind `CALLS` is recorded from `Service.Process` node ID to `Repo.Save` node ID
- AND the edge contains line number 42 and file path `service.go`

#### Scenario: Interface implementation resolution

- GIVEN an interface `Greeter` requiring method `Greet() string`
- AND a struct `SpanishGreeter` declaring method `func (g SpanishGreeter) Greet() string`
- WHEN the indexer finishes symbol extraction and evaluates interface satisfaction
- THEN an edge of kind `IMPLEMENTS` is created from `SpanishGreeter` to `Greeter`

---

### Requirement: SQLite Persistence and Schema Optimization

The system MUST persist nodes, edges, files, and indexing metadata into a local SQLite database using pure Go drivers (`modernc.org/sqlite` without CGO requirement). The schema MUST define normalized tables (`nodes`, `edges`, `files`, `metadata`) with indexes on node name, kind, package, source ID, target ID, and edge kind. All writes during an indexing run MUST be executed within batch transactions to achieve indexing throughput under 2 seconds for 10k-50k LOC.

#### Scenario: Initial index creation

- GIVEN a workspace with 25k LOC of Go code and no prior index database
- WHEN the indexer executes a full indexing pass
- THEN the SQLite database file is created with all tables and indices populated
- AND the total indexing execution time does not exceed 2 seconds

#### Scenario: Transaction rollback on fatal error

- GIVEN an active indexing transaction encountering an unrecoverable database I/O error
- WHEN the transaction fails
- THEN the transaction is rolled back cleanly without corrupting existing database state

---

### Requirement: Incremental Indexing and Cache Invalidation

The system SHOULD track file modification times or content hashes (e.g., SHA-256) in the `files` table. When re-indexing, the system MUST skip parsing unchanged files whose hashes match the index record. If a file has been modified or deleted, the system MUST purge stale nodes and edges originating from that file before re-indexing.

#### Scenario: Re-indexing with unmodified files

- GIVEN a project where 50 files were indexed previously and only 1 file was modified
- WHEN an incremental index run is executed
- THEN 49 files are skipped from AST re-parsing
- AND only the 1 modified file's nodes and edges are purged and updated in SQLite

#### Scenario: File deletion cleanup

- GIVEN a file `pkg/old/legacy.go` was deleted from the workspace
- WHEN the indexer runs
- THEN all nodes and edges associated with `pkg/old/legacy.go` are removed from the database

---

### Requirement: Malformed Syntax Resiliency

The indexer MUST NOT panic or crash when encountering Go source files with syntax errors or partially broken ASTs. The indexer SHALL record a non-fatal warning or error entry for the failing file and MUST continue indexing all other valid Go files in the workspace.

#### Scenario: Handling syntax error in single file

- GIVEN a Go file with invalid syntax (e.g., unmatched braces during active editing)
- WHEN the indexer parses the workspace
- THEN the failing file emits a parsing warning without aborting the indexer process
- AND all remaining valid Go files are successfully parsed and stored in the database

# CodeGraph Query Specification

## Purpose

High-performance semantic query engine for the Go CodeGraph index. Provides sub-millisecond graph and relational queries (callers, callees, implementations, call hierarchy, symbol details) via Hexagonal architecture ports and SQLite adapter queries to support AI subagents and developer tooling.

## Requirements

### Requirement: Query Port Architecture

The system MUST define a domain-level query port interface (`QueryEngine` or `CodeGraphQueryPort`) that abstracts underlying SQLite storage from query consumers. The port MUST expose explicit query capabilities:
- `FindCallers(ctx context.Context, target SymbolRef) ([]CallerResult, error)`
- `FindCallees(ctx context.Context, source SymbolRef) ([]CalleeResult, error)`
- `FindImplementations(ctx context.Context, interfaceRef SymbolRef) ([]SymbolNode, error)`
- `GetCallHierarchy(ctx context.Context, root SymbolRef, direction HierarchyDirection, maxDepth int) (*CallHierarchyTree, error)`
- `LookupSymbol(ctx context.Context, criteria SymbolFilter) ([]SymbolNode, error)`
- `GetStructDetails(ctx context.Context, structRef SymbolRef) (*StructDetails, error)`

#### Scenario: Port abstraction decoupling

- GIVEN a subagent requesting callers of a method via `CodeGraphQueryPort`
- WHEN the query engine executes the request
- THEN the query result is returned as domain entities without exposing raw SQL strings or SQLite connection handles to the caller

---

### Requirement: Find Callers and Callees

The system MUST query incoming call references (`FindCallers`) and outgoing call references (`FindCallees`) for any given function or method symbol. The query response MUST include the caller/callee symbol details (ID, name, package, receiver, kind) along with call site metadata (file path and line number of the invocation).

#### Scenario: Finding all callers of a function

- GIVEN function `auth.ValidateToken` is called by `api.LoginHandler` (line 55) and `api.RefreshHandler` (line 82)
- WHEN `FindCallers` is invoked for `auth.ValidateToken`
- THEN the response contains two records: `api.LoginHandler` with line 55, and `api.RefreshHandler` with line 82

#### Scenario: Finding all callees of a method

- GIVEN method `order.Service.Checkout` invokes `payment.Client.Charge` and `inventory.Client.Reserve`
- WHEN `FindCallees` is invoked for `order.Service.Checkout`
- THEN the response lists `payment.Client.Charge` and `inventory.Client.Reserve` with their respective call line numbers

#### Scenario: Querying symbol with no callers

- GIVEN an unreferenced helper function `util.UnusedHelper`
- WHEN `FindCallers` is invoked for `util.UnusedHelper`
- THEN the query returns an empty list and no error

---

### Requirement: Interface Implementation Queries

The system MUST support bi-directional implementation queries:
1. Interface to Implementations: Given an interface symbol, find all structs or types that implement all methods of the interface.
2. Type to Implemented Interfaces: Given a struct or type symbol, find all interfaces satisfied by its declared method set.

#### Scenario: Query implementations of an interface

- GIVEN interface `repository.UserStore` with methods `Get(id string)` and `Save(u *User)`
- AND structs `PostgresUserStore` and `MemoryUserStore` both implement both methods
- WHEN `FindImplementations` is invoked for `repository.UserStore`
- THEN the response contains nodes for both `PostgresUserStore` and `MemoryUserStore`

#### Scenario: Query interfaces implemented by a struct

- GIVEN struct `Buffer` implements interfaces `io.Reader`, `io.Writer`, and `io.Closer`
- WHEN querying interfaces implemented by `Buffer`
- THEN the response returns `io.Reader`, `io.Writer`, and `io.Closer` symbols

---

### Requirement: Call Hierarchy and Path Traversal

The system MUST compute multi-level call hierarchies (`GetCallHierarchy`) in either upstream (callers) or downstream (callees) direction up to a specified `maxDepth`. The traversal MUST detect recursive or circular call chains and terminate cycles gracefully without infinite recursion.

#### Scenario: Multi-level upstream call hierarchy

- GIVEN `Database.Query` is called by `Repository.FindUser`, which is called by `Service.GetUser`, which is called by `Handler.GetUserEndpoint`
- WHEN `GetCallHierarchy` is requested for `Database.Query` with direction `UPSTREAM` and `maxDepth = 3`
- THEN a tree structure is returned representing `Database.Query ← Repository.FindUser ← Service.GetUser ← Handler.GetUserEndpoint`

#### Scenario: Circular call cycle prevention

- GIVEN function `FuncA` calls `FuncB`, and `FuncB` calls `FuncA`
- WHEN `GetCallHierarchy` is requested for `FuncA` with `maxDepth = 5`
- THEN traversal marks the recursive visit on `FuncA` as circular and terminates without stack overflow or deadlock

---

### Requirement: Symbol Lookup and Structural Inspection

The system MUST allow searching for symbol nodes by exact qualified identifier (e.g., `github.com/org/repo/pkg/auth.TokenValidator`), symbol name prefix, symbol kind (`struct`, `interface`, `func`, `method`), or file path. The engine MUST return full symbol details including signature, receiver type, start/end line coordinates, docstrings, and visibility.

#### Scenario: Lookup struct fields and methods

- GIVEN struct `config.ServerConfig` containing fields `Port int`, `Host string` and methods `Listen()`, `Close()`
- WHEN `GetStructDetails` is requested for `config.ServerConfig`
- THEN the response includes all declared fields with types and all associated methods

#### Scenario: Fuzzy symbol search by name

- GIVEN symbols `auth.NewAuthenticator`, `auth.AuthenticateUser`, and `session.Authenticator`
- WHEN `LookupSymbol` is queried with name `Authenticator`
- THEN all matching symbol nodes are returned sorted by relevance

---

### Requirement: Query Performance and Latency SLO

The SQLite query adapter MUST execute single-hop queries (`FindCallers`, `FindCallees`, `FindImplementations`, `LookupSymbol`) in under 1 millisecond (p95) on indexed codebases up to 100k LOC. All relational lookups MUST utilize indexed columns (`source_id`, `target_id`, `kind`, `name`, `package`).

#### Scenario: Sub-millisecond caller lookup

- GIVEN an indexed codebase with 50,000 nodes and 120,000 edges in the SQLite database
- WHEN `FindCallers` is executed for a frequently called utility function
- THEN the query completes and returns results in less than 1.0 millisecond

---

### Requirement: Safe Execution and Missing Target Handling

The query engine MUST return typed errors (e.g., `ErrSymbolNotFound`, `ErrInvalidQueryParameter`) when querying non-existent symbols or passing invalid parameters. Queries on non-existent symbols MUST NOT panic or execute malformed SQL statements.

#### Scenario: Querying non-existent symbol ID

- GIVEN a query for symbol ID `"non-existent-symbol-id"`
- WHEN `FindCallers` is executed
- THEN the engine returns `ErrSymbolNotFound` with clear context

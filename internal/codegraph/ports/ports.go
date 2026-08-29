package ports

import (
	"context"

	"gaia/internal/codegraph/domain"
)

// IndexerPort defines the contract for AST parsing and graph persistence.
type IndexerPort interface {
	// IndexWorkspace recursively discovers, parses, and persists Go files in the given directory.
	IndexWorkspace(ctx context.Context, workspacePath string) error
}

// QueryEnginePort defines the contract for semantic graph queries.
type QueryEnginePort interface {
	// FindCallers returns all incoming calls for the specified function or method symbol.
	FindCallers(ctx context.Context, target domain.SymbolRef) ([]domain.CallerResult, error)

	// FindCallees returns all outgoing calls from the specified function or method symbol.
	FindCallees(ctx context.Context, source domain.SymbolRef) ([]domain.CalleeResult, error)

	// FindImplementations returns symbols that implement the specified interface or interfaces implemented by the struct.
	FindImplementations(ctx context.Context, interfaceRef domain.SymbolRef) ([]domain.SymbolNode, error)

	// GetCallHierarchy traverses upstream or downstream call chains up to maxDepth.
	GetCallHierarchy(ctx context.Context, root domain.SymbolRef, direction domain.HierarchyDirection, maxDepth int) (*domain.CallHierarchyTree, error)

	// LookupSymbol returns symbol nodes matching the provided criteria.
	LookupSymbol(ctx context.Context, criteria domain.SymbolFilter) ([]domain.SymbolNode, error)

	// GetStructDetails retrieves field and method metadata for a struct symbol.
	GetStructDetails(ctx context.Context, structRef domain.SymbolRef) (*domain.StructDetails, error)
}

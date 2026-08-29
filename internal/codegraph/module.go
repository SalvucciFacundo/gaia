package codegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	astadapter "gaia/internal/codegraph/adapters/ast"
	sqladapter "gaia/internal/codegraph/adapters/sqlite"
	"gaia/internal/codegraph/domain"
	"gaia/internal/codegraph/ports"
	coredomain "gaia/internal/core/domain"
	coreports "gaia/internal/core/ports"
)

var _ coreports.Module = (*Module)(nil)

// Module implements coreports.Module, exposing AST indexing and query capabilities to AI agents.
type Module struct {
	store   *sqladapter.Store
	indexer ports.IndexerPort
	query   ports.QueryEnginePort
}

// NewModule creates a CodeGraph module with its SQLite store.
func NewModule(dbPath string) (*Module, error) {
	store, err := sqladapter.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init codegraph store: %w", err)
	}

	return &Module{
		store:   store,
		indexer: astadapter.NewIndexer(store),
		query:   sqladapter.NewQueryEngine(store),
	}, nil
}

// NewModuleWithComponents initializes a CodeGraph module using custom ports.
func NewModuleWithComponents(store *sqladapter.Store, indexer ports.IndexerPort, query ports.QueryEnginePort) *Module {
	return &Module{
		store:   store,
		indexer: indexer,
		query:   query,
	}
}

// Name returns module identifier.
func (m *Module) Name() string {
	return "codegraph"
}

// Description returns module summary.
func (m *Module) Description() string {
	return "High-performance semantic code graph query and AST indexing engine for Go codebases"
}

// GetTools returns the tool definitions exposed by this module.
func (m *Module) GetTools() []coredomain.ToolCall {
	return []coredomain.ToolCall{
		{
			Name: "codegraph_index_workspace",
			Arguments: map[string]interface{}{
				"description": "Recursively index Go source files in the specified directory into the SQLite code graph",
				"path":        "Workspace directory path to index",
			},
		},
		{
			Name: "codegraph_find_callers",
			Arguments: map[string]interface{}{
				"description": "Find all incoming call references for a given function or method symbol",
				"symbol":      "Qualified symbol ID (e.g. 'pkg.Func' or 'pkg.Type.Method')",
			},
		},
		{
			Name: "codegraph_find_callees",
			Arguments: map[string]interface{}{
				"description": "Find all outgoing call references from a given function or method symbol",
				"symbol":      "Qualified symbol ID (e.g. 'pkg.Func' or 'pkg.Type.Method')",
			},
		},
		{
			Name: "codegraph_find_implementations",
			Arguments: map[string]interface{}{
				"description": "Find implementations of an interface, or interfaces implemented by a struct",
				"symbol":      "Qualified interface or struct symbol ID",
			},
		},
		{
			Name: "codegraph_call_hierarchy",
			Arguments: map[string]interface{}{
				"description": "Get multi-level call hierarchy tree (upstream or downstream)",
				"symbol":      "Qualified root symbol ID",
				"direction":   "UPSTREAM or DOWNSTREAM",
				"max_depth":   "Maximum call chain traversal depth (default 3)",
			},
		},
		{
			Name: "codegraph_lookup_symbol",
			Arguments: map[string]interface{}{
				"description": "Search for code symbols by name, prefix, kind, package, or file",
				"name":        "Exact symbol name (optional)",
				"prefix":      "Symbol name prefix (optional)",
				"kind":        "Symbol kind: struct, interface, func, method, field (optional)",
				"package":     "Package filter (optional)",
				"file":        "File path filter (optional)",
			},
		},
		{
			Name: "codegraph_struct_details",
			Arguments: map[string]interface{}{
				"description": "Inspect struct declaration, fields, methods, and implemented interfaces",
				"symbol":      "Qualified struct symbol ID",
			},
		},
	}
}

// Execute handles tool dispatch for CodeGraph operations.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*coredomain.ToolResult, error) {
	switch toolName {
	case "codegraph_index_workspace":
		path, _ := args["path"].(string)
		if path == "" {
			path = "."
		}
		if err := m.indexer.IndexWorkspace(ctx, path); err != nil {
			return &coredomain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("indexing failed: %v", err),
			}, nil
		}
		return &coredomain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Successfully indexed workspace at %s", path),
		}, nil

	case "codegraph_find_callers":
		sym, _ := args["symbol"].(string)
		if strings.TrimSpace(sym) == "" {
			return &coredomain.ToolResult{
				Success: false,
				Error:   "missing required 'symbol' argument",
			}, nil
		}
		callers, err := m.query.FindCallers(ctx, domain.SymbolRef(sym))
		if err != nil {
			return &coredomain.ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		out, _ := json.MarshalIndent(callers, "", "  ")
		return &coredomain.ToolResult{
			Success: true,
			Output:  string(out),
		}, nil

	case "codegraph_find_callees":
		sym, _ := args["symbol"].(string)
		if strings.TrimSpace(sym) == "" {
			return &coredomain.ToolResult{
				Success: false,
				Error:   "missing required 'symbol' argument",
			}, nil
		}
		callees, err := m.query.FindCallees(ctx, domain.SymbolRef(sym))
		if err != nil {
			return &coredomain.ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		out, _ := json.MarshalIndent(callees, "", "  ")
		return &coredomain.ToolResult{
			Success: true,
			Output:  string(out),
		}, nil

	case "codegraph_find_implementations":
		sym, _ := args["symbol"].(string)
		if strings.TrimSpace(sym) == "" {
			return &coredomain.ToolResult{
				Success: false,
				Error:   "missing required 'symbol' argument",
			}, nil
		}
		impls, err := m.query.FindImplementations(ctx, domain.SymbolRef(sym))
		if err != nil {
			return &coredomain.ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		out, _ := json.MarshalIndent(impls, "", "  ")
		return &coredomain.ToolResult{
			Success: true,
			Output:  string(out),
		}, nil

	case "codegraph_call_hierarchy":
		sym, _ := args["symbol"].(string)
		if strings.TrimSpace(sym) == "" {
			return &coredomain.ToolResult{
				Success: false,
				Error:   "missing required 'symbol' argument",
			}, nil
		}
		dirStr, _ := args["direction"].(string)
		dir := domain.HierarchyDirection(strings.ToUpper(dirStr))
		if dir != domain.DirectionDownstream {
			dir = domain.DirectionUpstream
		}
		maxDepth := 3
		if md, ok := args["max_depth"].(float64); ok && md > 0 {
			maxDepth = int(md)
		} else if md, ok := args["max_depth"].(int); ok && md > 0 {
			maxDepth = md
		}
		tree, err := m.query.GetCallHierarchy(ctx, domain.SymbolRef(sym), dir, maxDepth)
		if err != nil {
			return &coredomain.ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		out, _ := json.MarshalIndent(tree, "", "  ")
		return &coredomain.ToolResult{
			Success: true,
			Output:  string(out),
		}, nil

	case "codegraph_lookup_symbol":
		filter := domain.SymbolFilter{}
		if n, ok := args["name"].(string); ok {
			filter.Name = n
		}
		if p, ok := args["prefix"].(string); ok {
			filter.Prefix = p
		}
		if k, ok := args["kind"].(string); ok {
			filter.Kind = k
		}
		if pkg, ok := args["package"].(string); ok {
			filter.Package = pkg
		}
		if f, ok := args["file"].(string); ok {
			filter.File = f
		}
		if exp, ok := args["exported_only"].(bool); ok {
			filter.ExportedOnly = exp
		}
		symbols, err := m.query.LookupSymbol(ctx, filter)
		if err != nil {
			return &coredomain.ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		out, _ := json.MarshalIndent(symbols, "", "  ")
		return &coredomain.ToolResult{
			Success: true,
			Output:  string(out),
		}, nil

	case "codegraph_struct_details":
		sym, _ := args["symbol"].(string)
		if strings.TrimSpace(sym) == "" {
			return &coredomain.ToolResult{
				Success: false,
				Error:   "missing required 'symbol' argument",
			}, nil
		}
		details, err := m.query.GetStructDetails(ctx, domain.SymbolRef(sym))
		if err != nil {
			return &coredomain.ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		out, _ := json.MarshalIndent(details, "", "  ")
		return &coredomain.ToolResult{
			Success: true,
			Output:  string(out),
		}, nil

	default:
		return &coredomain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown codegraph tool: %s", toolName),
		}, nil
	}
}

// Close closes database resources.
func (m *Module) Close() error {
	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

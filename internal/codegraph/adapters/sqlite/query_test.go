package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gaia/internal/codegraph/adapters/sqlite"
	"gaia/internal/codegraph/domain"
)

func setupTestGraph(t *testing.T) (*sqlite.Store, *sqlite.QueryEngine) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "query_test.db")

	store, err := sqlite.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	files := map[string]string{
		"pkg/auth.go":    "h1",
		"pkg/api.go":     "h2",
		"pkg/storage.go": "h3",
	}

	nodes := []domain.SymbolNode{
		{
			ID:         "pkg.AuthService",
			Kind:       domain.KindStruct,
			Name:       "AuthService",
			Package:    "pkg",
			File:       "pkg/auth.go",
			LineStart:  10,
			LineEnd:    20,
			IsExported: true,
		},
		{
			ID:         "pkg.AuthService.SecretKey",
			Kind:       domain.KindField,
			Name:       "SecretKey",
			Package:    "pkg",
			File:       "pkg/auth.go",
			LineStart:  12,
			LineEnd:    12,
			Signature:  "string",
			IsExported: true,
		},
		{
			ID:         "pkg.AuthService.ValidateToken",
			Kind:       domain.KindMethod,
			Name:       "ValidateToken",
			Package:    "pkg",
			File:       "pkg/auth.go",
			LineStart:  22,
			LineEnd:    30,
			Signature:  "(token string) bool",
			IsExported: true,
		},
		{
			ID:         "pkg.TokenValidator",
			Kind:       domain.KindInterface,
			Name:       "TokenValidator",
			Package:    "pkg",
			File:       "pkg/auth.go",
			LineStart:  5,
			LineEnd:    8,
			IsExported: true,
		},
		{
			ID:         "pkg.LoginHandler",
			Kind:       domain.KindFunc,
			Name:       "LoginHandler",
			Package:    "pkg",
			File:       "pkg/api.go",
			LineStart:  50,
			LineEnd:    60,
			IsExported: true,
		},
		{
			ID:         "pkg.RefreshHandler",
			Kind:       domain.KindFunc,
			Name:       "RefreshHandler",
			Package:    "pkg",
			File:       "pkg/api.go",
			LineStart:  70,
			LineEnd:    90,
			IsExported: true,
		},
		{
			ID:         "pkg.CyclicA",
			Kind:       domain.KindFunc,
			Name:       "CyclicA",
			Package:    "pkg",
			File:       "pkg/api.go",
			LineStart:  100,
			LineEnd:    110,
		},
		{
			ID:         "pkg.CyclicB",
			Kind:       domain.KindFunc,
			Name:       "CyclicB",
			Package:    "pkg",
			File:       "pkg/api.go",
			LineStart:  120,
			LineEnd:    130,
		},
	}

	edges := []domain.Edge{
		{
			ID:       "contains-auth-secret",
			SourceID: "pkg.AuthService",
			TargetID: "pkg.AuthService.SecretKey",
			Kind:     domain.EdgeContains,
			File:     "pkg/auth.go",
			Line:     12,
		},
		{
			ID:       "contains-auth-validate",
			SourceID: "pkg.AuthService",
			TargetID: "pkg.AuthService.ValidateToken",
			Kind:     domain.EdgeContains,
			File:     "pkg/auth.go",
			Line:     22,
		},
		{
			ID:       "impl-auth-validator",
			SourceID: "pkg.AuthService",
			TargetID: "pkg.TokenValidator",
			Kind:     domain.EdgeImplements,
			File:     "pkg/auth.go",
			Line:     10,
		},
		{
			ID:       "call-login-validate",
			SourceID: "pkg.LoginHandler",
			TargetID: "pkg.AuthService.ValidateToken",
			Kind:     domain.EdgeCalls,
			File:     "pkg/api.go",
			Line:     55,
		},
		{
			ID:       "call-refresh-validate",
			SourceID: "pkg.RefreshHandler",
			TargetID: "pkg.AuthService.ValidateToken",
			Kind:     domain.EdgeCalls,
			File:     "pkg/api.go",
			Line:     82,
		},
		{
			ID:       "call-cyclic-a-b",
			SourceID: "pkg.CyclicA",
			TargetID: "pkg.CyclicB",
			Kind:     domain.EdgeCalls,
			File:     "pkg/api.go",
			Line:     105,
		},
		{
			ID:       "call-cyclic-b-a",
			SourceID: "pkg.CyclicB",
			TargetID: "pkg.CyclicA",
			Kind:     domain.EdgeCalls,
			File:     "pkg/api.go",
			Line:     125,
		},
	}

	if err := store.SaveBatch(ctx, files, nodes, edges); err != nil {
		t.Fatalf("failed to save batch: %v", err)
	}

	queryEngine := sqlite.NewQueryEngine(store)
	return store, queryEngine
}

func TestQueryEngine_FindCallersAndCallees(t *testing.T) {
	store, engine := setupTestGraph(t)
	defer store.Close()
	ctx := context.Background()

	// 1. FindCallers for ValidateToken
	callers, err := engine.FindCallers(ctx, "pkg.AuthService.ValidateToken")
	if err != nil {
		t.Fatalf("FindCallers failed: %v", err)
	}
	if len(callers) != 2 {
		t.Fatalf("expected 2 callers, got %d", len(callers))
	}

	callerMap := make(map[string]int)
	for _, c := range callers {
		callerMap[string(c.Caller.ID)] = c.CallLine
	}
	if line, ok := callerMap["pkg.LoginHandler"]; !ok || line != 55 {
		t.Errorf("expected LoginHandler at line 55, got %v", callerMap["pkg.LoginHandler"])
	}
	if line, ok := callerMap["pkg.RefreshHandler"]; !ok || line != 82 {
		t.Errorf("expected RefreshHandler at line 82, got %v", callerMap["pkg.RefreshHandler"])
	}

	// 2. FindCallees for LoginHandler
	callees, err := engine.FindCallees(ctx, "pkg.LoginHandler")
	if err != nil {
		t.Fatalf("FindCallees failed: %v", err)
	}
	if len(callees) != 1 {
		t.Fatalf("expected 1 callee, got %d", len(callees))
	}
	if callees[0].Callee.ID != "pkg.AuthService.ValidateToken" || callees[0].CallLine != 55 {
		t.Errorf("unexpected callee: %+v", callees[0])
	}

	// 3. Query non-existent symbol returns ErrSymbolNotFound
	_, err = engine.FindCallers(ctx, "non.Existent")
	if err != domain.ErrSymbolNotFound {
		t.Errorf("expected ErrSymbolNotFound, got %v", err)
	}
}

func TestQueryEngine_FindImplementations(t *testing.T) {
	store, engine := setupTestGraph(t)
	defer store.Close()
	ctx := context.Background()

	// Query interface implementations
	impls, err := engine.FindImplementations(ctx, "pkg.TokenValidator")
	if err != nil {
		t.Fatalf("FindImplementations failed: %v", err)
	}
	if len(impls) != 1 || impls[0].ID != "pkg.AuthService" {
		t.Fatalf("expected AuthService implementation, got: %+v", impls)
	}

	// Query struct implemented interfaces
	ifaces, err := engine.FindImplementations(ctx, "pkg.AuthService")
	if err != nil {
		t.Fatalf("FindImplementations for struct failed: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].ID != "pkg.TokenValidator" {
		t.Fatalf("expected TokenValidator interface, got: %+v", ifaces)
	}
}

func TestQueryEngine_GetStructDetails(t *testing.T) {
	store, engine := setupTestGraph(t)
	defer store.Close()
	ctx := context.Background()

	details, err := engine.GetStructDetails(ctx, "pkg.AuthService")
	if err != nil {
		t.Fatalf("GetStructDetails failed: %v", err)
	}
	if details.Node.Name != "AuthService" {
		t.Errorf("expected AuthService struct node, got %s", details.Node.Name)
	}
	if len(details.Fields) != 1 || details.Fields[0].Name != "SecretKey" {
		t.Errorf("expected SecretKey field, got %+v", details.Fields)
	}
	if len(details.Methods) != 1 || details.Methods[0].Name != "ValidateToken" {
		t.Errorf("expected ValidateToken method, got %+v", details.Methods)
	}
	if len(details.Implements) != 1 || details.Implements[0].Name != "TokenValidator" {
		t.Errorf("expected TokenValidator implementation, got %+v", details.Implements)
	}
}

func TestQueryEngine_GetCallHierarchy(t *testing.T) {
	store, engine := setupTestGraph(t)
	defer store.Close()
	ctx := context.Background()

	// Upstream hierarchy for ValidateToken
	tree, err := engine.GetCallHierarchy(ctx, "pkg.AuthService.ValidateToken", domain.DirectionUpstream, 3)
	if err != nil {
		t.Fatalf("GetCallHierarchy failed: %v", err)
	}
	if tree.Root.ID != "pkg.AuthService.ValidateToken" {
		t.Errorf("expected root ValidateToken, got %s", tree.Root.ID)
	}
	if len(tree.Nodes) != 2 {
		t.Errorf("expected 2 caller nodes in hierarchy, got %d", len(tree.Nodes))
	}

	// Cyclic call hierarchy termination
	cyclicTree, err := engine.GetCallHierarchy(ctx, "pkg.CyclicA", domain.DirectionDownstream, 5)
	if err != nil {
		t.Fatalf("GetCallHierarchy cyclic failed: %v", err)
	}
	if len(cyclicTree.Nodes) != 1 {
		t.Fatalf("expected 1 child node for CyclicA, got %d", len(cyclicTree.Nodes))
	}
	childB := cyclicTree.Nodes[0]
	if len(childB.Children) != 1 {
		t.Fatalf("expected 1 grandchild node for CyclicB, got %d", len(childB.Children))
	}
	grandChildA := childB.Children[0]
	if !grandChildA.IsCircular {
		t.Errorf("expected grandChildA to be marked circular")
	}
}

func TestQueryEngine_LookupSymbol(t *testing.T) {
	store, engine := setupTestGraph(t)
	defer store.Close()
	ctx := context.Background()

	// Filter by prefix
	results, err := engine.LookupSymbol(ctx, domain.SymbolFilter{
		Prefix: "Auth",
	})
	if err != nil {
		t.Fatalf("LookupSymbol failed: %v", err)
	}
	if len(results) != 1 || results[0].Name != "AuthService" {
		t.Errorf("expected AuthService symbol, got: %+v", results)
	}

	// Filter by kind
	funcs, err := engine.LookupSymbol(ctx, domain.SymbolFilter{
		Kind: domain.KindFunc,
	})
	if err != nil {
		t.Fatalf("LookupSymbol for funcs failed: %v", err)
	}
	if len(funcs) != 4 { // LoginHandler, RefreshHandler, CyclicA, CyclicB
		t.Errorf("expected 4 funcs, got %d", len(funcs))
	}
}

func BenchmarkQueryEngine_FindCallers(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")
	store, err := sqlite.NewStore(dbPath)
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	// Populate 1000 nodes and edges
	nodes := make([]domain.SymbolNode, 1000)
	edges := make([]domain.Edge, 1000)
	for i := 0; i < 1000; i++ {
		id := domain.SymbolRef(string(rune('a'+i%26)) + string(rune('0'+i%10)) + "-symbol")
		nodes[i] = domain.SymbolNode{
			ID:         id,
			Kind:       domain.KindFunc,
			Name:       string(id),
			Package:    "benchpkg",
			File:       "bench.go",
			LineStart:  i,
			LineEnd:    i + 5,
			IsExported: true,
		}
		edges[i] = domain.Edge{
			ID:       string(id) + "-call",
			SourceID: id,
			TargetID: "target-symbol",
			Kind:     domain.EdgeCalls,
			File:     "bench.go",
			Line:     i,
		}
	}
	targetNode := domain.SymbolNode{
		ID:         "target-symbol",
		Kind:       domain.KindFunc,
		Name:       "target-symbol",
		Package:    "benchpkg",
		File:       "bench.go",
		LineStart:  1,
		LineEnd:    10,
		IsExported: true,
	}
	nodes = append(nodes, targetNode)

	_ = store.SaveBatch(ctx, map[string]string{"bench.go": "h"}, nodes, edges)
	engine := sqlite.NewQueryEngine(store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		res, err := engine.FindCallers(ctx, "target-symbol")
		if err != nil || len(res) == 0 {
			b.Fatalf("FindCallers failed: %v", err)
		}
		duration := time.Since(start)
		if duration > time.Millisecond*5 {
			b.Logf("Warning: latency %v exceeded 5ms", duration)
		}
	}
}

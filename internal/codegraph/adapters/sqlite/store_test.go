package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"gaia/internal/codegraph/adapters/sqlite"
	"gaia/internal/codegraph/domain"
)

func TestStore_InitAndBatchOperations(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "codegraph_test.db")

	store, err := sqlite.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Initial hashes should be empty
	hashes, err := store.GetFileHashes(ctx)
	if err != nil {
		t.Fatalf("failed to get file hashes: %v", err)
	}
	if len(hashes) != 0 {
		t.Fatalf("expected 0 hashes, got %d", len(hashes))
	}

	// Save a batch of files, nodes, and edges
	testFiles := map[string]string{
		"pkg/service.go": "hash123",
	}
	testNodes := []domain.SymbolNode{
		{
			ID:         "pkg.Service",
			Kind:       domain.KindStruct,
			Name:       "Service",
			Package:    "pkg",
			File:       "pkg/service.go",
			LineStart:  10,
			LineEnd:    25,
			Doc:        "Service handles business logic.",
			IsExported: true,
		},
		{
			ID:         "pkg.Service.Process",
			Kind:       domain.KindMethod,
			Name:       "Process",
			Package:    "pkg",
			File:       "pkg/service.go",
			LineStart:  30,
			LineEnd:    45,
			Signature:  "() error",
			Doc:        "Process executes the service.",
			IsExported: true,
		},
	}
	testEdges := []domain.Edge{
		{
			ID:       "edge-1",
			SourceID: "pkg.Service",
			TargetID: "pkg.Service.Process",
			Kind:     domain.EdgeContains,
			File:     "pkg/service.go",
			Line:     30,
		},
	}

	err = store.SaveBatch(ctx, testFiles, testNodes, testEdges)
	if err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	// Verify hashes updated
	hashes, err = store.GetFileHashes(ctx)
	if err != nil {
		t.Fatalf("GetFileHashes failed: %v", err)
	}
	if hashes["pkg/service.go"] != "hash123" {
		t.Fatalf("expected hash123, got %s", hashes["pkg/service.go"])
	}

	// Verify node retrieved
	node, err := store.GetNode(ctx, "pkg.Service")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node.Name != "Service" || node.Kind != domain.KindStruct {
		t.Fatalf("unexpected node retrieved: %+v", node)
	}

	// Test stale file deletion
	err = store.DeleteStaleFile(ctx, "pkg/service.go")
	if err != nil {
		t.Fatalf("DeleteStaleFile failed: %v", err)
	}

	hashes, err = store.GetFileHashes(ctx)
	if err != nil {
		t.Fatalf("GetFileHashes failed: %v", err)
	}
	if _, exists := hashes["pkg/service.go"]; exists {
		t.Fatalf("expected file to be deleted from hashes")
	}

	_, err = store.GetNode(ctx, "pkg.Service")
	if err != domain.ErrSymbolNotFound {
		t.Fatalf("expected ErrSymbolNotFound, got %v", err)
	}
}

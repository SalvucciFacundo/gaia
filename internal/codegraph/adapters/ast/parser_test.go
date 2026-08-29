package ast_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	astadapter "gaia/internal/codegraph/adapters/ast"
	"gaia/internal/codegraph/adapters/sqlite"
	"gaia/internal/codegraph/domain"
)

func TestParser_ParseFile(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "service.go")

	src := `// Package testservice provides service logic.
package testservice

import (
	"context"
	"fmt"
)

// Greeter defines greeting interface.
type Greeter interface {
	Greet(ctx context.Context, name string) (string, error)
}

// Service implements Greeter.
type Service struct {
	Prefix string
}

// Greet greets someone.
func (s *Service) Greet(ctx context.Context, name string) (string, error) {
	msg := s.format(name)
	return msg, nil
}

func (s *Service) format(name string) string {
	return fmt.Sprintf("%s, %s!", s.Prefix, name)
}

// Run executes the service.
func Run(s *Service) {
	s.Greet(context.Background(), "Alice")
}
`
	if err := os.WriteFile(sourceFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	parser := astadapter.NewParser()
	result, err := parser.ParseFile(sourceFile, "testservice")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify symbols
	symbolMap := make(map[string]domain.SymbolNode)
	for _, node := range result.Nodes {
		symbolMap[node.Name] = node
	}

	if _, ok := symbolMap["Greeter"]; !ok {
		t.Errorf("expected Greeter interface node")
	}
	if symbolMap["Greeter"].Kind != domain.KindInterface {
		t.Errorf("expected Greeter to be interface, got %s", symbolMap["Greeter"].Kind)
	}

	if _, ok := symbolMap["Service"]; !ok {
		t.Errorf("expected Service struct node")
	}
	if symbolMap["Service"].Kind != domain.KindStruct {
		t.Errorf("expected Service to be struct, got %s", symbolMap["Service"].Kind)
	}

	if _, ok := symbolMap["Greet"]; !ok {
		t.Errorf("expected Greet method node")
	}
	if symbolMap["Greet"].Kind != domain.KindMethod {
		t.Errorf("expected Greet to be method, got %s", symbolMap["Greet"].Kind)
	}

	if _, ok := symbolMap["Run"]; !ok {
		t.Errorf("expected Run func node")
	}
	if symbolMap["Run"].Kind != domain.KindFunc {
		t.Errorf("expected Run to be func, got %s", symbolMap["Run"].Kind)
	}

	// Verify edges
	var hasReceiver, hasCalls, hasImplements, hasContains bool
	for _, edge := range result.Edges {
		if edge.Kind == domain.EdgeReceiverOf {
			hasReceiver = true
		}
		if edge.Kind == domain.EdgeCalls {
			hasCalls = true
		}
		if edge.Kind == domain.EdgeImplements {
			hasImplements = true
		}
		if edge.Kind == domain.EdgeContains {
			hasContains = true
		}
	}

	if !hasReceiver {
		t.Errorf("expected RECEIVER_OF edge")
	}
	if !hasCalls {
		t.Errorf("expected CALLS edge")
	}
	if !hasImplements {
		t.Errorf("expected IMPLEMENTS edge from Service to Greeter")
	}
	if !hasContains {
		t.Errorf("expected CONTAINS edge")
	}
}

func TestIndexer_WorkspaceAndIncremental(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "index.db")
	workspace := filepath.Join(tempDir, "workspace")

	pkgDir := filepath.Join(workspace, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	file1 := filepath.Join(pkgDir, "a.go")
	file2 := filepath.Join(pkgDir, "b.go")
	invalidFile := filepath.Join(pkgDir, "invalid.go")

	if err := os.WriteFile(file1, []byte("package pkg\n\nfunc Helper() string { return \"ok\" }\n"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("package pkg\n\nfunc Main() { Helper() }\n"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	// Malformed file that should not crash the indexer
	if err := os.WriteFile(invalidFile, []byte("package pkg\n\nfunc Broken( { \n"), 0644); err != nil {
		t.Fatalf("failed to write invalidFile: %v", err)
	}

	store, err := sqlite.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	indexer := astadapter.NewIndexer(store)
	if err := indexer.IndexWorkspace(ctx, workspace); err != nil {
		t.Fatalf("IndexWorkspace failed: %v", err)
	}

	// Verify helper was indexed
	node, err := store.GetNode(ctx, "pkg.Helper")
	if err != nil {
		t.Fatalf("failed to find pkg.Helper: %v", err)
	}
	if node.Name != "Helper" {
		t.Errorf("expected Helper node, got %s", node.Name)
	}

	// Modify file2 and re-index incrementally
	if err := os.WriteFile(file2, []byte("package pkg\n\nfunc Main() { /* modified */ Helper() }\n"), 0644); err != nil {
		t.Fatalf("failed to update file2: %v", err)
	}

	if err := indexer.IndexWorkspace(ctx, workspace); err != nil {
		t.Fatalf("incremental IndexWorkspace failed: %v", err)
	}

	// Delete file2 and ensure it gets cleaned up
	_ = os.Remove(file2)
	if err := indexer.IndexWorkspace(ctx, workspace); err != nil {
		t.Fatalf("IndexWorkspace after deletion failed: %v", err)
	}

	_, err = store.GetNode(ctx, "pkg.Main")
	if err != domain.ErrSymbolNotFound {
		t.Errorf("expected pkg.Main to be cleaned up, got err = %v", err)
	}
}

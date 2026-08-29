package codegraph_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gaia/internal/codegraph"
)

func TestModule_ToolsExecution(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "codegraph.db")
	workspace := filepath.Join(tempDir, "workspace")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	testSrc := `package sample

type Worker interface {
	DoWork() string
}

type MyWorker struct {
	ID string
}

func (w *MyWorker) DoWork() string {
	return w.helper()
}

func (w *MyWorker) helper() string {
	return "done"
}
`
	if err := os.WriteFile(filepath.Join(workspace, "worker.go"), []byte(testSrc), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	mod, err := codegraph.NewModule(dbPath)
	if err != nil {
		t.Fatalf("failed to init module: %v", err)
	}
	defer mod.Close()

	ctx := context.Background()

	// 1. Index Workspace Tool
	res, err := mod.Execute(ctx, "codegraph_index_workspace", map[string]interface{}{
		"path": workspace,
	})
	if err != nil || !res.Success {
		t.Fatalf("codegraph_index_workspace failed: %v, out: %+v", err, res)
	}

	// 2. Lookup Symbol Tool
	res, err = mod.Execute(ctx, "codegraph_lookup_symbol", map[string]interface{}{
		"name": "MyWorker",
	})
	if err != nil || !res.Success {
		t.Fatalf("codegraph_lookup_symbol failed: %v, out: %+v", err, res)
	}

	// 3. Find Implementations Tool
	res, err = mod.Execute(ctx, "codegraph_find_implementations", map[string]interface{}{
		"symbol": "sample.Worker",
	})
	if err != nil || !res.Success {
		t.Fatalf("codegraph_find_implementations failed: %v, out: %+v", err, res)
	}

	// 4. Struct Details Tool
	res, err = mod.Execute(ctx, "codegraph_struct_details", map[string]interface{}{
		"symbol": "sample.MyWorker",
	})
	if err != nil || !res.Success {
		t.Fatalf("codegraph_struct_details failed: %v, out: %+v", err, res)
	}

	// 5. Find Callers Tool
	res, err = mod.Execute(ctx, "codegraph_find_callers", map[string]interface{}{
		"symbol": "sample.MyWorker.helper",
	})
	if err != nil || !res.Success {
		t.Fatalf("codegraph_find_callers failed: %v, out: %+v", err, res)
	}

	// 6. Find Callees Tool
	res, err = mod.Execute(ctx, "codegraph_find_callees", map[string]interface{}{
		"symbol": "sample.MyWorker.DoWork",
	})
	if err != nil || !res.Success {
		t.Fatalf("codegraph_find_callees failed: %v, out: %+v", err, res)
	}

	// 7. Call Hierarchy Tool
	res, err = mod.Execute(ctx, "codegraph_call_hierarchy", map[string]interface{}{
		"symbol":    "sample.MyWorker.DoWork",
		"direction": "DOWNSTREAM",
		"max_depth": 3,
	})
	if err != nil || !res.Success {
		t.Fatalf("codegraph_call_hierarchy failed: %v, out: %+v", err, res)
	}
}

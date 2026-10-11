package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"gaia/internal/core/domain"
)

func TestNewToolCatalogAndRegister(t *testing.T) {
	cat := NewToolCatalog()
	if cat == nil {
		t.Fatal("expected non-nil catalog")
	}

	if len(cat.List()) != 0 {
		t.Fatalf("expected 0 entries in fresh catalog, got %d", len(cat.List()))
	}

	entry := CatalogEntry{
		Name:        "custom_tool",
		Category:    "gateway",
		Description: "A custom test tool",
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			return &domain.ToolResult{Success: true, Output: "custom output"}, nil
		},
	}
	cat.Register(entry)

	got, ok := cat.Get("custom_tool")
	if !ok {
		t.Fatal("expected tool to be found")
	}
	if got.Name != "custom_tool" {
		t.Errorf("expected name 'custom_tool', got %q", got.Name)
	}
	if got.Category != "gateway" {
		t.Errorf("expected category 'gateway', got %q", got.Category)
	}
	if got.Definition.Name != "custom_tool" {
		t.Errorf("expected Definition.Name to be populated, got %q", got.Definition.Name)
	}
	if got.Definition.Description != "A custom test tool" {
		t.Errorf("expected Definition.Description to be populated, got %q", got.Definition.Description)
	}
}

func TestToolCatalog_DefaultCatalog(t *testing.T) {
	cat := DefaultCatalog()
	if cat == nil {
		t.Fatal("expected non-nil default catalog")
	}

	expectedTools := []string{
		"web_search",
		"web_fetch",
		"search_memory",
		"bash",
		"fileops",
		"gitops",
		"generate_image",
		"analyze_image",
	}

	for _, name := range expectedTools {
		entry, ok := cat.Get(name)
		if !ok {
			t.Errorf("expected default catalog to contain %q", name)
			continue
		}
		if entry.Category == "" {
			t.Errorf("expected category to be set for %q", name)
		}
		if entry.Executor == nil {
			t.Errorf("expected executor to be defined for %q", name)
		}
	}
}

func TestToolCatalog_ListAndListByCategory(t *testing.T) {
	cat := DefaultCatalog()

	all := cat.List()
	if len(all) < 8 {
		t.Errorf("expected at least 8 tools in default catalog, got %d", len(all))
	}

	// Verify sorting
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Errorf("expected catalog entries to be sorted by name: %s > %s", all[i-1].Name, all[i].Name)
		}
	}

	// Filter by categories
	webTools := cat.ListByCategory("web")
	if len(webTools) != 2 {
		t.Errorf("expected 2 web tools (web_search, web_fetch), got %d", len(webTools))
	}
	for _, w := range webTools {
		if w.Category != "web" {
			t.Errorf("expected category 'web', got %q", w.Category)
		}
	}

	// Case-insensitive category lookup
	multimodalTools := cat.ListByCategory("MULTIMODAL")
	if len(multimodalTools) < 2 {
		t.Errorf("expected at least 2 multimodal tools, got %d", len(multimodalTools))
	}
}

func TestToolCatalog_Filter(t *testing.T) {
	cat := DefaultCatalog()

	requested := []string{"bash", "web_search", "nonexistent", "gitops"}
	filtered := cat.Filter(requested)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered entries, got %d", len(filtered))
	}
	if filtered[0].Name != "bash" {
		t.Errorf("expected first filtered tool to be 'bash', got %q", filtered[0].Name)
	}
	if filtered[1].Name != "web_search" {
		t.Errorf("expected second filtered tool to be 'web_search', got %q", filtered[1].Name)
	}
	if filtered[2].Name != "gitops" {
		t.Errorf("expected third filtered tool to be 'gitops', got %q", filtered[2].Name)
	}
}

func TestToolCatalog_Execute(t *testing.T) {
	ctx := context.Background()
	cat := DefaultCatalog()

	// 1. Success execution
	res, err := cat.Execute(ctx, "bash", map[string]interface{}{"command": "ls -la"})
	if err != nil {
		t.Fatalf("unexpected error executing bash: %v", err)
	}
	if !res.Success {
		t.Errorf("expected tool success, got failure: %s", res.Error)
	}
	if res.Output != "Executed bash command: ls -la" {
		t.Errorf("unexpected output: %q", res.Output)
	}

	// 2. Missing required argument error from executor
	res, err = cat.Execute(ctx, "bash", map[string]interface{}{})
	if err == nil {
		t.Errorf("expected error executing bash without command")
	}
	if res == nil || res.Success {
		t.Errorf("expected failure result, got %v", res)
	}

	// 3. Unknown tool error
	res, err = cat.Execute(ctx, "unknown_tool", map[string]interface{}{})
	if err == nil {
		t.Errorf("expected error executing unknown tool")
	}
	if res == nil || res.Success {
		t.Errorf("expected failure result for unknown tool")
	}

	// 4. Tool with nil executor
	cat.Register(CatalogEntry{
		Name:     "no_exec_tool",
		Category: "gateway",
	})
	res, err = cat.Execute(ctx, "no_exec_tool", map[string]interface{}{})
	if err == nil {
		t.Errorf("expected error executing tool without executor")
	}
	if res == nil || res.Success {
		t.Errorf("expected failure result for tool without executor")
	}
}

func TestToolCatalog_ConcurrentAccess(t *testing.T) {
	cat := DefaultCatalog()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		toolIndex := i
		go func() {
			defer wg.Done()
			cat.Register(CatalogEntry{
				Name:        fmt.Sprintf("dynamic_%d", toolIndex),
				Category:    "shell",
				Description: "dynamic",
				Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
					return &domain.ToolResult{Success: true, Output: "ok"}, nil
				},
			})
		}()
		go func() {
			defer wg.Done()
			_ = cat.List()
			_ = cat.ListByCategory("shell")
			_ = cat.Filter([]string{"bash", "web_search"})
		}()
		go func() {
			defer wg.Done()
			_, _ = cat.Execute(ctx, "web_search", map[string]interface{}{"query": "go"})
		}()
	}
	wg.Wait()
}

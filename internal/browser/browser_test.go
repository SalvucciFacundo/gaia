package browser

import (
	"testing"

	"gaia/internal/core/domain"
)

func TestNewModule_NotEnabled(t *testing.T) {
	cfg := domain.BrowserToolsConfig{
		Enabled: false,
	}
	_, err := NewModule(cfg)
	if err == nil {
		t.Error("expected error for disabled browser tools")
	}
}

func TestNewModule_NoCommand(t *testing.T) {
	cfg := domain.BrowserToolsConfig{
		Enabled: true,
		Command: "",
	}
	_, err := NewModule(cfg)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestModuleName(t *testing.T) {
	cfg := domain.BrowserToolsConfig{
		Enabled: true,
		Command: "npx playwright-mcp",
	}
	mod, err := NewModule(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mod.Name() != "browser" {
		t.Errorf("expected name 'browser', got %q", mod.Name())
	}
	if mod.Description() == "" {
		t.Error("expected non-empty description")
	}
}

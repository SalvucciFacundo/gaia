package profiles

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/tools"
	"gaia/internal/tools/web/search"
)

func setupTestManager() (*Manager, *domain.Config, *int) {
	saveCount := 0
	cfg := &domain.Config{
		CurrentProfile: "assistant",
		Profiles: map[string]domain.ProfileConfig{
			"assistant": {Enabled: true},
			"dev":       {Enabled: false},
			"art":       {Enabled: false},
		},
	}
	saveFn := func(c *domain.Config) error {
		saveCount++
		return nil
	}

	mgr := NewManager(cfg, saveFn)
	mgr.Register(NewAssistantProfile())
	mgr.Register(NewDevProfile())
	mgr.Register(NewArtProfile())

	return mgr, cfg, &saveCount
}

func TestDefaultProfileResolutionAndActiveTools(t *testing.T) {
	mgr, cfg, _ := setupTestManager()

	active := mgr.Active()
	if active == nil {
		t.Fatal("expected active profile, got nil")
	}
	if active.ID() != "assistant" {
		t.Errorf("expected active profile ID 'assistant', got %q", active.ID())
	}
	if active.Name() != "Asistente Personal" {
		t.Errorf("expected active profile name 'Asistente Personal', got %q", active.Name())
	}

	tools := mgr.ActiveTools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools for assistant, got %d", len(tools))
	}
	foundWebSearch := false
	foundWebFetch := false
	foundMemorySearch := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			foundWebSearch = true
		}
		if tool.Name == "web_fetch" {
			foundWebFetch = true
		}
		if tool.Name == "search_memory" {
			foundMemorySearch = true
		}
	}
	if !foundWebSearch {
		t.Errorf("expected active tools to include 'web_search'")
	}
	if !foundWebFetch {
		t.Errorf("expected active tools to include 'web_fetch'")
	}
	if !foundMemorySearch {
		t.Errorf("expected active tools to include 'search_memory'")
	}

	prompt := mgr.ActiveSystemPrompt()
	if prompt == "" {
		t.Errorf("expected non-empty active system prompt")
	}

	list := mgr.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 profiles in list, got %d", len(list))
	}
	// Verify sorted by ID: art, assistant, dev
	expectedOrder := []string{"art", "assistant", "dev"}
	for i, expected := range expectedOrder {
		if list[i].ID != expected {
			t.Errorf("expected list[%d].ID = %q, got %q", i, expected, list[i].ID)
		}
	}

	// Verify active and enabled flags
	for _, info := range list {
		if info.ID == "assistant" {
			if !info.Active || !info.Enabled {
				t.Errorf("assistant profile should be active and enabled")
			}
		} else {
			if info.Active || info.Enabled {
				t.Errorf("profile %q should be inactive and disabled", info.ID)
			}
		}
	}

	_ = cfg
}

func TestSetActiveSwitchAndSave(t *testing.T) {
	mgr, cfg, saveCount := setupTestManager()

	// Enable dev profile first
	if err := mgr.Enable("dev"); err != nil {
		t.Fatalf("unexpected error enabling dev: %v", err)
	}

	initialSaves := *saveCount

	// Switch to dev
	if err := mgr.SetActive("dev"); err != nil {
		t.Fatalf("unexpected error setting active profile to dev: %v", err)
	}

	if cfg.CurrentProfile != "dev" {
		t.Errorf("expected cfg.CurrentProfile = 'dev', got %q", cfg.CurrentProfile)
	}
	if mgr.Active().ID() != "dev" {
		t.Errorf("expected mgr.Active().ID() = 'dev', got %q", mgr.Active().ID())
	}
	if *saveCount <= initialSaves {
		t.Errorf("expected saveFn to be called on SetActive")
	}

	tools := mgr.ActiveTools()
	if len(tools) != 3 {
		t.Errorf("expected 3 tools for dev profile, got %d", len(tools))
	}
}

func TestSetActiveFailsOnDisabledOrNonexistent(t *testing.T) {
	mgr, _, _ := setupTestManager()

	// "dev" is disabled by default
	err := mgr.SetActive("dev")
	if err == nil {
		t.Fatal("expected error when setting disabled profile active, got nil")
	}

	// "nonexistent" is not registered
	err = mgr.SetActive("nonexistent")
	if err == nil {
		t.Fatal("expected error when setting nonexistent profile active, got nil")
	}
}

func TestEnableAndDisableWorkflow(t *testing.T) {
	mgr, cfg, saveCount := setupTestManager()

	// Enable art
	if err := mgr.Enable("art"); err != nil {
		t.Fatalf("unexpected error enabling art: %v", err)
	}
	if !cfg.Profiles["art"].Enabled {
		t.Errorf("expected art profile to be enabled in cfg")
	}
	savesAfterEnable := *saveCount

	// Disable art
	if err := mgr.Disable("art"); err != nil {
		t.Fatalf("unexpected error disabling art: %v", err)
	}
	if cfg.Profiles["art"].Enabled {
		t.Errorf("expected art profile to be disabled in cfg")
	}
	if *saveCount <= savesAfterEnable {
		t.Errorf("expected saveFn to be called on Disable")
	}

	// Attempting to enable/disable unregistered profile should error
	if err := mgr.Enable("ghost"); err == nil {
		t.Errorf("expected error enabling unregistered profile")
	}
	if err := mgr.Disable("ghost"); err == nil {
		t.Errorf("expected error disabling unregistered profile")
	}
}

func TestCannotDisableActiveProfile(t *testing.T) {
	mgr, cfg, _ := setupTestManager()

	// "assistant" is currently active
	err := mgr.Disable("assistant")
	if err == nil {
		t.Fatal("expected error when disabling active profile, got nil")
	}

	if !cfg.Profiles["assistant"].Enabled {
		t.Errorf("active profile should have remained enabled")
	}
}

func TestToolRoutingViaResolveTool(t *testing.T) {
	mgr, _, _ := setupTestManager()

	// Active profile is assistant -> resolve web_search
	p, found := mgr.ResolveTool("web_search")
	if !found || p == nil {
		t.Fatal("expected to resolve 'web_search'")
	}
	if p.ID() != "assistant" {
		t.Errorf("expected 'web_search' to resolve to 'assistant', got %q", p.ID())
	}

	// Dev profile tool -> code_search
	p, found = mgr.ResolveTool("code_search")
	if !found || p == nil {
		t.Fatal("expected to resolve 'code_search'")
	}
	if p.ID() != "dev" {
		t.Errorf("expected 'code_search' to resolve to 'dev', got %q", p.ID())
	}

	// Art profile tool -> generate_image
	p, found = mgr.ResolveTool("generate_image")
	if !found || p == nil {
		t.Fatal("expected to resolve 'generate_image'")
	}
	if p.ID() != "art" {
		t.Errorf("expected 'generate_image' to resolve to 'art', got %q", p.ID())
	}

	// Nonexistent tool
	_, found = mgr.ResolveTool("unknown_tool_xyz")
	if found {
		t.Errorf("expected false for unknown tool")
	}
}

func TestProfileExecuteTool(t *testing.T) {
	ctx := context.Background()

	// Assistant
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<html><body><div class="result web-result"><h2 class="result__title"><a class="result__a" href="https://golang.org">Go</a></h2><a class="result__snippet">Go language</a></div></body></html>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	ddg := search.NewDuckDuckGoSearcher(search.WithDuckDuckGoEndpoint(server.URL))
	asst := NewAssistantProfile().WithSearchRunner(tools.NewWebSearchRunner(ddg, "duckduckgo", search.WebProviders{}))

	res, err := asst.ExecuteTool(ctx, "web_search", map[string]interface{}{"query": "golang"})
	if err != nil || !res.Success {
		t.Errorf("assistant web_search failed: %v", err)
	}
	res, err = asst.ExecuteTool(ctx, "unknown", nil)
	if err == nil || res.Success {
		t.Errorf("expected assistant unknown tool to fail")
	}

	// Dev
	dev := NewDevProfile()
	res, err = dev.ExecuteTool(ctx, "code_search", map[string]interface{}{"pattern": "func main"})
	if err != nil || !res.Success {
		t.Errorf("dev code_search failed: %v", err)
	}
	res, err = dev.ExecuteTool(ctx, "unknown", nil)
	if err == nil || res.Success {
		t.Errorf("expected dev unknown tool to fail")
	}

	// Art
	art := NewArtProfile()
	res, err = art.ExecuteTool(ctx, "generate_image", map[string]interface{}{"prompt": "cyberpunk city"})
	if err != nil || !res.Success {
		t.Errorf("art generate_image failed: %v", err)
	}
	res, err = art.ExecuteTool(ctx, "unknown", nil)
	if err == nil || res.Success {
		t.Errorf("expected art unknown tool to fail")
	}
}

func TestConcurrentManagerAccess(t *testing.T) {
	mgr, _, _ := setupTestManager()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = mgr.Active()
			_ = mgr.ActiveTools()
			_ = mgr.ActiveSystemPrompt()
		}()
		go func() {
			defer wg.Done()
			_ = mgr.List()
			_, _ = mgr.ResolveTool("web_search")
			_, _ = mgr.ResolveTool("code_search")
		}()
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_ = mgr.Enable("dev")
			} else {
				_ = mgr.Enable("art")
			}
		}(i)
	}
	wg.Wait()
}

func TestSaveFnErrorHandling(t *testing.T) {
	cfg := &domain.Config{
		CurrentProfile: "assistant",
		Profiles: map[string]domain.ProfileConfig{
			"assistant": {Enabled: true},
			"dev":       {Enabled: true},
		},
	}
	saveErr := fmt.Errorf("disk full")
	mgr := NewManager(cfg, func(c *domain.Config) error {
		return saveErr
	})
	mgr.Register(NewAssistantProfile())
	mgr.Register(NewDevProfile())

	if err := mgr.SetActive("dev"); err == nil {
		t.Error("expected error when saveFn fails in SetActive")
	}
	if err := mgr.Enable("dev"); err == nil {
		t.Error("expected error when saveFn fails in Enable")
	}
}

func TestAssistantProfile_PersonaAndSoulIntegration(t *testing.T) {
	tempDir := t.TempDir()
	soulPath := tempDir + "/SOUL.md"
	customSoul := "# Custom Soul\n\nYou are a helpful and concise assistant.\n"
	if err := os.WriteFile(soulPath, []byte(customSoul), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	p := NewAssistantProfile().WithSoulPath(soulPath)
	prompt := p.SystemPrompt()

	if !strings.Contains(prompt, "Custom Soul") {
		t.Errorf("expected prompt to contain custom soul, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "helpful and concise assistant") {
		t.Errorf("expected prompt to contain custom soul text, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "You are GAIA") {
		t.Errorf("expected prompt to contain GAIA guidelines, got:\n%s", prompt)
	}
}

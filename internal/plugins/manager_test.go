package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerLoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")

	mgr := NewManager(pluginsDir)
	if err := mgr.Load(); err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(mgr.List()) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(mgr.List()))
	}
}

func TestManagerLoadNonExistentDir(t *testing.T) {
	mgr := NewManager("/nonexistent/path")
	if err := mgr.Load(); err != nil {
		t.Fatalf("load should not error on nonexistent dir: %v", err)
	}
}

func TestManagerLoadPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")

	// Create a plugin directory with manifest.
	pluginDir := filepath.Join(pluginsDir, "my-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `{"name": "my-plugin", "version": "1.0.0", "description": "Test plugin", "command": "./plugin.sh", "tools": ["greet", "sum"]}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(pluginsDir)
	if err := mgr.Load(); err != nil {
		t.Fatalf("load error: %v", err)
	}

	plugins := mgr.List()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	p := plugins[0]
	if p.Manifest.Name != "my-plugin" {
		t.Errorf("expected name 'my-plugin', got %q", p.Manifest.Name)
	}
	if p.Manifest.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", p.Manifest.Version)
	}
	if !p.Enabled {
		t.Error("new plugins should be enabled by default")
	}
}

func TestManagerEnableDisable(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")

	pluginDir := filepath.Join(pluginsDir, "test-plugin")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name": "test-plugin", "version": "1.0.0", "command": "./test", "tools": []}`), 0644)

	mgr := NewManager(pluginsDir)
	mgr.Load()

	if err := mgr.Disable("test-plugin"); err != nil {
		t.Fatalf("disable error: %v", err)
	}

	p, _ := mgr.Get("test-plugin")
	if p.Enabled {
		t.Error("plugin should be disabled")
	}

	if err := mgr.Enable("test-plugin"); err != nil {
		t.Fatalf("enable error: %v", err)
	}

	p, _ = mgr.Get("test-plugin")
	if !p.Enabled {
		t.Error("plugin should be enabled")
	}
}

func TestManagerGetNotFound(t *testing.T) {
	mgr := NewManager(t.TempDir())
	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestManagerToolsForPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")

	pluginDir := filepath.Join(pluginsDir, "tool-plugin")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name": "tool-plugin", "version": "1.0.0", "command": "./tool", "tools": ["greet", "calc"]}`), 0644)

	mgr := NewManager(pluginsDir)
	mgr.Load()

	tools, err := mgr.ToolsForPlugin("tool-plugin")
	if err != nil {
		t.Fatalf("ToolsForPlugin error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	expectedNames := map[string]bool{
		"plugin_tool-plugin_greet": true,
		"plugin_tool-plugin_calc":  true,
	}
	for _, tc := range tools {
		if !expectedNames[tc.Name] {
			t.Errorf("unexpected tool name: %s", tc.Name)
		}
	}
}

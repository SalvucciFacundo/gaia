package config

import (
	"os"
	"path/filepath"
	"testing"

	"gaia/internal/core/domain"
	"gopkg.in/yaml.v3"
)

func TestLoadDefaultProfileInitialization(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Case 1: Fresh config (file does not exist)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load fresh config: %v", err)
	}

	if cfg.CurrentProfile != "assistant" {
		t.Errorf("expected default CurrentProfile to be 'assistant', got %q", cfg.CurrentProfile)
	}

	if cfg.Profiles == nil {
		t.Fatal("expected Profiles map to be initialized, got nil")
	}

	expectedProfiles := map[string]bool{
		"assistant": true,
		"dev":       false,
		"art":       false,
	}

	for id, expectedEnabled := range expectedProfiles {
		pCfg, ok := cfg.Profiles[id]
		if !ok {
			t.Errorf("expected profile %q in default profiles", id)
			continue
		}
		if pCfg.Enabled != expectedEnabled {
			t.Errorf("expected profile %q enabled=%v, got %v", id, expectedEnabled, pCfg.Enabled)
		}
	}
}

func TestLoadExistingConfigWithoutProfiles(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	configFilePath := filepath.Join(tempHome, configDir, configFile)
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Write raw config with no CurrentProfile or Profiles
	rawYAML := `
api_keys:
  openai: "test-key"
`
	if err := os.WriteFile(configFilePath, []byte(rawYAML), 0600); err != nil {
		t.Fatalf("failed to write raw config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load existing config: %v", err)
	}

	if cfg.CurrentProfile != "assistant" {
		t.Errorf("expected CurrentProfile = 'assistant', got %q", cfg.CurrentProfile)
	}
	if cfg.Profiles == nil {
		t.Fatal("expected Profiles map to be initialized, got nil")
	}
	if !cfg.Profiles["assistant"].Enabled {
		t.Errorf("expected assistant to be enabled")
	}
	if cfg.Profiles["dev"].Enabled {
		t.Errorf("expected dev to be disabled by default")
	}
	if cfg.Profiles["art"].Enabled {
		t.Errorf("expected art to be disabled by default")
	}
}

func TestLoadExistingConfigWithCustomProfiles(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	configFilePath := filepath.Join(tempHome, configDir, configFile)
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	customCfg := domain.Config{
		CurrentProfile: "dev",
		Profiles: map[string]domain.ProfileConfig{
			"assistant": {Enabled: true},
			"dev":       {Enabled: true, Model: "claude-3-7-sonnet"},
			"art":       {Enabled: false},
		},
	}
	data, err := yaml.Marshal(customCfg)
	if err != nil {
		t.Fatalf("failed to marshal custom config: %v", err)
	}
	if err := os.WriteFile(configFilePath, data, 0600); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.CurrentProfile != "dev" {
		t.Errorf("expected CurrentProfile = 'dev', got %q", cfg.CurrentProfile)
	}
	if !cfg.Profiles["dev"].Enabled {
		t.Errorf("expected dev profile to remain enabled")
	}
	if cfg.Profiles["dev"].Model != "claude-3-7-sonnet" {
		t.Errorf("expected dev model 'claude-3-7-sonnet', got %q", cfg.Profiles["dev"].Model)
	}
}

func TestBaseHomeAndGaiaHome(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// When GAIA_HOME is unset
	t.Setenv("GAIA_HOME", "")
	expectedDefault := filepath.Join(tempHome, ".config/gaia")
	if got := BaseHome(); got != expectedDefault {
		t.Errorf("BaseHome() = %q, want %q", got, expectedDefault)
	}
	if got := GaiaHome(); got != expectedDefault {
		t.Errorf("GaiaHome() = %q, want %q", got, expectedDefault)
	}

	// When GAIA_HOME is set
	customGaiaHome := filepath.Join(tempHome, "custom-gaia")
	t.Setenv("GAIA_HOME", customGaiaHome)
	if got := BaseHome(); got != customGaiaHome {
		t.Errorf("BaseHome() = %q, want %q", got, customGaiaHome)
	}
	if got := GaiaHome(); got != customGaiaHome {
		t.Errorf("GaiaHome() = %q, want %q", got, customGaiaHome)
	}
}

func TestValidateProfileName(t *testing.T) {
	valid := []string{"assistant", "dev", "art", "prod-1", "user_test", "A-Z_09"}
	for _, name := range valid {
		if err := ValidateProfileName(name); err != nil {
			t.Errorf("ValidateProfileName(%q) unexpected error: %v", name, err)
		}
	}

	// Explicit invalid tests:
	if err := ValidateProfileName(""); err == nil {
		t.Errorf("ValidateProfileName(\"\") expected error")
	}
	longName := "abcdefghijklmnopqrstuvwxyz01234567" // 35 chars
	if err := ValidateProfileName(longName); err == nil {
		t.Errorf("ValidateProfileName(%q) expected error for >32 chars", longName)
	}
	badChars := []string{"../dev", "dev/test", "dev\\test", "dev test", "dev@1", "dev.test"}
	for _, bc := range badChars {
		if err := ValidateProfileName(bc); err == nil {
			t.Errorf("ValidateProfileName(%q) expected error for invalid characters", bc)
		}
	}
}

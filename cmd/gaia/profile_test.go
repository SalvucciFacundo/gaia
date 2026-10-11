package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"gaia/internal/config"
)

func captureOutput(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestProfileCLI_ListAndShow(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Test default list output
	output := captureOutput(func() {
		handleProfileCLI([]string{"list"})
	})

	if !strings.Contains(output, "Perfiles disponibles en GAIA:") {
		t.Errorf("expected list header, got %q", output)
	}
	if !strings.Contains(output, "assistant") || !strings.Contains(output, "dev") || !strings.Contains(output, "art") {
		t.Errorf("expected all 3 default profiles in list, got %q", output)
	}
	if !strings.Contains(output, "ACTIVO (*)") {
		t.Errorf("expected an active profile mark, got %q", output)
	}

	// Test show current
	showOutput := captureOutput(func() {
		handleProfileCLI([]string{"show"})
	})

	if !strings.Contains(showOutput, "Perfil Activo: assistant") {
		t.Errorf("expected active profile to be assistant, got %q", showOutput)
	}
}

func TestProfileCLI_EnableAndSwitch(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Initially dev is disabled. Enabling dev should succeed.
	enableOutput := captureOutput(func() {
		handleProfileCLI([]string{"enable", "dev"})
	})
	if !strings.Contains(enableOutput, "habilitado exitosamente") {
		t.Errorf("expected success message on enable, got %q", enableOutput)
	}

	// Switching to dev should succeed.
	useOutput := captureOutput(func() {
		handleProfileCLI([]string{"use", "dev"})
	})
	if !strings.Contains(useOutput, "Perfil activo actualizado a 'dev'") {
		t.Errorf("expected success message on use dev, got %q", useOutput)
	}

	// Verify persistence in config
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if cfg.CurrentProfile != "dev" {
		t.Errorf("expected CurrentProfile = 'dev', got %q", cfg.CurrentProfile)
	}
	if !cfg.Profiles["dev"].Enabled {
		t.Errorf("expected dev to be enabled in config")
	}
}

func TestProfileManager_Helpers(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	mgr, err := getProfileManager()
	if err != nil {
		t.Fatalf("getProfileManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.Active().ID() != "assistant" {
		t.Errorf("expected active ID = assistant, got %q", mgr.Active().ID())
	}

	// Truncate helper test
	short := truncate("hello", 10)
	if short != "hello" {
		t.Errorf("truncate short string failed: got %q", short)
	}
	long := truncate("hello world from gaia", 10)
	if long != "hello w..." {
		t.Errorf("truncate long string failed: got %q", long)
	}
}

func TestProfileCLI_ToolsList(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// 1. gaia profile tools (active profile: assistant)
	out := captureOutput(func() {
		handleProfileCLI([]string{"tools"})
	})

	if !strings.Contains(out, "Herramientas para el perfil 'assistant'") {
		t.Errorf("expected header for assistant, got: %s", out)
	}
	if !strings.Contains(out, "[HABILITADA]") || !strings.Contains(out, "[DESACTIVADA]") {
		t.Errorf("expected [HABILITADA] and [DESACTIVADA] in output, got: %s", out)
	}
	if !strings.Contains(out, "web_search") || !strings.Contains(out, "bash") {
		t.Errorf("expected tools listed, got: %s", out)
	}

	// 2. gaia profile tools dev (specific profile)
	devOut := captureOutput(func() {
		handleProfileCLI([]string{"tools", "dev"})
	})
	if !strings.Contains(devOut, "Herramientas para el perfil 'dev'") {
		t.Errorf("expected header for dev, got: %s", devOut)
	}
}

func TestProfileCLI_ToolsEnableDisableReset(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// 1. Enable bash for assistant
	enableOut := captureOutput(func() {
		handleProfileCLI([]string{"tools", "enable", "assistant", "bash"})
	})
	if !strings.Contains(enableOut, "habilitada exitosamente") {
		t.Errorf("expected success enabling tool, got: %s", enableOut)
	}

	// Verify persistence in config
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	asstTools := cfg.Profiles["assistant"].Tools
	foundBash := false
	for _, tool := range asstTools {
		if tool == "bash" {
			foundBash = true
		}
	}
	if !foundBash {
		t.Errorf("expected bash in assistant tools in config, got %v", asstTools)
	}

	// 2. Verify listing shows bash as [HABILITADA]
	listOut := captureOutput(func() {
		handleProfileCLI([]string{"tools", "assistant"})
	})
	lines := strings.Split(listOut, "\n")
	bashEnabled := false
	for _, l := range lines {
		if strings.Contains(l, "bash") && strings.Contains(l, "[HABILITADA]") {
			bashEnabled = true
			break
		}
	}
	if !bashEnabled {
		t.Errorf("expected bash to show as [HABILITADA] in list output: %s", listOut)
	}

	// 3. Disable bash for assistant
	disableOut := captureOutput(func() {
		handleProfileCLI([]string{"tools", "disable", "assistant", "bash"})
	})
	if !strings.Contains(disableOut, "deshabilitada exitosamente") {
		t.Errorf("expected success disabling tool, got: %s", disableOut)
	}

	// 4. Reset assistant tools
	resetOut := captureOutput(func() {
		handleProfileCLI([]string{"tools", "reset", "assistant"})
	})
	if !strings.Contains(resetOut, "restablecidas a los valores por defecto") {
		t.Errorf("expected success resetting tools, got: %s", resetOut)
	}

	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if len(cfg.Profiles["assistant"].Tools) != 0 {
		t.Errorf("expected assistant tools to be cleared in config, got %v", cfg.Profiles["assistant"].Tools)
	}
}

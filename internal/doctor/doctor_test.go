package doctor

import (
	"context"
	"testing"
	"time"
)

func TestAllChecks(t *testing.T) {
	checks := AllChecks()
	if len(checks) == 0 {
		t.Error("expected non-empty checks list")
	}

	// Verify each check has a name
	ctx := context.Background()
	for _, check := range checks {
		result := check(ctx)
		if result.Name == "" {
			t.Error("check returned empty name")
		}
		if result.Duration < 0 {
			t.Error("check returned negative duration")
		}
		// Status should be one of the known values
		switch result.Status {
		case StatusOK, StatusWarn, StatusFail:
			// valid
		default:
			t.Errorf("unexpected status %q for check %q", result.Status, result.Name)
		}
	}
}

func TestCheckLLMProvider(t *testing.T) {
	result := CheckLLMProvider(context.Background())
	if result.Name != "LLM Provider" {
		t.Errorf("expected name 'LLM Provider', got %q", result.Name)
	}
	// LLM check should complete quickly
	if result.Duration > time.Second {
		t.Errorf("LLM check took too long: %v", result.Duration)
	}
}

func TestCheckConfig(t *testing.T) {
	result := CheckConfig(context.Background())
	if result.Name != "Config" {
		t.Errorf("expected name 'Config', got %q", result.Name)
	}
	// Config check should complete quickly
	if result.Duration > time.Second {
		t.Errorf("Config check took too long: %v", result.Duration)
	}
}

func TestCheckGoVersion(t *testing.T) {
	result := CheckGoVersion(context.Background())
	if result.Name != "Go Version" {
		t.Errorf("expected name 'Go Version', got %q", result.Name)
	}
	// Go version should be OK since we're building/testing
	if result.Status != StatusOK {
		t.Errorf("expected Go version check to pass, got %s: %s", result.Status, result.Message)
	}
}

func TestRunAll(t *testing.T) {
	results := RunAll(context.Background())
	if len(results) != len(AllChecks()) {
		t.Errorf("expected %d results, got %d", len(AllChecks()), len(results))
	}
}

func TestFormatTable(t *testing.T) {
	results := []CheckResult{
		{Name: "Test OK", Status: StatusOK, Message: "All good", Duration: time.Millisecond},
		{Name: "Test Warn", Status: StatusWarn, Message: "Something minor", Duration: time.Millisecond * 2},
		{Name: "Test Fail", Status: StatusFail, Message: "Broken", Duration: time.Millisecond * 3},
	}

	table := FormatTable(results)

	// Check that the table contains expected elements
	if len(table) == 0 {
		t.Error("expected non-empty table output")
	}

	requiredStrings := []string{
		"GAIA System Diagnostics",
		"Test OK",
		"Test Warn",
		"Test Fail",
		"Summary:",
	}

	for _, s := range requiredStrings {
		if !contains(table, s) {
			t.Errorf("table missing expected string: %q", s)
		}
	}
}

func TestBuildInfo(t *testing.T) {
	info := BuildInfo()
	if info["go_version"] == "" {
		t.Error("expected go_version in build info")
	}
	if info["os"] == "" {
		t.Error("expected os in build info")
	}
	if info["arch"] == "" {
		t.Error("expected arch in build info")
	}
}

func TestStatusIcon(t *testing.T) {
	if statusIcon(StatusOK) != "✓" {
		t.Error("expected ✓ for OK status")
	}
	if statusIcon(StatusWarn) != "⚠" {
		t.Error("expected ⚠ for Warn status")
	}
	if statusIcon(StatusFail) != "✗" {
		t.Error("expected ✗ for Fail status")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(len(s) >= len(substr)) &&
		(s[0:len(substr)] == substr ||
			findIn(s, substr))
}

func findIn(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

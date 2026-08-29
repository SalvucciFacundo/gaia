package review

import (
	"testing"
)

func TestReviewMode_DefaultOff(t *testing.T) {
	tmpDir := t.TempDir()
	status := IsEnabled(tmpDir)
	if status.Mode != ModeDisabled {
		t.Errorf("expected default mode 'disabled', got %q", status.Mode)
	}
	if status.DecidingSource != "default" {
		t.Errorf("expected deciding source 'default', got %q", status.DecidingSource)
	}
}

func TestReviewMode_CloneScope_EnableAndDisable(t *testing.T) {
	tmpDir := t.TempDir()

	// Enable clone scope
	err := SetMode(ModeEnabled, ScopeClone, tmpDir)
	if err != nil {
		t.Fatalf("SetMode enabled failed: %v", err)
	}

	status := IsEnabled(tmpDir)
	if status.Mode != ModeEnabled {
		t.Errorf("expected mode 'enabled', got %q", status.Mode)
	}
	if status.DecidingSource != "clone" {
		t.Errorf("expected deciding source 'clone', got %q", status.DecidingSource)
	}

	// Disable clone scope
	err = SetMode(ModeDisabled, ScopeClone, tmpDir)
	if err != nil {
		t.Fatalf("SetMode disabled failed: %v", err)
	}

	status = IsEnabled(tmpDir)
	if status.Mode != ModeDisabled {
		t.Errorf("expected mode 'disabled', got %q", status.Mode)
	}
	if status.DecidingSource != "clone" {
		t.Errorf("expected deciding source 'clone', got %q", status.DecidingSource)
	}
}

func TestReviewMode_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()
	err := SetMode("invalid", ScopeClone, tmpDir)
	if err == nil {
		t.Error("expected error for invalid mode, got nil")
	}
}

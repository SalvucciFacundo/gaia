package sdd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gaia/internal/agent/memory"
)

func TestStorePolicy_ParseStoreMode(t *testing.T) {
	tests := []struct {
		input    string
		expected StoreMode
	}{
		{"engram", StoreModeEngram},
		{"openspec", StoreModeOpenSpec},
		{"hybrid", StoreModeHybrid},
		{"none", StoreModeNone},
		{"", StoreModeHybrid},
		{"unknown", StoreModeHybrid},
	}

	for _, tt := range tests {
		if got := ParseStoreMode(tt.input); got != tt.expected {
			t.Errorf("ParseStoreMode(%q) = %v, expected %v", tt.input, got, tt.expected)
		}
	}
}

func TestStorePolicy_DispatcherGuards(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")

	engramPolicy := NewStorePolicy(StoreModeEngram, "", ns)
	if engramPolicy.ShouldQueryFilesystem() {
		t.Error("expected engram policy to NOT query filesystem")
	}
	if !engramPolicy.ShouldQueryEngram() {
		t.Error("expected engram policy to query engram")
	}

	openspecPolicy := NewStorePolicy(StoreModeOpenSpec, "", ns)
	if !openspecPolicy.ShouldQueryFilesystem() {
		t.Error("expected openspec policy to query filesystem")
	}
	if openspecPolicy.ShouldQueryEngram() {
		t.Error("expected openspec policy to NOT query engram")
	}

	hybridPolicy := NewStorePolicy(StoreModeHybrid, "", ns)
	if !hybridPolicy.ShouldQueryFilesystem() || !hybridPolicy.ShouldQueryEngram() {
		t.Error("expected hybrid policy to query both filesystem and engram")
	}
}

func TestStorePolicy_SaveAndRead_Hybrid(t *testing.T) {
	tmpDir := t.TempDir()
	ns := memory.NewNamespaceManager("test-project")
	policy := NewStorePolicy(StoreModeHybrid, tmpDir, ns)

	ctx := context.Background()
	changeName := "feature-test"
	artifactName := "proposal"
	content := "# Proposal\nIntent: test store policy."

	err := policy.SaveArtifact(ctx, changeName, artifactName, content)
	if err != nil {
		t.Fatalf("SaveArtifact failed: %v", err)
	}

	// Verify file was written
	filePath := filepath.Join(tmpDir, "openspec", "changes", changeName, artifactName+".md")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("expected file %q to exist", filePath)
	}

	readContent, err := policy.ReadArtifact(ctx, changeName, artifactName)
	if err != nil {
		t.Fatalf("ReadArtifact failed: %v", err)
	}

	if readContent != content {
		t.Errorf("expected %q, got %q", content, readContent)
	}
}

func TestStorePolicy_Read_EngramMode_BypassesFilesystem(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	policy := NewStorePolicy(StoreModeEngram, "/nonexistent/path", ns)

	ctx := context.Background()
	readContent, err := policy.ReadArtifact(ctx, "feature-memory", "proposal")
	if err != nil {
		t.Fatalf("unexpected error in engram mode: %v", err)
	}

	if readContent == "" {
		t.Error("expected non-empty engram descriptor")
	}
}

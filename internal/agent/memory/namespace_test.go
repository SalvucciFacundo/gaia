package memory

import (
	"strings"
	"testing"
)

// TestNamespaceFormat_Prefix verifies the subagent prefix format.
func TestNamespaceFormat_Prefix(t *testing.T) {
	mgr := NewNamespaceManager("myproject")

	want := "gaia/explorer/myproject"
	got := mgr.SubagentPrefix("explorer")
	if got != want {
		t.Errorf("SubagentPrefix: want %q, got %q", want, got)
	}
}

// TestNamespaceFormat_SharedPrefix verifies the shared namespace format.
func TestNamespaceFormat_SharedPrefix(t *testing.T) {
	mgr := NewNamespaceManager("gaia")

	want := "gaia/shared/gaia"
	got := mgr.SharedPrefix()
	if got != want {
		t.Errorf("SharedPrefix: want %q, got %q", want, got)
	}
}

// TestNamespaceFormat_DefaultProject verifies empty project gets "default".
func TestNamespaceFormat_DefaultProject(t *testing.T) {
	mgr := NewNamespaceManager("")

	if mgr.Project() != "default" {
		t.Errorf("expected project 'default' for empty input, got %q", mgr.Project())
	}
}

// TestTopicKey verifies the fully-qualified topic key construction.
func TestTopicKey(t *testing.T) {
	mgr := NewNamespaceManager("myapp")

	want := "gaia/implementer/myapp/architecture-patterns"
	got := mgr.TopicKey("implementer", "architecture-patterns")
	if got != want {
		t.Errorf("TopicKey: want %q, got %q", want, got)
	}
}

// TestSaveInstructions verifies that the save instructions contain
// the correct namespace prefixes.
func TestSaveInstructions(t *testing.T) {
	mgr := NewNamespaceManager("testproject")

	instr := mgr.SaveInstructions("verifier")

	// Should contain the subagent prefix
	if !strings.Contains(instr, "gaia/verifier/testproject") {
		t.Error("SaveInstructions should contain subagent prefix")
	}
	// Should contain the shared prefix
	if !strings.Contains(instr, "gaia/shared/testproject") {
		t.Error("SaveInstructions should contain shared prefix")
	}
	// Should mention mem_save
	if !strings.Contains(instr, "mem_save") {
		t.Error("SaveInstructions should mention mem_save")
	}
	// Should mention read-only for shared
	if !strings.Contains(strings.ToLower(instr), "must not write") {
		t.Error("SaveInstructions should mention MUST NOT write for shared namespace")
	}
}

// TestSearchInstructions verifies search instructions contain namespace info.
func TestSearchInstructions(t *testing.T) {
	mgr := NewNamespaceManager("proj")

	instr := mgr.SearchInstructions("explorer")

	if !strings.Contains(instr, "gaia/explorer/proj") {
		t.Error("SearchInstructions should contain subagent prefix")
	}
	if !strings.Contains(instr, "gaia/shared/proj") {
		t.Error("SearchInstructions should contain shared prefix")
	}
	if !strings.Contains(instr, "mem_get_observation") {
		t.Error("SearchInstructions should mention mem_get_observation")
	}
}

// TestSubagentNamespacesDoNotCollide verifies different subagents
// get different namespaces.
func TestSubagentNamespacesDoNotCollide(t *testing.T) {
	mgr := NewNamespaceManager("proj")

	explorer := mgr.SubagentPrefix("explorer")
	proposer := mgr.SubagentPrefix("proposer")

	if explorer == proposer {
		t.Error("different subagents should have different namespace prefixes")
	}
}

// TestProject verifies GetProject returns the correct value.
func TestProject(t *testing.T) {
	mgr := NewNamespaceManager("gaia")
	if mgr.Project() != "gaia" {
		t.Errorf("Project: want 'gaia', got %q", mgr.Project())
	}
}

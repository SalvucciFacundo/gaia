package memory

import (
	"fmt"
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

// TestNamespaceManager_SessionID verifies session identity management.
func TestNamespaceManager_SessionID(t *testing.T) {
	mgr := NewNamespaceManager("gaia")
	if mgr.SessionID() != "" {
		t.Errorf("expected empty initial session ID, got %q", mgr.SessionID())
	}

	mgr.SetSessionID("sess-12345")
	if mgr.SessionID() != "sess-12345" {
		t.Errorf("expected session ID 'sess-12345', got %q", mgr.SessionID())
	}

	instr := mgr.SaveInstructions("explorer")
	if !strings.Contains(instr, "sess-12345") {
		t.Error("SaveInstructions should contain active session_id when configured")
	}
	if !strings.Contains(instr, "capture_prompt: false") {
		t.Error("SaveInstructions should advise capture_prompt: false for automated artifacts")
	}
	if !strings.Contains(instr, "needs_review") {
		t.Error("SaveInstructions should mention needs_review lifecycle state")
	}
}

func TestNamespaceManager_ProfilePrefix(t *testing.T) {
	mgr := NewNamespaceManager("gaia")

	want := "gaia/profile/dev/gaia"
	got := mgr.ProfilePrefix("dev")
	if got != want {
		t.Errorf("ProfilePrefix: want %q, got %q", want, got)
	}

	wantKey := "gaia/profile/art/gaia/palette"
	gotKey := mgr.ProfileTopicKey("art", "palette")
	if gotKey != wantKey {
		t.Errorf("ProfileTopicKey: want %q, got %q", wantKey, gotKey)
	}
}

func TestNamespaceManager_ParseTopicKey(t *testing.T) {
	mgr := NewNamespaceManager("gaia")

	// Profile topic key
	info, ok := mgr.ParseTopicKey("gaia/profile/dev/gaia/patterns/code")
	if !ok || info.Domain != "profile" || info.Owner != "dev" || info.Project != "gaia" || info.Topic != "patterns/code" {
		t.Errorf("ParseTopicKey profile failed: got %+v, ok=%v", info, ok)
	}

	// Shared topic key
	info, ok = mgr.ParseTopicKey("gaia/shared/gaia/global-decisions")
	if !ok || info.Domain != "shared" || info.Owner != "shared" || info.Topic != "global-decisions" {
		t.Errorf("ParseTopicKey shared failed: got %+v, ok=%v", info, ok)
	}

	// Subagent topic key
	info, ok = mgr.ParseTopicKey("gaia/subagent/explorer/gaia/discovery")
	if !ok || info.Domain != "subagent" || info.Owner != "explorer" || info.Topic != "discovery" {
		t.Errorf("ParseTopicKey subagent failed: got %+v, ok=%v", info, ok)
	}

	// Invalid key
	_, ok = mgr.ParseTopicKey("other/invalid/key")
	if ok {
		t.Error("ParseTopicKey should fail on non-gaia keys")
	}
}

func TestNamespaceManager_ValidateMemoryAccess(t *testing.T) {
	mgr := NewNamespaceManager("gaia")

	// 1. Cross-profile read: always allowed without human approval
	allowed, needApproval, _ := mgr.ValidateMemoryAccess("dev", "gaia/profile/art/gaia/color-scheme", false)
	if !allowed || needApproval {
		t.Errorf("cross-profile read should be allowed without approval, got allowed=%v, needApproval=%v", allowed, needApproval)
	}

	// 2. Self write: dev writing to dev -> allowed, no approval needed
	allowed, needApproval, _ = mgr.ValidateMemoryAccess("dev", "gaia/profile/dev/gaia/standards", true)
	if !allowed || needApproval {
		t.Errorf("self-write should be allowed without approval, got allowed=%v, needApproval=%v", allowed, needApproval)
	}

	// 3. Cross-profile write: dev writing to art -> not allowed directly, requires human approval
	allowed, needApproval, reason := mgr.ValidateMemoryAccess("dev", "gaia/profile/art/gaia/color-scheme", true)
	if allowed || !needApproval {
		t.Errorf("cross-profile write should require approval, got allowed=%v, needApproval=%v", allowed, needApproval)
	}
	if !strings.Contains(reason, "cross-profile") {
		t.Errorf("expected reason to mention cross-profile, got %q", reason)
	}

	// 4. Shared write: profile writing to shared -> requires human approval
	allowed, needApproval, reason = mgr.ValidateMemoryAccess("dev", "gaia/shared/gaia/rules", true)
	if allowed || !needApproval {
		t.Errorf("shared write should require approval, got allowed=%v, needApproval=%v", allowed, needApproval)
	}
	if !strings.Contains(reason, "shared namespace") {
		t.Errorf("expected reason to mention shared namespace, got %q", reason)
	}
}

func TestNamespaceManager_ProfileInstructions(t *testing.T) {
	mgr := NewNamespaceManager("myproj")
	mgr.SetSessionID("sess-999")

	saveInstr := mgr.ProfileSaveInstructions("dev")
	if !strings.Contains(saveInstr, "gaia/profile/dev/myproj") {
		t.Error("ProfileSaveInstructions should contain profile prefix")
	}
	if !strings.Contains(saveInstr, "Cross-Profile Collaboration") {
		t.Error("ProfileSaveInstructions should describe cross-profile reading")
	}
	if !strings.Contains(saveInstr, "Mutation Restriction") {
		t.Error("ProfileSaveInstructions should state mutation restriction")
	}

	searchInstr := mgr.ProfileSearchInstructions("dev")
	if !strings.Contains(searchInstr, "gaia/profile/dev/myproj") {
		t.Error("ProfileSearchInstructions should contain profile scope")
	}
}

func TestNamespaceManager_ConcurrentSafe(t *testing.T) {
	mgr := NewNamespaceManager("concurrent-proj")

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				mgr.SetSessionID(fmt.Sprintf("sess-%d-%d", id, j))
				_ = mgr.SessionID()
				_ = mgr.SaveInstructions("explorer")
				_ = mgr.DynamicSaveInstructions("dyn")
				_ = mgr.ProfileSaveInstructions("dev")
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}


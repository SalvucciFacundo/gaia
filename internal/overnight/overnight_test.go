package overnight

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreflight_HeuristicsAndPersistence(t *testing.T) {
	tempDir := t.TempDir()
	interviewer := NewPreflightInterviewer(nil, tempDir)

	ctx := context.Background()
	questions, err := interviewer.AnalyzeGoal(ctx, "Refactor database migrations and auth modules", "dev")
	if err != nil {
		t.Fatalf("AnalyzeGoal failed: %v", err)
	}

	if len(questions) < 2 {
		t.Errorf("expected at least 2 questions for refactoring goal, got %d", len(questions))
	}

	plan := &ActionPlan{
		ID:         "test-plan-1",
		Goal:       "Refactor database migrations and auth modules",
		Profile:    "dev",
		Questions:  questions,
		Status:     PlanStatusPlanning,
		Directives: []string{"Preserve backwards compatibility"},
	}

	if err := interviewer.SavePlan(plan); err != nil {
		t.Fatalf("SavePlan failed: %v", err)
	}

	loaded, err := interviewer.LoadPlan("test-plan-1")
	if err != nil {
		t.Fatalf("LoadPlan failed: %v", err)
	}

	if loaded.ID != plan.ID || loaded.Goal != plan.Goal {
		t.Errorf("loaded plan mismatch: got %+v, want %+v", loaded, plan)
	}
}

func TestDeferredQueue_Lifecycle(t *testing.T) {
	queue := NewDeferredQueue()

	act1 := queue.Enqueue("plan-42", "file_delete", map[string]interface{}{"path": "/tmp/test.go"}, "Destructive deletion")
	if act1.ID == "" || act1.Status != "pending" {
		t.Fatalf("unexpected action: %+v", act1)
	}

	list := queue.List("plan-42")
	if len(list) != 1 || list[0].ID != act1.ID {
		t.Fatalf("expected 1 action in list, got %+v", list)
	}

	if err := queue.UpdateStatus("plan-42", act1.ID, "approved"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	updated := queue.List("plan-42")
	if updated[0].Status != "approved" {
		t.Errorf("expected status approved, got %s", updated[0].Status)
	}

	queue.Clear("plan-42")
	if len(queue.List("plan-42")) != 0 {
		t.Errorf("expected empty list after clear")
	}
}

func TestRunner_StagingSandboxAndReviewPack(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewRunner(nil, nil, tempDir)

	plan := &ActionPlan{
		ID:      "plan-staging-1",
		Goal:    "Generate market analysis report",
		Profile: "assistant",
	}

	ctx := context.Background()
	// Non-git directory forces Staging sandbox
	nonGitDir := filepath.Join(tempDir, "non_git_workspace")
	_ = os.MkdirAll(nonGitDir, 0755)

	if err := runner.ProvisionSandbox(ctx, plan, nonGitDir); err != nil {
		t.Fatalf("ProvisionSandbox failed: %v", err)
	}

	if plan.SandboxMode != SandboxStaging {
		t.Errorf("expected SandboxStaging, got %v", plan.SandboxMode)
	}
	if !strings.Contains(plan.SandboxPath, "staging") {
		t.Errorf("expected staging path, got %s", plan.SandboxPath)
	}

	// Create a dummy generated file in staging
	testFile := filepath.Join(plan.SandboxPath, "report.md")
	_ = os.WriteFile(testFile, []byte("# Market Analysis"), 0644)

	pack, err := runner.BuildReviewPack(ctx, plan)
	if err != nil {
		t.Fatalf("BuildReviewPack failed: %v", err)
	}

	if len(pack.GeneratedFiles) != 1 || pack.GeneratedFiles[0] != "report.md" {
		t.Errorf("unexpected generated files in pack: %+v", pack.GeneratedFiles)
	}

	// Discard should clean up
	if err := runner.Discard(ctx, plan, nonGitDir); err != nil {
		t.Fatalf("Discard failed: %v", err)
	}

	if plan.Status != PlanStatusDiscarded {
		t.Errorf("expected PlanStatusDiscarded, got %v", plan.Status)
	}
	if _, err := os.Stat(plan.SandboxPath); !os.IsNotExist(err) {
		t.Errorf("expected staging path to be removed")
	}
}

func TestOvernight_ContextCancelled(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewRunner(nil, nil, tempDir)
	interviewer := NewPreflightInterviewer(nil, tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	plan := &ActionPlan{ID: "test-cancel"}

	if _, err := interviewer.AnalyzeGoal(ctx, "Goal", "dev"); err == nil {
		t.Error("expected error for cancelled context in AnalyzeGoal")
	}

	if err := runner.ProvisionSandbox(ctx, plan, tempDir); err == nil {
		t.Error("expected error for cancelled context in ProvisionSandbox")
	}

	if _, err := runner.BuildReviewPack(ctx, plan); err == nil {
		t.Error("expected error for cancelled context in BuildReviewPack")
	}

	if err := runner.Merge(ctx, plan, tempDir); err == nil {
		t.Error("expected error for cancelled context in Merge")
	}

	if err := runner.Discard(ctx, plan, tempDir); err == nil {
		t.Error("expected error for cancelled context in Discard")
	}
}

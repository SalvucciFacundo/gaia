package gates

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gaia/internal/core/domain"
)

// setupTestGitRepo initializes a temporary Git repository for testing.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
	}

	run("init")
	run("config", "user.email", "test@gaia.agent")
	run("config", "user.name", "GAIA Test")
	run("config", "commit.gpgsign", "false")

	// Create initial commit
	readme := filepath.Join(dir, "README.md")
	_ = os.WriteFile(readme, []byte("# Test Repo\n"), 0644)
	run("add", "README.md")
	run("commit", "-m", "init")

	return dir
}

func TestCASReceiptStore_GitRepo_SaveAndRead(t *testing.T) {
	gitDir := setupTestGitRepo(t)
	store := NewCASReceiptStore(gitDir)

	if !store.isGitRepo() {
		t.Fatal("expected store to identify git repository")
	}

	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "lin-cas-001",
		SnapshotHash: "sha256:abcd1234efgh5678",
		State:        domain.ReviewStateApproved,
		RiskLevel:    "medium",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	changeName := "feature-auth"

	// 1. Save Receipt
	err := store.SaveReceipt(receipt, changeName)
	if err != nil {
		t.Fatalf("SaveReceipt failed: %v", err)
	}

	// 2. Read back via LatestReceipt
	loaded, err := store.LatestReceipt(changeName)
	if err != nil {
		t.Fatalf("LatestReceipt failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil receipt")
	}

	if loaded.LineageID != receipt.LineageID {
		t.Errorf("expected lineage %q, got %q", receipt.LineageID, loaded.LineageID)
	}
	if loaded.SnapshotHash != receipt.SnapshotHash {
		t.Errorf("expected snapshot hash %q, got %q", receipt.SnapshotHash, loaded.SnapshotHash)
	}
	if loaded.State != domain.ReviewStateApproved {
		t.Errorf("expected state approved, got %q", loaded.State)
	}

	// 3. List receipts
	summaries, err := store.ListReceipts()
	if err != nil {
		t.Fatalf("ListReceipts failed: %v", err)
	}
	if len(summaries) == 0 {
		t.Fatal("expected at least 1 receipt summary")
	}
}

func TestCASReceiptStore_NonGitRepo_Fallback(t *testing.T) {
	nonGitDir := t.TempDir()
	store := NewCASReceiptStore(nonGitDir)

	if store.isGitRepo() {
		t.Fatal("expected non-git dir to return false for isGitRepo")
	}

	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "lin-nongit-002",
		SnapshotHash: "sha256:1111222233334444",
		State:        domain.ReviewStateApproved,
		RiskLevel:    "low",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	changeName := "feature-docs"

	// 1. Save should succeed using filesystem fallback
	err := store.SaveReceipt(receipt, changeName)
	if err != nil {
		t.Fatalf("SaveReceipt on non-git dir failed: %v", err)
	}

	// 2. Read back from filesystem
	loaded, err := store.LatestReceipt(changeName)
	if err != nil {
		t.Fatalf("LatestReceipt on non-git dir failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil receipt from fallback")
	}

	if loaded.LineageID != receipt.LineageID {
		t.Errorf("expected lineage %q, got %q", receipt.LineageID, loaded.LineageID)
	}
}

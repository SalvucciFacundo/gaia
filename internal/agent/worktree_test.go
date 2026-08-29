package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTempGitRepo initializes an empty Git repo with an initial commit for testing.
func setupTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@gaia.agent")
	runGit("config", "user.name", "GAIA Test")
	runGit("config", "commit.gpgsign", "false")

	// Create initial commit
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Initial Repo\n"), 0644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	return dir
}

func TestWorktreeManager_Lifecycle(t *testing.T) {
	repoDir := setupTempGitRepo(t)
	wm := NewWorktreeManager()
	ctx := context.Background()

	// 1. Verify IsGitRepo
	if !wm.IsGitRepo(ctx, repoDir) {
		t.Fatal("expected repoDir to be identified as a git repository")
	}

	taskID := "task-iso-001"

	// 2. Create Worktree
	wt, err := wm.Create(ctx, repoDir, taskID)
	if err != nil {
		t.Fatalf("Create worktree failed: %v", err)
	}

	if wt.WorktreeDir == "" || wt.BranchName != "gaia-wt/"+taskID {
		t.Errorf("unexpected worktree context: %+v", wt)
	}

	if _, err := os.Stat(wt.WorktreeDir); os.IsNotExist(err) {
		t.Fatalf("expected worktree directory %q to exist", wt.WorktreeDir)
	}

	// 3. Make a change in the isolated worktree
	newFilePath := filepath.Join(wt.WorktreeDir, "isolated_feature.go")
	if err := os.WriteFile(newFilePath, []byte("package feature\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatalf("write isolated file: %v", err)
	}

	// 4. Verify GetDiff captures the modification
	diff, err := wm.GetDiff(ctx, wt)
	if err != nil {
		t.Fatalf("GetDiff failed: %v", err)
	}

	if !strings.Contains(diff, "isolated_feature.go") {
		t.Errorf("expected diff to contain 'isolated_feature.go', got:\n%s", diff)
	}

	// Verify main repo was NOT touched
	mainFileCheck := filepath.Join(repoDir, "isolated_feature.go")
	if _, err := os.Stat(mainFileCheck); !os.IsNotExist(err) {
		t.Error("expected main repo to be untouched, but isolated_feature.go was found!")
	}

	// 5. Remove Worktree
	if err := wm.Remove(ctx, wt); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := os.Stat(wt.WorktreeDir); !os.IsNotExist(err) {
		t.Errorf("expected worktree dir %q to be deleted after Remove", wt.WorktreeDir)
	}
}

func TestWorktreeManager_NonGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	wm := NewWorktreeManager()
	ctx := context.Background()

	if wm.IsGitRepo(ctx, tmpDir) {
		t.Error("expected non-git dir to return false for IsGitRepo")
	}

	_, err := wm.Create(ctx, tmpDir, "task-error")
	if err == nil {
		t.Error("expected error when creating worktree in non-git directory, got nil")
	}
}

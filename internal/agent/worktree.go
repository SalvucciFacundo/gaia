package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// WorktreeContext holds paths and metadata for an isolated Git worktree.
type WorktreeContext struct {
	TaskID      string    `json:"task_id"`
	BasePath    string    `json:"base_path"`
	WorktreeDir string    `json:"worktree_dir"`
	BranchName  string    `json:"branch_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// WorktreeManager provisions, manages, and destroys ephemeral git worktrees.
type WorktreeManager struct {
	mu sync.Mutex
}

// NewWorktreeManager creates a new WorktreeManager.
func NewWorktreeManager() *WorktreeManager {
	return &WorktreeManager{}
}

// IsGitRepo returns true if the specified directory is inside a valid Git repository.
func (wm *WorktreeManager) IsGitRepo(ctx context.Context, dir string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// Create provisions an isolated git worktree and branch for the given task.
func (wm *WorktreeManager) Create(ctx context.Context, baseRepoPath, taskID string) (*WorktreeContext, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if baseRepoPath == "" {
		baseRepoPath = "."
	}
	absBase, err := filepath.Abs(baseRepoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve base repo path: %w", err)
	}

	if !wm.IsGitRepo(ctx, absBase) {
		return nil, fmt.Errorf("directory %q is not a valid git repository", absBase)
	}

	worktreesBase := filepath.Join(absBase, ".gaia", "worktrees")
	if err := os.MkdirAll(worktreesBase, 0755); err != nil {
		return nil, fmt.Errorf("create worktrees base dir: %w", err)
	}

	worktreeDir := filepath.Join(worktreesBase, taskID)
	branchName := fmt.Sprintf("gaia-wt/%s", taskID)

	// Clean up any stale directory at the target path
	_ = os.RemoveAll(worktreeDir)

	// Execute: git worktree add -b <branchName> <worktreeDir> HEAD
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, worktreeDir, "HEAD")
	cmd.Dir = absBase
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git worktree add failed: %s (%w)", stderr.String(), err)
	}

	return &WorktreeContext{
		TaskID:      taskID,
		BasePath:    absBase,
		WorktreeDir: worktreeDir,
		BranchName:  branchName,
		CreatedAt:   time.Now(),
	}, nil
}

// GetDiff returns a unified diff of all changes made in the isolated worktree relative to HEAD.
func (wm *WorktreeManager) GetDiff(ctx context.Context, wt *WorktreeContext) (string, error) {
	if wt == nil || wt.WorktreeDir == "" {
		return "", fmt.Errorf("invalid worktree context")
	}

	// 1. Stage untracked files temporarily or get standard git diff
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = wt.WorktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	// 2. Check for untracked files
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = wt.WorktreeDir
	statusOut, _ := statusCmd.Output()

	var sb strings.Builder
	sb.Write(out)

	if len(statusOut) > 0 {
		untrackedLines := strings.Split(string(statusOut), "\n")
		for _, l := range untrackedLines {
			if strings.HasPrefix(l, "?? ") {
				relPath := strings.TrimSpace(l[3:])
				fullPath := filepath.Join(wt.WorktreeDir, relPath)
				data, readErr := os.ReadFile(fullPath)
				if readErr == nil {
					sb.WriteString(fmt.Sprintf("\n--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", relPath, len(strings.Split(string(data), "\n"))))
					for _, line := range strings.Split(string(data), "\n") {
						sb.WriteString("+" + line + "\n")
					}
				}
			}
		}
	}

	return sb.String(), nil
}

// Remove detaches and deletes the isolated worktree and its temporary branch.
func (wm *WorktreeManager) Remove(ctx context.Context, wt *WorktreeContext) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wt == nil {
		return nil
	}

	// 1. git worktree remove --force <worktreeDir>
	removeCmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", wt.WorktreeDir)
	removeCmd.Dir = wt.BasePath
	_ = removeCmd.Run()

	// 2. Delete temporary branch: git branch -D <branchName>
	branchCmd := exec.CommandContext(ctx, "git", "branch", "-D", wt.BranchName)
	branchCmd.Dir = wt.BasePath
	_ = branchCmd.Run()

	// 3. Fallback filesystem cleanup
	_ = os.RemoveAll(wt.WorktreeDir)

	return nil
}

// Prune cleans stale worktree references from Git's administrative files.
func (wm *WorktreeManager) Prune(ctx context.Context, baseRepoPath string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = baseRepoPath
	return cmd.Run()
}

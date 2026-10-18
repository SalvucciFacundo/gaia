package overnight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gaia/internal/agent"
)

// Runner manages the sandboxed execution and review pack generation for overnight plans.
type Runner struct {
	worktreeMgr *agent.WorktreeManager
	queue       *DeferredQueue
	storage     string
}

// NewRunner creates a new overnight execution runner.
func NewRunner(wtMgr *agent.WorktreeManager, queue *DeferredQueue, storageDir string) *Runner {
	if wtMgr == nil {
		wtMgr = agent.NewWorktreeManager()
	}
	if queue == nil {
		queue = NewDeferredQueue()
	}
	if storageDir == "" {
		storageDir = ".gaia/overnight"
	}
	_ = os.MkdirAll(storageDir, 0755)

	return &Runner{
		worktreeMgr: wtMgr,
		queue:       queue,
		storage:     storageDir,
	}
}

// ProvisionSandbox creates an isolated git worktree or filesystem staging directory.
func (r *Runner) ProvisionSandbox(ctx context.Context, plan *ActionPlan, baseDir string) error {
	if baseDir == "" {
		baseDir = "."
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("resolve base path: %w", err)
	}

	if r.worktreeMgr.IsGitRepo(ctx, absBase) {
		wt, err := r.worktreeMgr.Create(ctx, absBase, plan.ID)
		if err != nil {
			return fmt.Errorf("provision git worktree: %w", err)
		}
		plan.SandboxMode = SandboxGitWorktree
		plan.SandboxPath = wt.WorktreeDir
		plan.BranchName = wt.BranchName
	} else {
		stagingDir := filepath.Join(absBase, ".gaia", "staging", plan.ID)
		if err := os.MkdirAll(stagingDir, 0755); err != nil {
			return fmt.Errorf("provision staging dir: %w", err)
		}
		plan.SandboxMode = SandboxStaging
		plan.SandboxPath = stagingDir
	}

	plan.Status = PlanStatusReady
	return nil
}

// BuildReviewPack captures diffs, test results, and deferred actions into a consolidated pack.
func (r *Runner) BuildReviewPack(ctx context.Context, plan *ActionPlan) (*ReviewPack, error) {
	pack := &ReviewPack{
		PlanID:          plan.ID,
		Status:          plan.Status,
		Goal:            plan.Goal,
		DeferredActions: r.queue.List(plan.ID),
		CreatedAt:       time.Now(),
	}

	if plan.SandboxMode == SandboxGitWorktree && plan.SandboxPath != "" {
		wt := &agent.WorktreeContext{
			TaskID:      plan.ID,
			WorktreeDir: plan.SandboxPath,
			BranchName:  plan.BranchName,
		}
		diff, err := r.worktreeMgr.GetDiff(ctx, wt)
		if err == nil {
			pack.Diff = diff
		}

		// Run quick verification test in worktree if it has go.mod
		if _, err := os.Stat(filepath.Join(plan.SandboxPath, "go.mod")); err == nil {
			cmd := exec.CommandContext(ctx, "go", "test", "./...")
			cmd.Dir = plan.SandboxPath
			out, err := cmd.CombinedOutput()
			if err != nil {
				pack.TestResults = fmt.Sprintf("FAILED:\n%s", string(out))
			} else {
				pack.TestResults = fmt.Sprintf("PASSED:\n%s", string(out))
			}
		}
	} else if plan.SandboxPath != "" {
		// List generated files in staging
		var files []string
		_ = filepath.Walk(plan.SandboxPath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				rel, _ := filepath.Rel(plan.SandboxPath, path)
				files = append(files, rel)
			}
			return nil
		})
		pack.GeneratedFiles = files
	}

	pack.Summary = fmt.Sprintf("Autonomous execution finished for plan %q. Deferred actions: %d.", plan.ID, len(pack.DeferredActions))

	// Save review pack to disk
	packPath := filepath.Join(r.storage, fmt.Sprintf("%s-review.json", plan.ID))
	data, _ := json.MarshalIndent(pack, "", "  ")
	_ = os.WriteFile(packPath, data, 0644)

	return pack, nil
}

// Merge merges worktree branch into base HEAD and removes the worktree.
func (r *Runner) Merge(ctx context.Context, plan *ActionPlan, baseRepoPath string) error {
	if plan.SandboxMode != SandboxGitWorktree {
		return fmt.Errorf("plan %q is not using a git worktree", plan.ID)
	}

	if baseRepoPath == "" {
		baseRepoPath = "."
	}
	absBase, _ := filepath.Abs(baseRepoPath)

	// Execute: git merge --no-ff <branchName> -m "feat(overnight): merge plan <ID>"
	mergeMsg := fmt.Sprintf("feat(overnight): merge plan %s - %s", plan.ID, plan.Goal)
	cmd := exec.CommandContext(ctx, "git", "merge", "--no-ff", plan.BranchName, "-m", mergeMsg)
	cmd.Dir = absBase
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git merge failed: %s (%w)", string(out), err)
	}

	// Clean up worktree
	wt := &agent.WorktreeContext{
		TaskID:      plan.ID,
		BasePath:    absBase,
		WorktreeDir: plan.SandboxPath,
		BranchName:  plan.BranchName,
	}
	_ = r.worktreeMgr.Remove(ctx, wt)

	plan.Status = PlanStatusMerged
	now := time.Now()
	plan.CompletedAt = &now
	return nil
}

// Discard cleans up the worktree or staging directory and marks plan discarded.
func (r *Runner) Discard(ctx context.Context, plan *ActionPlan, baseRepoPath string) error {
	if plan.SandboxMode == SandboxGitWorktree {
		if baseRepoPath == "" {
			baseRepoPath = "."
		}
		absBase, _ := filepath.Abs(baseRepoPath)
		wt := &agent.WorktreeContext{
			TaskID:      plan.ID,
			BasePath:    absBase,
			WorktreeDir: plan.SandboxPath,
			BranchName:  plan.BranchName,
		}
		_ = r.worktreeMgr.Remove(ctx, wt)

		// Delete branch
		cmd := exec.CommandContext(ctx, "git", "branch", "-D", plan.BranchName)
		cmd.Dir = absBase
		_ = cmd.Run()
	} else if plan.SandboxPath != "" {
		_ = os.RemoveAll(plan.SandboxPath)
	}

	r.queue.Clear(plan.ID)
	plan.Status = PlanStatusDiscarded
	return nil
}

// LoadReviewPack loads a saved review pack by plan ID.
func (r *Runner) LoadReviewPack(planID string) (*ReviewPack, error) {
	packPath := filepath.Join(r.storage, fmt.Sprintf("%s-review.json", planID))
	data, err := os.ReadFile(packPath)
	if err != nil {
		return nil, fmt.Errorf("read review pack: %w", err)
	}

	var pack ReviewPack
	if err := json.Unmarshal(data, &pack); err != nil {
		return nil, fmt.Errorf("unmarshal review pack: %w", err)
	}

	return &pack, nil
}

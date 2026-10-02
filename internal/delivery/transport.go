package delivery

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Transport defines the operations needed to push code and open pull requests remotely.
type Transport interface {
	PushBranch(ctx context.Context, branch string) error
	CreatePullRequest(ctx context.Context, item DeliveryItem) (string, error)
}

// ExecTransport executes remote git and GitHub CLI operations via external subprocesses.
type ExecTransport struct {
	workDir string
}

// NewExecTransport creates a new ExecTransport rooted in the given work directory.
func NewExecTransport(workDir string) *ExecTransport {
	return &ExecTransport{
		workDir: workDir,
	}
}

// WorkDir returns the configured working directory for subprocesses.
func (t *ExecTransport) WorkDir() string {
	return t.workDir
}

// PushBranch pushes the specified local branch to the origin remote.
func (t *ExecTransport) PushBranch(ctx context.Context, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", branch)
	if t.workDir != "" {
		cmd.Dir = t.workDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CreatePullRequest opens a pull request on GitHub using the gh CLI.
func (t *ExecTransport) CreatePullRequest(ctx context.Context, item DeliveryItem) (string, error) {
	args := []string{
		"pr", "create",
		"--head", item.Branch,
		"--title", item.PRTitle,
		"--body", item.PRBody,
	}
	if item.BaseBranch != "" {
		args = append(args, "--base", item.BaseBranch)
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	if t.workDir != "" {
		cmd.Dir = t.workDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// MockTransport provides an in-memory transport for testing without network/subprocess side effects.
type MockTransport struct {
	mu             sync.Mutex
	PushedBranches []string
	CreatedPRs     []DeliveryItem
	PushBranchFunc func(ctx context.Context, branch string) error
	CreatePRFunc   func(ctx context.Context, item DeliveryItem) (string, error)
	DefaultPRURL   string
	PushErr        error
	CreatePRErr    error
}

// NewMockTransport creates a new MockTransport with sensible defaults.
func NewMockTransport() *MockTransport {
	return &MockTransport{
		DefaultPRURL: "https://github.com/example/repo/pull/1",
	}
}

// PushBranch records the pushed branch name and returns any configured error.
func (m *MockTransport) PushBranch(ctx context.Context, branch string) error {
	m.mu.Lock()
	m.PushedBranches = append(m.PushedBranches, branch)
	pushFunc := m.PushBranchFunc
	pushErr := m.PushErr
	m.mu.Unlock()

	if pushFunc != nil {
		return pushFunc(ctx, branch)
	}
	return pushErr
}

// CreatePullRequest records the pull request details and returns a PR URL or configured error.
func (m *MockTransport) CreatePullRequest(ctx context.Context, item DeliveryItem) (string, error) {
	m.mu.Lock()
	m.CreatedPRs = append(m.CreatedPRs, item)
	createFunc := m.CreatePRFunc
	createErr := m.CreatePRErr
	defaultURL := m.DefaultPRURL
	count := len(m.CreatedPRs)
	m.mu.Unlock()

	if createFunc != nil {
		return createFunc(ctx, item)
	}
	if createErr != nil {
		return "", createErr
	}
	if defaultURL == "" {
		defaultURL = fmt.Sprintf("https://github.com/example/repo/pull/%d", count)
	}
	return defaultURL, nil
}

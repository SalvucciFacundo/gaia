package shell

import (
	"context"
	"os/exec"
)

// CommandExecutor abstracts command execution across local, Docker, and SSH backends.
type CommandExecutor interface {
	// Exec runs a command with the given arguments in the specified directory.
	// Returns combined stdout+stderr output and any error (including non-zero exit codes).
	Exec(ctx context.Context, cmd string, args []string, dir string) (string, error)

	// Name returns the backend identifier ("local", "docker", or "ssh").
	Name() string
}

// LocalExecutor runs commands directly on the host via os/exec.
type LocalExecutor struct{}

// Name returns "local".
func (e *LocalExecutor) Name() string { return "local" }

// Exec runs the command on the local host using exec.CommandContext.
func (e *LocalExecutor) Exec(ctx context.Context, cmd string, args []string, dir string) (string, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	output, err := c.CombinedOutput()
	return string(output), err
}

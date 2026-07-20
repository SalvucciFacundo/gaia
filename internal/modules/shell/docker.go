package shell

import (
	"context"
	"fmt"
	"os/exec"
)

// DockerExecutor runs commands inside a Docker container via "docker exec".
type DockerExecutor struct {
	container string
	workDir   string
}

// NewDockerExecutor creates a DockerExecutor targeting the given container.
func NewDockerExecutor(container, workDir string) *DockerExecutor {
	return &DockerExecutor{container: container, workDir: workDir}
}

// Name returns "docker".
func (e *DockerExecutor) Name() string { return "docker" }

// Exec runs the command inside the configured container via "docker exec -i".
func (e *DockerExecutor) Exec(ctx context.Context, cmd string, args []string, dir string) (string, error) {
	if e.container == "" {
		return "", fmt.Errorf("docker container not configured")
	}

	// Build: docker exec -i <container> <cmd> <args...>
	execArgs := make([]string, 0, 4+len(args))
	execArgs = append(execArgs, "exec", "-i", e.container, cmd)
	execArgs = append(execArgs, args...)

	c := exec.CommandContext(ctx, "docker", execArgs...)
	if e.workDir != "" {
		c.Dir = e.workDir
	} else {
		c.Dir = dir
	}
	output, err := c.CombinedOutput()
	return string(output), err
}

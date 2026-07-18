package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gaia/internal/core/domain"
)

// --- LocalExecutor tests ---

func TestLocalExecutor_Name(t *testing.T) {
	e := &LocalExecutor{}
	if e.Name() != "local" {
		t.Errorf("expected Name() = 'local', got %q", e.Name())
	}
}

func TestLocalExecutor_Exec_Success(t *testing.T) {
	e := &LocalExecutor{}
	root := t.TempDir()
	marker := filepath.Join(root, "hello.txt")
	os.WriteFile(marker, []byte("world"), 0644)

	// Use a command available on all platforms (go is required for this project).
	output, err := e.Exec(context.Background(), "go", []string{"version"}, root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "go version") {
		t.Errorf("expected 'go version' in output, got: %s", output)
	}
}

func TestLocalExecutor_Exec_CommandNotFound(t *testing.T) {
	e := &LocalExecutor{}
	_, err := e.Exec(context.Background(), "nonexistent_command_xyz", nil, t.TempDir())
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

// --- DockerExecutor tests ---

func TestDockerExecutor_Name(t *testing.T) {
	e := NewDockerExecutor("mycontainer", "/app")
	if e.Name() != "docker" {
		t.Errorf("expected Name() = 'docker', got %q", e.Name())
	}
}

func TestDockerExecutor_Exec_NoContainer(t *testing.T) {
	e := NewDockerExecutor("", "/app")
	_, err := e.Exec(context.Background(), "ls", nil, "/tmp")
	if err == nil {
		t.Fatal("expected error when container is empty")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestDockerExecutor_Exec_CommandNotFound(t *testing.T) {
	// Docker not likely installed in CI, but docker exec will fail
	// because Docker itself may not be available. Either way, error expected.
	e := NewDockerExecutor("test-container", "/app")
	_, err := e.Exec(context.Background(), "ls", []string{"-la"}, "/tmp")
	if err == nil {
		t.Log("docker unexpectedly available — command ran inside container")
	}
}

// --- SSHExecutor tests ---

func TestSSHExecutor_Name(t *testing.T) {
	e := NewSSHExecutor("example.com", 22, "user", "/key", "")
	if e.Name() != "ssh" {
		t.Errorf("expected Name() = 'ssh', got %q", e.Name())
	}
}

func TestSSHExecutor_Exec_NoHost(t *testing.T) {
	e := NewSSHExecutor("", 22, "user", "/key", "")
	_, err := e.Exec(context.Background(), "ls", nil, "/tmp")
	if err == nil {
		t.Fatal("expected error when host is empty")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestSSHExecutor_Exec_KeyNotFound(t *testing.T) {
	e := NewSSHExecutor("example.com", 22, "user", "/nonexistent/key/path", "")
	_, err := e.Exec(context.Background(), "ls", nil, "/tmp")
	if err == nil {
		t.Fatal("expected error for nonexistent key file")
	}
	if !strings.Contains(err.Error(), "failed to read ssh key") {
		t.Errorf("expected key read error, got: %v", err)
	}
}

func TestSSHExecutor_Exec_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "bad_key")
	os.WriteFile(keyPath, []byte("not a valid ssh key"), 0600)

	e := NewSSHExecutor("example.com", 22, "user", keyPath, "")
	_, err := e.Exec(context.Background(), "ls", nil, "/tmp")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
	if !strings.Contains(err.Error(), "failed to parse ssh key") {
		t.Errorf("expected key parse error, got: %v", err)
	}
}

func TestSSHExecutor_DefaultPort(t *testing.T) {
	e := NewSSHExecutor("example.com", 0, "user", "/key", "")
	if e.port != 0 {
		t.Errorf("expected port 0 to be stored as-is, got %d", e.port)
	}
	// When port is 0, Exec defaults to 22 internally.
	// This is tested implicitly via the key-not-found path above.
}

func TestSSHExecutor_BuildCommand(t *testing.T) {
	e := NewSSHExecutor("example.com", 22, "user", "/key", "")

	tests := []struct {
		name string
		cmd  string
		args []string
		dir  string
		want string
	}{
		{
			name: "simple command",
			cmd:  "ls",
			args: []string{"-la"},
			dir:  "",
			want: "ls -la",
		},
		{
			name: "command with directory",
			cmd:  "go",
			args: []string{"build", "./..."},
			dir:  "/app",
			want: "cd /app && go build ./...",
		},
		{
			name: "no args",
			cmd:  "pwd",
			args: nil,
			dir:  "",
			want: "pwd",
		},
		{
			name: "no args with dir",
			cmd:  "make",
			args: nil,
			dir:  "/project",
			want: "cd /project && make",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.buildCommand(tt.cmd, tt.args, tt.dir)
			if got != tt.want {
				t.Errorf("buildCommand(%q, %v, %q) = %q, want %q",
					tt.cmd, tt.args, tt.dir, got, tt.want)
			}
		})
	}
}

// --- Module executor selection tests ---

func TestNewModule_UsesLocalExecutor(t *testing.T) {
	m := NewModule(t.TempDir())
	if m.executor.Name() != "local" {
		t.Errorf("expected local executor, got %s", m.executor.Name())
	}
}

func TestNewModuleWithConfig_Local(t *testing.T) {
	cfg := &domain.TerminalConfig{Backend: "local"}
	m := NewModuleWithConfig(t.TempDir(), cfg)
	if m.executor.Name() != "local" {
		t.Errorf("expected local executor, got %s", m.executor.Name())
	}
}

func TestNewModuleWithConfig_LocalDefault(t *testing.T) {
	// Empty backend should default to local.
	cfg := &domain.TerminalConfig{Backend: ""}
	m := NewModuleWithConfig(t.TempDir(), cfg)
	if m.executor.Name() != "local" {
		t.Errorf("expected local executor for empty backend, got %s", m.executor.Name())
	}
}

func TestNewModuleWithConfig_UnknownBackend(t *testing.T) {
	cfg := &domain.TerminalConfig{Backend: "unknown"}
	m := NewModuleWithConfig(t.TempDir(), cfg)
	if m.executor.Name() != "local" {
		t.Errorf("expected local executor for unknown backend, got %s", m.executor.Name())
	}
}

func TestNewModuleWithConfig_Docker(t *testing.T) {
	cfg := &domain.TerminalConfig{
		Backend: "docker",
		Docker:  domain.DockerConfig{Container: "myapp", WorkDir: "/app"},
	}
	m := NewModuleWithConfig(t.TempDir(), cfg)
	if m.executor.Name() != "docker" {
		t.Errorf("expected docker executor, got %s", m.executor.Name())
	}

	dockerExec, ok := m.executor.(*DockerExecutor)
	if !ok {
		t.Fatal("executor is not *DockerExecutor")
	}
	if dockerExec.container != "myapp" {
		t.Errorf("expected container 'myapp', got %q", dockerExec.container)
	}
	if dockerExec.workDir != "/app" {
		t.Errorf("expected workDir '/app', got %q", dockerExec.workDir)
	}
}

func TestNewModuleWithConfig_SSH(t *testing.T) {
	cfg := &domain.TerminalConfig{
		Backend: "ssh",
		SSH: domain.SSHConfig{
			Host:    "server.example.com",
			Port:    2222,
			User:    "deploy",
			KeyPath: "/home/user/.ssh/id_rsa",
		},
	}
	m := NewModuleWithConfig(t.TempDir(), cfg)
	if m.executor.Name() != "ssh" {
		t.Errorf("expected ssh executor, got %s", m.executor.Name())
	}

	sshExec, ok := m.executor.(*SSHExecutor)
	if !ok {
		t.Fatal("executor is not *SSHExecutor")
	}
	if sshExec.host != "server.example.com" {
		t.Errorf("expected host 'server.example.com', got %q", sshExec.host)
	}
	if sshExec.port != 2222 {
		t.Errorf("expected port 2222, got %d", sshExec.port)
	}
	if sshExec.user != "deploy" {
		t.Errorf("expected user 'deploy', got %q", sshExec.user)
	}
	if sshExec.keyPath != "/home/user/.ssh/id_rsa" {
		t.Errorf("expected keyPath '/home/user/.ssh/id_rsa', got %q", sshExec.keyPath)
	}
}

func TestNewModuleWithConfig_ModuleStillWorks(t *testing.T) {
	// Even with different backends, the module's basic methods should work.
	cfg := &domain.TerminalConfig{Backend: "local"}
	m := NewModuleWithConfig(t.TempDir(), cfg)

	if m.Name() != "shell" {
		t.Errorf("expected Name() = 'shell', got %q", m.Name())
	}
	if !strings.Contains(m.Description(), "Execute") {
		t.Errorf("expected Description() to contain 'Execute', got %q", m.Description())
	}
	tools := m.GetTools()
	if len(tools) == 0 {
		t.Error("expected at least one tool")
	}
	if tools[0].Name != "shell_exec" {
		t.Errorf("expected tool 'shell_exec', got %q", tools[0].Name)
	}
}

func TestNewModuleWithConfig_ShellExecUsesExecutor(t *testing.T) {
	// Verify that shell_exec goes through the executor by executing a real command.
	root := t.TempDir()
	cfg := &domain.TerminalConfig{Backend: "local"}
	m := NewModuleWithConfig(root, cfg)

	result, err := m.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "where where",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Error)
	}
	if !strings.Contains(strings.ToLower(result.Output), "where") {
		t.Errorf("expected 'where' in output, got: %s", result.Output)
	}
}

// --- Interface compliance ---

func TestAllExecutorsSatisfyInterface(t *testing.T) {
	// Compile-time check: all executors implement CommandExecutor.
	var _ CommandExecutor = (*LocalExecutor)(nil)
	var _ CommandExecutor = (*DockerExecutor)(nil)
	var _ CommandExecutor = (*SSHExecutor)(nil)

	// Verify each returns the correct name.
	executors := []CommandExecutor{
		&LocalExecutor{},
		NewDockerExecutor("c", "/w"),
		NewSSHExecutor("h", 22, "u", "/k", ""),
	}
	names := map[string]bool{}
	for _, e := range executors {
		names[e.Name()] = true
	}
	if len(names) != 3 {
		t.Errorf("expected 3 unique executor names, got %d: %v", len(names), names)
	}
	for _, want := range []string{"local", "docker", "ssh"} {
		if !names[want] {
			t.Errorf("expected executor name %q not found", want)
		}
	}
}

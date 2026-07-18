package shell

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHExecutor runs commands on a remote host via SSH using key-based authentication.
type SSHExecutor struct {
	host       string
	port       int
	user       string
	keyPath    string
	knownHosts string
}

// NewSSHExecutor creates an SSHExecutor for the given host configuration.
// If port is 0, defaults to 22.
func NewSSHExecutor(host string, port int, user, keyPath, knownHosts string) *SSHExecutor {
	return &SSHExecutor{
		host:       host,
		port:       port,
		user:       user,
		keyPath:    keyPath,
		knownHosts: knownHosts,
	}
}

// Name returns "ssh".
func (e *SSHExecutor) Name() string { return "ssh" }

// Exec connects to the remote host via SSH and runs the command.
// The dir parameter is prepended as "cd <dir> && " before the command.
func (e *SSHExecutor) Exec(ctx context.Context, cmd string, args []string, dir string) (string, error) {
	if e.host == "" {
		return "", fmt.Errorf("ssh host not configured")
	}

	config, err := e.clientConfig()
	if err != nil {
		return "", err
	}

	port := e.port
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(e.host, fmt.Sprintf("%d", port))

	client, err := e.dialWithContext(ctx, addr, config)
	if err != nil {
		return "", fmt.Errorf("ssh connection to %s failed: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session creation failed: %w", err)
	}
	defer session.Close()

	commandStr := e.buildCommand(cmd, args, dir)
	output, err := session.CombinedOutput(commandStr)
	return string(output), err
}

// clientConfig builds the SSH client configuration from the executor's settings.
func (e *SSHExecutor) clientConfig() (*ssh.ClientConfig, error) {
	key, err := os.ReadFile(e.keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ssh key %s: %w", e.keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh key %s: %w", e.keyPath, err)
	}

	return &ssh.ClientConfig{
		User: e.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}

// dialWithContext attempts an SSH dial respecting the context deadline.
func (e *SSHExecutor) dialWithContext(ctx context.Context, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	type result struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		client, err := ssh.Dial("tcp", addr, config)
		ch <- result{client, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.client, r.err
	}
}

// buildCommand reconstructs a single command string from cmd + args, with optional cd prefix.
func (e *SSHExecutor) buildCommand(cmd string, args []string, dir string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, cmd)
	parts = append(parts, args...)
	commandStr := strings.Join(parts, " ")

	if dir != "" {
		commandStr = fmt.Sprintf("cd %s && %s", dir, commandStr)
	}
	return commandStr
}

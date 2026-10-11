package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestGatewayCLI_HelpAndInvalidFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunGatewayCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("RunGatewayCLI(--help) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") && !strings.Contains(stdout.String(), "gateway") {
		t.Errorf("stdout = %q, want usage output", stdout.String())
	}
}

func TestGatewayCLI_DisabledGateway(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GAIA_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte("gateway:\n  enabled: false\n"), 0o600)

	var stdout, stderr bytes.Buffer
	code := RunGatewayCLI([]string{"run", "--config", configPath}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("RunGatewayCLI(disabled gateway) = 0, want non-zero error code")
	}
	if !strings.Contains(stderr.String(), "disabled") && !strings.Contains(stderr.String(), "enabled") {
		t.Errorf("stderr = %q, want mention of disabled gateway", stderr.String())
	}
}

func TestGatewayCLI_RunWithContextCancel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GAIA_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte(`
db:
  path: ":memory:"
gateway:
  enabled: true
  telegram:
    enabled: true
    token: "mock-token"
    allowlist: ["123"]
`), 0o600)

	ctx, cancel := context.WithCancel(context.Background())

	var stdout, stderr safeBuffer
	done := make(chan int, 1)
	go func() {
		done <- runGatewayWithContext(ctx, []string{"run", "--config", configPath}, &stdout, &stderr)
	}()

	// Wait until daemon initializes, then cancel context to simulate SIGINT
	for i := 0; i < 100; i++ {
		if strings.Contains(stdout.String(), "running") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("runGatewayWithContext exit code = %d, want 0 on graceful shutdown", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("gateway daemon did not shut down within timeout")
	}
}

func TestGatewayCLI_SlackAndWhatsApp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GAIA_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte(`
db:
  path: ":memory:"
gateway:
  enabled: true
  slack:
    enabled: true
    bot_token: "xoxb-mock"
    signing_secret: "mock-secret"
    listen_addr: "127.0.0.1:0"
    allowed_users: ["U123"]
  whatsapp:
    enabled: true
    api_token: "meta-mock"
    phone_number_id: "phone123"
    verify_token: "ver-mock"
    app_secret: "app-mock"
    listen_addr: "127.0.0.1:0"
    allowed_users: ["+15551234567"]
`), 0o600)

	ctx, cancel := context.WithCancel(context.Background())

	var stdout, stderr safeBuffer
	done := make(chan int, 1)
	go func() {
		done <- runGatewayWithContext(ctx, []string{"run", "--config", configPath}, &stdout, &stderr)
	}()

	for i := 0; i < 100; i++ {
		if strings.Contains(stdout.String(), "running") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("runGatewayWithContext exit code = %d, want 0 on graceful shutdown", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("gateway daemon did not shut down within timeout")
	}
}

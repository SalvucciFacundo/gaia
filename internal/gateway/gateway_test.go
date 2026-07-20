package gateway

import (
	"context"
	"testing"

	"gaia/internal/core/ports"
)

func TestGatewayRegisterAndList(t *testing.T) {
	gw := NewGateway()

	if len(gw.ListAdapters()) != 0 {
		t.Error("expected zero adapters for new gateway")
	}

	// Register a mock adapter.
	mock := &mockAdapter{name: "test-adapter"}
	gw.Register(mock)

	adapters := gw.ListAdapters()
	if len(adapters) != 1 {
		t.Fatalf("expected 1 adapter, got %d", len(adapters))
	}
	if adapters[0] != "test-adapter" {
		t.Errorf("expected 'test-adapter', got %q", adapters[0])
	}
}

func TestGatewayRemove(t *testing.T) {
	gw := NewGateway()
	mock := &mockAdapter{name: "test-adapter"}
	gw.Register(mock)

	if err := gw.Remove("test-adapter"); err != nil {
		t.Fatalf("unexpected remove error: %v", err)
	}
	if len(gw.ListAdapters()) != 0 {
		t.Error("expected zero adapters after remove")
	}
}

func TestGatewayRemoveNotFound(t *testing.T) {
	gw := NewGateway()
	err := gw.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent adapter")
	}
}

func TestGatewayIsRunning(t *testing.T) {
	gw := NewGateway()
	if gw.IsRunning() {
		t.Error("gateway should not be running initially")
	}

	mock := &mockAdapter{name: "test"}
	gw.Register(mock)

	ctx := context.Background()
	err := gw.Start(ctx, func(ctx context.Context, msg ports.IncomingMessage) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("start error: %v", err)
	}
	defer gw.Stop()

	if !gw.IsRunning() {
		t.Error("gateway should be running after start")
	}
}

func TestGatewaySend(t *testing.T) {
	gw := NewGateway()
	mock := &mockAdapter{name: "test"}
	gw.Register(mock)

	ctx := context.Background()
	err := gw.Send(ctx, "test", "target-id", "hello")
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	if mock.lastTarget != "target-id" {
		t.Errorf("expected target 'target-id', got %q", mock.lastTarget)
	}
	if mock.lastContent != "hello" {
		t.Errorf("expected content 'hello', got %q", mock.lastContent)
	}
}

func TestGatewaySendUnknownPlatform(t *testing.T) {
	gw := NewGateway()
	err := gw.Send(context.Background(), "unknown", "target", "hello")
	if err == nil {
		t.Error("expected error for unknown platform")
	}
}

// mockAdapter implements ports.GatewayAdapter for testing.
type mockAdapter struct {
	name        string
	lastTarget  string
	lastContent string
	started     bool
}

func (m *mockAdapter) Name() string { return m.name }

func (m *mockAdapter) Start(ctx context.Context, handler ports.MessageHandler) error {
	m.started = true
	return nil
}

func (m *mockAdapter) Stop() error {
	m.started = false
	return nil
}

func (m *mockAdapter) Send(ctx context.Context, target string, content string) error {
	m.lastTarget = target
	m.lastContent = content
	return nil
}

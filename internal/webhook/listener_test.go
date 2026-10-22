package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewListener(t *testing.T) {
	l := NewListener(":8888")
	if l.Addr() != ":8888" {
		t.Errorf("expected addr ':8888', got %q", l.Addr())
	}
	if l.IsRunning() {
		t.Error("listener should not be running initially")
	}
}

func TestAddAndListSubscriptions(t *testing.T) {
	l := NewListener(":8080")

	sub := Subscription{
		ID:     "sub-1",
		Name:   "test-subscription",
		Secret: "secret123",
		Events: []string{"push", "pull_request"},
		Action: "Test action",
	}
	l.AddSubscription(sub)

	subs := l.ListSubscriptions()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}

	if subs[0].ID != "sub-1" {
		t.Errorf("expected ID 'sub-1', got %q", subs[0].ID)
	}
}

func TestRemoveSubscription(t *testing.T) {
	l := NewListener(":8080")

	l.AddSubscription(Subscription{ID: "sub-1"})
	l.RemoveSubscription("sub-1")

	if len(l.ListSubscriptions()) != 0 {
		t.Error("expected zero subscriptions after remove")
	}
}

func TestRegisterHandler(t *testing.T) {
	l := NewListener(":8080")
	mh := &mockHandler{types: []string{"push"}}
	l.RegisterHandler(mh)

	// Cannot inspect private handlers directly, but we verify
	// the listener doesn't panic on registration.
	l.mu.RLock()
	if len(l.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(l.handlers))
	}
	l.mu.RUnlock()
}

// mockHandler implements Handler for testing.
type mockHandler struct {
	types   []string
	handled []Event
}

func (m *mockHandler) HandleEvent(ctx context.Context, event Event) error {
	m.handled = append(m.handled, event)
	return nil
}

func (m *mockHandler) EventTypes() []string {
	return m.types
}

func TestHandleWebhook_Endpoints(t *testing.T) {
	l := NewListener(":0")

	// Test health endpoint
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	l.handleHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("health endpoint returned %d, want 200", rec.Code)
	}

	// Test invalid method on webhook
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/webhook", nil)
	l.handleWebhook(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("webhook GET returned %d, want 405", rec.Code)
	}

	// Test valid unsigned webhook (no secret configured)
	rec = httptest.NewRecorder()
	payload := `{"repository":{"full_name":"owner/repo"}}`
	req = httptest.NewRequest("POST", "/webhook", strings.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "123")
	l.handleWebhook(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("webhook POST returned %d, want 200", rec.Code)
	}
}

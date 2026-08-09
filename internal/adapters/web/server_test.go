package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebDashboard_Render(t *testing.T) {
	server := NewServer(nil, "8080", "copilot", "gpt-4o")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.handleDashboard(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status OK, got %d", res.StatusCode)
	}

	body := w.Body.String()
	if !testing.Short() && len(body) == 0 {
		t.Fatal("expected HTML body to be non-empty")
	}
}

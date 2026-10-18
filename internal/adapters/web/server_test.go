package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWebPolicy_StatusAndEndpoints(t *testing.T) {
	server := NewServer(nil, "8080", "copilot", "gpt-4o")

	// 1. GET /web/policy (HTML)
	reqHTML := httptest.NewRequest("GET", "/web/policy", nil)
	wHTML := httptest.NewRecorder()
	server.handlePolicyStatus(wHTML, reqHTML)
	if wHTML.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wHTML.Code)
	}
	bodyHTML := wHTML.Body.String()
	if !strings.Contains(bodyHTML, "Security & Permissions Control Panel") {
		t.Fatalf("expected modal title in body, got: %s", bodyHTML)
	}

	// 2. GET /web/policy?format=json (JSON)
	reqJSON := httptest.NewRequest("GET", "/web/policy?format=json", nil)
	wJSON := httptest.NewRecorder()
	server.handlePolicyStatus(wJSON, reqJSON)
	if wJSON.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wJSON.Code)
	}
	if !strings.Contains(wJSON.Body.String(), "filesystem") {
		t.Fatalf("expected json with filesystem category, got: %s", wJSON.Body.String())
	}

	// 3. POST /web/policy/update (HTML)
	reqUpdate := httptest.NewRequest("POST", "/web/policy/update?category=filesystem&state=block", nil)
	wUpdate := httptest.NewRecorder()
	server.handlePolicyUpdate(wUpdate, reqUpdate)
	if wUpdate.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wUpdate.Code)
	}
	if !strings.Contains(wUpdate.Body.String(), "Security & Permissions Control Panel") {
		t.Fatalf("expected modal in update response")
	}
}


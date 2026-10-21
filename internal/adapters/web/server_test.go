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

func TestWeb_HostOption(t *testing.T) {
	server := NewServer(nil, "9090", "copilot", "gpt-4o", WithHost("0.0.0.0"))
	if server.host != "0.0.0.0" {
		t.Fatalf("expected host 0.0.0.0, got %s", server.host)
	}

	serverDefault := NewServer(nil, "9090", "copilot", "gpt-4o")
	if serverDefault.host != "127.0.0.1" {
		t.Fatalf("expected default host 127.0.0.1, got %s", serverDefault.host)
	}
}

func TestWeb_CSRFProtection(t *testing.T) {
	server := NewServer(nil, "8080", "copilot", "gpt-4o")
	handler := server.csrfProtectionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// 1. GET requests are never blocked
	reqGET := httptest.NewRequest("GET", "/web/policy", nil)
	reqGET.Header.Set("Origin", "http://evil.com")
	wGET := httptest.NewRecorder()
	handler.ServeHTTP(wGET, reqGET)
	if wGET.Code != http.StatusOK {
		t.Fatalf("expected GET to pass, got %d", wGET.Code)
	}

	// 2. Untrusted Cross-Origin POST must be rejected with 403 Forbidden
	reqEvil := httptest.NewRequest("POST", "/web/message", nil)
	reqEvil.Header.Set("Origin", "http://evil-attacker.com")
	reqEvil.Host = "127.0.0.1:8080"
	wEvil := httptest.NewRecorder()
	handler.ServeHTTP(wEvil, reqEvil)
	if wEvil.Code != http.StatusForbidden {
		t.Fatalf("expected evil origin POST to be 403 Forbidden, got %d", wEvil.Code)
	}

	// 3. Same-origin or loopback POST is accepted
	reqGood := httptest.NewRequest("POST", "/web/message", nil)
	reqGood.Header.Set("Origin", "http://127.0.0.1:8080")
	reqGood.Host = "127.0.0.1:8080"
	wGood := httptest.NewRecorder()
	handler.ServeHTTP(wGood, reqGood)
	if wGood.Code != http.StatusOK {
		t.Fatalf("expected same-origin POST to pass, got %d", wGood.Code)
	}

	// 4. Referer check fallback
	reqRefererEvil := httptest.NewRequest("POST", "/web/message", nil)
	reqRefererEvil.Header.Set("Referer", "http://malicious.org/page")
	reqRefererEvil.Host = "localhost:8080"
	wRefererEvil := httptest.NewRecorder()
	handler.ServeHTTP(wRefererEvil, reqRefererEvil)
	if wRefererEvil.Code != http.StatusForbidden {
		t.Fatalf("expected evil referer to be 403 Forbidden, got %d", wRefererEvil.Code)
	}
}


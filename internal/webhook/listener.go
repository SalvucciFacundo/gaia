// Package webhook provides an HTTP webhook listener for GAIA.
// It receives GitHub webhook events, verifies HMAC-SHA256 signatures,
// and routes them to registered handlers that trigger automations.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

// Subscription defines a webhook event subscription.
type Subscription struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Secret  string   `json:"secret"`
	Events  []string `json:"events"`
	Action  string   `json:"action"`
	Enabled bool     `json:"enabled"`
}

// Event represents a verified webhook event.
type Event struct {
	Type      string            `json:"type"`
	Action    string            `json:"action,omitempty"`
	Delivery  string            `json:"delivery"`
	Repo      string            `json:"repo"`
	Payload   map[string]interface{} `json:"payload"`
	Signature string            `json:"-"`
}

// Handler processes verified webhook events.
type Handler interface {
	HandleEvent(ctx context.Context, event Event) error
	EventTypes() []string
}

// Listener is an HTTP server that receives and verifies webhook events.
type Listener struct {
	mu            sync.RWMutex
	server        *http.Server
	subs          map[string]Subscription
	handlers      []Handler
	logger        *log.Logger
	addr          string
	started       bool
	stopCh        chan struct{}
}

// NewListener creates a webhook listener on the given address.
func NewListener(addr string) *Listener {
	return &Listener{
		subs:     make(map[string]Subscription),
		logger:   log.Default(),
		addr:     addr,
		stopCh:   make(chan struct{}),
	}
}

// AddSubscription registers a webhook subscription for event routing.
func (l *Listener) AddSubscription(sub Subscription) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.subs[sub.ID] = sub
}

// RemoveSubscription removes a webhook subscription.
func (l *Listener) RemoveSubscription(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.subs, id)
}

// ListSubscriptions returns all configured subscriptions.
func (l *Listener) ListSubscriptions() []Subscription {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]Subscription, 0, len(l.subs))
	for _, s := range l.subs {
		result = append(result, s)
	}
	return result
}

// RegisterHandler registers a event handler for webhook events.
func (l *Listener) RegisterHandler(h Handler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handlers = append(l.handlers, h)
}

// Start begins listening for webhook events.
func (l *Listener) Start(ctx context.Context) error {
	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return fmt.Errorf("webhook listener already started")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", l.handleWebhook)
	mux.HandleFunc("/health", l.handleHealth)

	l.server = &http.Server{
		Addr:    l.addr,
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}
	l.started = true
	l.mu.Unlock()

	go func() {
		l.logger.Printf("Webhook listener started on %s", l.addr)
		if err := l.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.logger.Printf("Webhook listener error: %v", err)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			l.Stop()
		case <-l.stopCh:
			l.Stop()
		}
	}()

	return nil
}

// Stop gracefully shuts down the webhook listener.
func (l *Listener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.started || l.server == nil {
		return nil
	}

	if err := l.server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("webhook shutdown: %w", err)
	}

	l.started = false
	l.logger.Println("Webhook listener stopped")
	return nil
}

// IsRunning returns whether the listener is currently active.
func (l *Listener) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.started
}

// Addr returns the listener's configured address.
func (l *Listener) Addr() string {
	return l.addr
}

// handleWebhook is the HTTP handler for incoming webhook requests.
func (l *Listener) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		l.logger.Printf("webhook: read body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	signature := r.Header.Get("X-Hub-Signature-256")

	if eventType == "" {
		http.Error(w, "missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	// Verify HMAC signature if a secret is configured for any subscription.
	if err := l.verifySignature(body, signature, eventType); err != nil {
		l.logger.Printf("webhook: signature verification failed: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse payload.
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		l.logger.Printf("webhook: parse payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	event := Event{
		Type:      eventType,
		Delivery:  deliveryID,
		Repo:      extractRepo(payload),
		Payload:   payload,
		Signature: signature,
	}

	// Dispatch to matching handlers.
	l.mu.RLock()
	handlers := l.handlers
	l.mu.RUnlock()

	for _, h := range handlers {
		for _, et := range h.EventTypes() {
			if et == eventType || et == "*" {
				go func(handler Handler) {
					if err := handler.HandleEvent(context.Background(), event); err != nil {
						l.logger.Printf("webhook: handler error: %v", err)
					}
				}(h)
				break
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleHealth responds to health check requests.
func (l *Listener) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// verifySignature validates the HMAC-SHA256 signature against configured
// subscription secrets. If no subscription has a secret configured, verification
// is skipped (insecure — log a warning).
func (l *Listener) verifySignature(body []byte, signature, eventType string) error {
	if signature == "" {
		// No signature provided — check if any subscription requires it.
		l.mu.RLock()
		for _, s := range l.subs {
			if s.Secret != "" {
				l.mu.RUnlock()
				return fmt.Errorf("missing signature for event %q", eventType)
			}
		}
		l.mu.RUnlock()
		return nil // No secrets configured — accept unsigned.
	}

	// signature format: sha256=<hex>
	const prefix = "sha256="
	if len(signature) < len(prefix) || signature[:len(prefix)] != prefix {
		return fmt.Errorf("invalid signature format")
	}
	hexSig := signature[len(prefix):]

	// Try each configured subscription secret.
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, s := range l.subs {
		if s.Secret == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(s.Secret))
		mac.Write(body)
		expectedMAC := hex.EncodeToString(mac.Sum(nil))

		if hmac.Equal([]byte(hexSig), []byte(expectedMAC)) {
			return nil // Valid.
		}
	}

	return fmt.Errorf("HMAC signature mismatch")
}

// extractRepo extracts the repository name from a GitHub webhook payload.
func extractRepo(payload map[string]interface{}) string {
	if repo, ok := payload["repository"].(map[string]interface{}); ok {
		if name, ok := repo["full_name"].(string); ok {
			return name
		}
	}
	return ""
}

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"gaia/internal/adapters/web/views"
	"gaia/internal/core"
	"gaia/internal/core/domain"
)

// ServerOption configures Server behavior.
type ServerOption func(*Server)

// WithHost configures the network host to bind to.
func WithHost(host string) ServerOption {
	return func(s *Server) {
		if host != "" {
			s.host = host
		}
	}
}

// Server implements the Web UI server adapter for GAIA.
type Server struct {
	mu            sync.RWMutex
	brain         *core.Brain
	host          string
	port          string
	projects      []views.ProjectViewModel
	activeProject views.ProjectViewModel
	messages      []domain.Message
	subagents     []views.SubagentStateViewModel
	tasks         []string
	providerName  string
	modelName     string
}

// NewServer creates a new Web UI Server adapter.
func NewServer(brain *core.Brain, port string, providerName, modelName string, opts ...ServerOption) *Server {
	if port == "" {
		port = "8080"
	}

	cwd, _ := os.Getwd()
	defaultProject := views.ProjectViewModel{
		ID:         "default",
		Name:       "Current Workspace",
		Path:       cwd,
		Active:     true,
		TaskCount:  0,
		StatusText: "Ready",
	}

	subagentList := []views.SubagentStateViewModel{
		{Name: "explorer", Role: "Investigate codebase", Active: false},
		{Name: "proposer", Role: "Change proposal", Active: false},
		{Name: "specifier", Role: "Requirements & specs", Active: false},
		{Name: "designer", Role: "Technical architecture", Active: false},
		{Name: "planner", Role: "Task breakdown", Active: false},
		{Name: "implementer", Role: "Write & modify code", Active: false},
		{Name: "verifier", Role: "Execute tests & verify", Active: false},
	}

	s := &Server{
		brain:         brain,
		host:          "127.0.0.1",
		port:          port,
		projects:      []views.ProjectViewModel{defaultProject},
		activeProject: defaultProject,
		subagents:     subagentList,
		providerName:  providerName,
		modelName:     modelName,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start launches the HTTP Web UI server.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/web/message", s.handleSendMessage)
	mux.HandleFunc("/web/projects/select", s.handleSelectProject)
	mux.HandleFunc("/web/stream", s.handleSSEStream)
	mux.HandleFunc("/web/policy", s.handlePolicyStatus)
	mux.HandleFunc("/web/policy/update", s.handlePolicyUpdate)

	bindHost := s.host
	if bindHost == "" {
		bindHost = "127.0.0.1"
	}
	addr := net.JoinHostPort(bindHost, s.port)
	log.Printf("🚀 GAIA Web UI Dashboard listening on http://%s", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: s.csrfProtectionMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("web ui server: %w", err)
	}
	return nil
}

// csrfProtectionMiddleware blocks state-changing cross-origin requests (POST, PUT, DELETE, PATCH).
func (s *Server) csrfProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch {
			if !s.isSameOrigin(r) {
				slog.Warn("Blocked untrusted cross-origin mutation request", "method", r.Method, "path", r.URL.Path, "origin", r.Header.Get("Origin"))
				http.Error(w, "Forbidden: Cross-Origin Request Blocked", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isSameOrigin checks if the request origin matches the requested host or is localhost/127.0.0.1.
func (s *Server) isSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		// Non-browser or direct curl/tools request without origin/referer headers is allowed
		return true
	}

	// Strip scheme if present
	originClean := strings.TrimPrefix(strings.TrimPrefix(origin, "http://"), "https://")
	if idx := strings.Index(originClean, "/"); idx != -1 {
		originClean = originClean[:idx]
	}

	host := r.Host
	if originClean == host {
		return true
	}

	// Allow loopback combinations (localhost, 127.0.0.1)
	originHost, _, err := net.SplitHostPort(originClean)
	if err != nil {
		originHost = originClean
	}
	reqHost, _, err := net.SplitHostPort(host)
	if err != nil {
		reqHost = host
	}

	isLoopback := func(h string) bool {
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	if isLoopback(originHost) && isLoopback(reqHost) {
		return true
	}

	return false
}

func (s *Server) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	// Keep stream alive
	<-r.Context().Done()
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	data := views.WebDashboardData{
		Projects:      s.projects,
		ActiveProject: s.activeProject,
		Messages:      s.messages,
		Subagents:     s.subagents,
		Tasks:         s.tasks,
		ProviderName:  s.providerName,
		ModelName:     s.modelName,
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.RenderLayout(data).Render(r.Context(), w); err != nil {
		slog.Error("Failed to render dashboard layout", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Content cannot be empty", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.messages = append(s.messages, domain.Message{Role: domain.RoleUser, Content: content})
	s.mu.Unlock()

	// Process message asynchronously through Brain
	go func() {
		ctx := context.Background()
		if err := s.brain.ProcessMessage(ctx, content); err != nil {
			s.mu.Lock()
			s.messages = append(s.messages, domain.Message{Role: domain.RoleAssistant, Content: "Error: " + err.Error()})
			s.mu.Unlock()
		}
	}()

	s.mu.RLock()
	data := views.WebDashboardData{
		Messages: s.messages,
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.RenderLayout(data).Render(r.Context(), w); err != nil {
		slog.Error("Failed to render dashboard layout after message", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSelectProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	s.mu.Lock()
	for i, p := range s.projects {
		if p.ID == id {
			s.projects[i].Active = true
			s.activeProject = s.projects[i]
		} else {
			s.projects[i].Active = false
		}
	}
	s.mu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) getPolicyGuardAndTools() (*core.PolicyGuard, []string) {
	if s.brain == nil {
		return nil, nil
	}
	var tools []string
	if reg := s.brain.Registry(); reg != nil {
		tools = reg.Tools()
	}
	return s.brain.PolicyGuard(), tools
}

func (s *Server) handlePolicyStatus(w http.ResponseWriter, r *http.Request) {
	guard, tools := s.getPolicyGuardAndTools()
	categories := core.GetCategorySummaries(guard, tools)

	if r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"categories": categories,
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := views.RenderPolicyModal(categories)
	_, _ = w.Write([]byte(html))
}

func (s *Server) handlePolicyUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	guard, tools := s.getPolicyGuardAndTools()

	category := r.URL.Query().Get("category")
	if category == "" {
		category = r.FormValue("category")
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		state = r.FormValue("state")
	}
	toolName := r.URL.Query().Get("tool")
	if toolName == "" {
		toolName = r.FormValue("tool")
	}

	if guard != nil {
		if category != "" && state != "" {
			core.SetCategoryState(guard, core.PolicyCategoryID(category), core.SimplifiedState(state))
		} else if toolName != "" && state != "" {
			var override core.OverridePolicy
			switch core.SimplifiedState(state) {
			case core.StateAllow:
				override = core.OverrideAllow
			case core.StateAsk:
				override = core.OverrideAskSession
			case core.StateBlock:
				override = core.OverrideDeny
			}
			guard.SetOverride(toolName, override)
		}
	}

	categories := core.GetCategorySummaries(guard, tools)

	if r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"categories": categories,
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := views.RenderPolicyModal(categories)
	_, _ = w.Write([]byte(html))
}

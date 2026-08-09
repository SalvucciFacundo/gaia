package web

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"gaia/internal/adapters/web/views"
	"gaia/internal/core"
	"gaia/internal/core/domain"
)

// Server implements the Web UI server adapter for GAIA.
type Server struct {
	mu            sync.RWMutex
	brain         *core.Brain
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
func NewServer(brain *core.Brain, port string, providerName, modelName string) *Server {
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

	return &Server{
		brain:         brain,
		port:          port,
		projects:      []views.ProjectViewModel{defaultProject},
		activeProject: defaultProject,
		subagents:     subagentList,
		providerName:  providerName,
		modelName:     modelName,
	}
}

// Start launches the HTTP Web UI server.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/web/message", s.handleSendMessage)
	mux.HandleFunc("/web/projects/select", s.handleSelectProject)
	mux.HandleFunc("/web/stream", s.handleSSEStream)

	addr := net.JoinHostPort("0.0.0.0", s.port)
	log.Printf("🚀 GAIA Web UI Dashboard listening on http://%s", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
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
	_ = views.RenderLayout(data).Render(r.Context(), w)
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
	_ = views.RenderLayout(data).Render(r.Context(), w)
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

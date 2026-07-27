package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
)

// ServerStatus represents the current state of an MCP server connection.
type ServerStatus string

const (
	StatusConnected    ServerStatus = "connected"
	StatusDisconnected ServerStatus = "disconnected"
	StatusError        ServerStatus = "error"
)

// ManagedServer wraps an MCP Client with status tracking.
type ManagedServer struct {
	Name       string
	Config     domain.MCPServerConfig
	Client     *Client
	Status     ServerStatus
	Error      string
	ConnectedAt time.Time
	ToolCount  int
}

// Manager maintains a pool of MCP server connections and provides
// lifecycle management: connect, disconnect, status, and tool discovery.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*ManagedServer
}

// NewManager creates an empty MCP manager.
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*ManagedServer),
	}
}

// Add registers a new MCP server configuration without connecting.
func (m *Manager) Add(cfg domain.MCPServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[cfg.Name] = &ManagedServer{
		Name:   cfg.Name,
		Config: cfg,
		Status: StatusDisconnected,
	}
}

// Remove deregisters and disconnects a server.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("MCP server %q not found", name)
	}
	if server.Client != nil && server.Client.IsConnected() {
		server.Client.Close()
	}
	delete(m.servers, name)
	return nil
}

// Connect establishes a connection to a registered server.
func (m *Manager) Connect(ctx context.Context, name string) error {
	m.mu.Lock()
	server, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("MCP server %q not found", name)
	}
	if server.Client != nil && server.Client.IsConnected() {
		m.mu.Unlock()
		return fmt.Errorf("MCP server %q is already connected", name)
	}
	m.mu.Unlock()

	client := NewClient(server.Config)
	if err := client.Connect(ctx); err != nil {
		m.mu.Lock()
		server.Status = StatusError
		server.Error = err.Error()
		m.mu.Unlock()
		return fmt.Errorf("connect MCP %q: %w", name, err)
	}

	// Discover available tools
	tools, err := client.DiscoverTools(ctx)
	if err != nil {
		client.Close()
		m.mu.Lock()
		server.Status = StatusError
		server.Error = fmt.Sprintf("discover tools failed: %v", err)
		m.mu.Unlock()
		return fmt.Errorf("discover MCP tools %q: %w", name, err)
	}

	m.mu.Lock()
	server.Client = client
	server.Status = StatusConnected
	server.ConnectedAt = time.Now()
	server.ToolCount = len(tools)
	server.Error = ""
	m.mu.Unlock()
	return nil
}

// Disconnect closes a server connection.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("MCP server %q not found", name)
	}
	if server.Client != nil && server.Client.IsConnected() {
		server.Client.Close()
	}
	server.Status = StatusDisconnected
	server.Error = ""
	return nil
}

// ConnectAll connects all registered servers.
func (m *Manager) ConnectAll(ctx context.Context) {
	m.mu.RLock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if err := m.Connect(ctx, name); err != nil {
			fmt.Printf("[mcp] %v\n", err)
		}
	}
}

// DisconnectAll disconnects all servers.
func (m *Manager) DisconnectAll() {
	m.mu.RLock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		m.Disconnect(name)
	}
}

// List returns all managed servers with their status.
func (m *Manager) List() []*ManagedServer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ManagedServer, 0, len(m.servers))
	for _, s := range m.servers {
		// Return a copy to avoid race conditions
		copy := *s
		result = append(result, &copy)
	}
	return result
}

// Get returns a single server by name.
func (m *Manager) Get(name string) *ManagedServer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.servers[name]; ok {
		copy := *s
		return &copy
	}
	return nil
}

// StatusText returns a human-readable status summary.
func (m *Manager) StatusText() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.servers) == 0 {
		return "No MCP servers configured."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("MCP Servers (%d):\n\n", len(m.servers)))
	for _, s := range m.servers {
		statusIcon := "●"
		statusColor := "connected"
		switch s.Status {
		case StatusConnected:
			statusIcon = "●"
			statusColor = "connected"
		case StatusDisconnected:
			statusIcon = "○"
			statusColor = "disconnected"
		case StatusError:
			statusIcon = "✕"
			statusColor = "error"
		}
		uptime := ""
		if s.Status == StatusConnected {
			uptime = fmt.Sprintf(" (%s)", time.Since(s.ConnectedAt).Truncate(time.Second).String())
		}
		errInfo := ""
		if s.Error != "" {
			errInfo = fmt.Sprintf(" — %s", s.Error)
		}
		sb.WriteString(fmt.Sprintf("  %s %-20s %s%s%s\n", statusIcon, s.Name, statusColor, uptime, errInfo))
		if s.Status == StatusConnected {
			sb.WriteString(fmt.Sprintf("    Tools: %d\n", s.ToolCount))
		}
	}

	sb.WriteString("\nCommands:\n")
	sb.WriteString("  /mcp              — Show this status\n")
	sb.WriteString("  /mcp connect <n>  — Connect a server\n")
	sb.WriteString("  /mcp disconnect <n> — Disconnect a server\n")
	sb.WriteString("  /mcp list         — List servers\n")

	return sb.String()
}

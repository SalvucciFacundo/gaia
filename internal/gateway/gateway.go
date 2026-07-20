// Package gateway provides a multi-platform messaging gateway for GAIA.
// It defines the GatewayAdapter port interface and a Gateway multiplexer
// that routes messages from platform adapters (Telegram, Discord via MCP)
// to the Brain and back.
package gateway

import (
	"context"
	"fmt"
	"log"
	"sync"

	"gaia/internal/core/ports"
)

// Gateway is a multi-platform messaging gateway that owns several adapters
// and routes messages between them and a central message handler.
type Gateway struct {
	adapters map[string]ports.GatewayAdapter
	mu       sync.RWMutex
	logger   *log.Logger
	started  bool
}

// NewGateway creates an empty Gateway.
func NewGateway() *Gateway {
	return &Gateway{
		adapters: make(map[string]ports.GatewayAdapter),
		logger:   log.Default(),
		started:  false,
	}
}

// Register adds a platform adapter to the gateway. If an adapter with the
// same name already exists, it is replaced.
func (g *Gateway) Register(adapter ports.GatewayAdapter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.adapters[adapter.Name()] = adapter
}

// Remove deregisters and stops a platform adapter by name.
func (g *Gateway) Remove(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	a, ok := g.adapters[name]
	if !ok {
		return fmt.Errorf("gateway: adapter %q not found", name)
	}
	if err := a.Stop(); err != nil {
		return fmt.Errorf("gateway: stop adapter %q: %w", name, err)
	}
	delete(g.adapters, name)
	return nil
}

// Start launches all registered adapters with the given message handler.
func (g *Gateway) Start(ctx context.Context, handler ports.MessageHandler) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.started {
		return fmt.Errorf("gateway already started")
	}

	for name, a := range g.adapters {
		if err := a.Start(ctx, handler); err != nil {
			return fmt.Errorf("gateway: start adapter %q: %w", name, err)
		}
		g.logger.Printf("gateway: adapter %q started", name)
	}

	g.started = true
	g.logger.Println("Gateway started with all adapters")
	return nil
}

// Stop stops all registered adapters.
func (g *Gateway) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.started {
		return nil
	}

	var errs []error
	for name, a := range g.adapters {
		if err := a.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("stop %q: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("gateway stop errors: %v", errs)
	}

	g.started = false
	g.logger.Println("Gateway stopped")
	return nil
}

// IsRunning returns true if the gateway is currently started.
func (g *Gateway) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.started
}

// ListAdapters returns the names of all registered adapters.
func (g *Gateway) ListAdapters() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	names := make([]string, 0, len(g.adapters))
	for name := range g.adapters {
		names = append(names, name)
	}
	return names
}

// Send routes an outgoing message to the specified platform adapter by name.
func (g *Gateway) Send(ctx context.Context, platform, target, content string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	a, ok := g.adapters[platform]
	if !ok {
		return fmt.Errorf("gateway: no adapter for platform %q", platform)
	}
	return a.Send(ctx, target, content)
}

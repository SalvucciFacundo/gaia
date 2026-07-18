package llm

import (
	"context"
	"fmt"
	"io"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// Router implements LLMProvider by delegating to a chain of providers.
// On error, it falls back to the next provider in the chain.
type Router struct {
	providers []ports.LLMProvider
}

// NewRouter creates a router from a list of providers. The first provider
// in the list is used as the primary; subsequent providers act as fallbacks.
func NewRouter(providers []ports.LLMProvider) *Router {
	return &Router{providers: providers}
}

// Chat sends a request to the primary provider, falling back on error.
func (r *Router) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	var lastErr error
	for _, p := range r.providers {
		resp, err := p.Chat(ctx, messages, opts...)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// Stream sends a streaming request with fallback.
func (r *Router) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	var lastErr error
	for _, p := range r.providers {
		stream, err := p.Stream(ctx, messages, opts...)
		if err == nil {
			return stream, nil
		}
		lastErr = err
	}
	pr, pw := io.Pipe()
	pw.CloseWithError(fmt.Errorf("all providers failed: %w", lastErr))
	return pr, nil
}

// Tools delegates to the primary provider's Tools.
func (r *Router) Tools() []domain.ToolDef {
	if len(r.providers) > 0 {
		return r.providers[0].Tools()
	}
	return nil
}

// Package llm provides provider adapters (OpenAI, Anthropic, Ollama, Copilot)
// and a fallback Router that delegates to the active chain.
package llm

import (
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// ProviderConstructor is a factory that creates an LLMProvider from config.
type ProviderConstructor func(cfg *domain.Config) (ports.LLMProvider, error)

// Registry maps provider names to their constructors.
var Registry = map[string]ProviderConstructor{
	"openai":   NewOpenAI,
	"anthropic": NewAnthropic,
	"ollama":   NewOllama,
	"copilot":  NewCopilot,
}

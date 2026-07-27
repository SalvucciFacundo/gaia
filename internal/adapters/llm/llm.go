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
	"deepseek":   NewOpenAICompatible("deepseek", "https://api.deepseek.com/v1", "deepseek", "deepseek-chat"),
	"qwen":       NewOpenAICompatible("qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen", "qwen-max"),
	"groq":       NewOpenAICompatible("groq", "https://api.groq.com/openai/v1", "groq", "llama-3.3-70b-versatile"),
	"openrouter": NewOpenAICompatible("openrouter", "https://openrouter.ai/api/v1", "openrouter", "openai/gpt-4o"),
	"together":   NewOpenAICompatible("together", "https://api.together.xyz/v1", "together", "mistralai/Mixtral-8x7B-Instruct-v0.1"),
	"perplexity": NewOpenAICompatible("perplexity", "https://api.perplexity.ai", "perplexity", "sonar-pro"),
	"fireworks":  NewOpenAICompatible("fireworks", "https://api.fireworks.ai/inference/v1", "fireworks", "accounts/fireworks/models/llama-v3p1-70b-instruct"),
	"opencode-go": NewOpenAICompatible("opencode-go", "https://api.opencode.ai/v1", "opencode-go", "grok-4.5"),
	"opencode-zen": NewOpenAICompatible("opencode-zen", "https://api.opencode.ai/v1", "opencode-zen", "claude-sonnet-4"),
}

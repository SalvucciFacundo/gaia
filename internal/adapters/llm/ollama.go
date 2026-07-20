package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// OllamaAdapter calls the Ollama REST API (default http://localhost:11434).
type OllamaAdapter struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

// NewOllama creates an Ollama adapter from config.
func NewOllama(cfg *domain.Config) (ports.LLMProvider, error) {
	model := cfg.LLM.Model
	if model == "" {
		model = "llama3"
	}
	endpoint := cfg.APIKeys["ollama_endpoint"]
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	return &OllamaAdapter{
		endpoint:   strings.TrimRight(endpoint, "/"),
		model:      model,
		httpClient: &http.Client{},
	}, nil
}

// Chat sends a non-streaming request to Ollama.
func (a *OllamaAdapter) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	payload := a.buildPayload(messages, false)
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", a.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Done bool `json:"done"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama decode: %w", err)
	}

	return &domain.Message{
		Role:    domain.RoleAssistant,
		Content: result.Message.Content,
	}, nil
}

// Stream sends a streaming request to Ollama; returns an io.ReadCloser of normalized tokens.
func (a *OllamaAdapter) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	payload := a.buildPayload(messages, true)
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", a.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: %w", err)
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := json.Unmarshal(line, &chunk); err != nil {
				continue
			}
			if chunk.Message.Content != "" {
				data, _ := json.Marshal(domain.TokenChunk{Content: chunk.Message.Content})
				data = append(data, '\n')
				pw.Write(data)
			}
		}
	}()

	return pr, nil
}

// Tools returns the provider's tool definitions.
func (a *OllamaAdapter) Tools() []domain.ToolDef {
	return nil
}

func (a *OllamaAdapter) buildPayload(messages []domain.Message, stream bool) map[string]interface{} {
	msgs := make([]map[string]interface{}, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, map[string]interface{}{
			"role":    string(m.Role),
			"content": m.Content,
		})
	}
	return map[string]interface{}{
		"model":    a.model,
		"messages": msgs,
		"stream":   stream,
	}
}

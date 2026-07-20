package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type CopilotClient struct {
	token      string
	model      string
	httpClient *http.Client
}

// NewCopilot creates a Copilot adapter from config.
func NewCopilot(cfg *domain.Config) (ports.LLMProvider, error) {
	token := cfg.APIKeys["copilot"]
	if token == "" {
		return nil, fmt.Errorf("copilot: missing api key")
	}
	model := cfg.LLM.Model
	if model == "" {
		model = "gpt-4o"
	}
	return NewCopilotClient(token, model), nil
}

func NewCopilotClient(token, model string) *CopilotClient {
	return &CopilotClient{
		token:      token,
		model:      model,
		httpClient: &http.Client{},
	}
}

func (c *CopilotClient) Chat(ctx context.Context, history []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	if c.model == "" {
		c.model = "gpt-4o"
	}

	payload := map[string]interface{}{
		"messages": history,
		"model":    c.model,
		"stream":   false,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.githubcopilot.com/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("copilot api error (%d): %s", resp.StatusCode, string(b))
	}

	var result struct {
		Choices []struct {
			Message domain.Message `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response from copilot")
	}

	return &result.Choices[0].Message, nil
}

// Stream implements the LLMProvider streaming interface.
func (c *CopilotClient) Stream(ctx context.Context, history []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	if c.model == "" {
		c.model = "gpt-4o"
	}

	payload := map[string]interface{}{
		"messages": history,
		"model":    c.model,
		"stream":   true,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.githubcopilot.com/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					return
				}
				_ = pw.CloseWithError(err)
				return
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				data, _ := json.Marshal(domain.TokenChunk{Content: chunk.Choices[0].Delta.Content})
				data = append(data, '\n')
				pw.Write(data)
			}
		}
	}()

	return pr, nil
}

// Tools returns the provider's tool definitions.
func (c *CopilotClient) Tools() []domain.ToolDef {
	return nil
}

// FetchModels retrieves available models from the Copilot API.
func (c *CopilotClient) FetchModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.githubcopilot.com/models", nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func (c *CopilotClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Github-Api-Version", "2023-07-07")
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")
	req.Header.Set("Editor-Version", "vscode/1.90.0")
	req.Header.Set("Editor-Plugin-Version", "copilot-chat/0.15.0")
	req.Header.Set("User-Agent", "GitHub-Copilot-CLI/0.1.0")
}

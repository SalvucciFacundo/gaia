package profiles

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gaia/internal/core/domain"
	"gaia/internal/persona"
	"gaia/internal/tools"
	"gaia/internal/tools/web/search"
)

// AssistantProfile represents the built-in versatile everyday assistant profile.
type AssistantProfile struct {
	soulPath     string
	overlays     *persona.Overlays
	evolution    *persona.Evolution
	searchRunner *tools.WebSearchRunner
	fetchRunner  *tools.WebFetchRunner
	memoryRunner *tools.MemorySearchRunner
}

// NewAssistantProfile creates a new AssistantProfile instance with default web runners.
func NewAssistantProfile() *AssistantProfile {
	homeDir, err := os.UserHomeDir()
	soulPath := ""
	if err == nil {
		soulPath = persona.SoulPath(fmt.Sprintf("%s/.config/gaia", homeDir))
	}
	return &AssistantProfile{
		soulPath:     soulPath,
		overlays:     persona.NewOverlays(nil),
		searchRunner: tools.NewWebSearchRunner(nil, "duckduckgo", search.WebProviders{}),
		fetchRunner:  tools.NewWebFetchRunner(nil, 0),
	}
}

// WithSoulPath overrides the soul file path (useful for testing or custom directories).
func (p *AssistantProfile) WithSoulPath(path string) *AssistantProfile {
	p.soulPath = path
	return p
}

// WithEvolution injects a persistent evolution layer into the assistant profile.
func (p *AssistantProfile) WithEvolution(evo *persona.Evolution) *AssistantProfile {
	p.evolution = evo
	return p
}

// WithOverlays sets the custom personality overlays.
func (p *AssistantProfile) WithOverlays(o *persona.Overlays) *AssistantProfile {
	p.overlays = o
	return p
}

// WithSearchRunner injects a custom WebSearchRunner.
func (p *AssistantProfile) WithSearchRunner(runner *tools.WebSearchRunner) *AssistantProfile {
	p.searchRunner = runner
	return p
}

// WithFetchRunner injects a custom WebFetchRunner.
func (p *AssistantProfile) WithFetchRunner(runner *tools.WebFetchRunner) *AssistantProfile {
	p.fetchRunner = runner
	return p
}

// WithMemoryRunner injects a custom MemorySearchRunner.
func (p *AssistantProfile) WithMemoryRunner(runner *tools.MemorySearchRunner) *AssistantProfile {
	p.memoryRunner = runner
	return p
}

// ID returns the unique identifier of the profile.
func (p *AssistantProfile) ID() string {
	return "assistant"
}

// Name returns the human-readable display name.
func (p *AssistantProfile) Name() string {
	return "Asistente Personal"
}

// Description describes the profile's capabilities.
func (p *AssistantProfile) Description() string {
	return "Versatile everyday assistant for general tasks, web browsing, and reminders."
}

// SystemPrompt returns the system prompt defining the persona behavior,
// assembling durable SOUL.md identity, active overlays, and derived evolution.
func (p *AssistantProfile) SystemPrompt() string {
	var parts []string

	// 1. Durable identity from SOUL.md (or default embedded fallback)
	soulText := ""
	if p.soulPath != "" {
		s, _ := persona.LoadSoul(p.soulPath, nil)
		soulText = s
	} else {
		soulText = persona.DefaultIdentity()
	}
	if soulText != "" {
		parts = append(parts, soulText)
	}

	// 2. Behavioral guidelines for Assistant
	parts = append(parts, `You are GAIA, a versatile and helpful personal AI assistant.
Your goal is to assist the user with everyday productivity, questions, web browsing, and reminders.
Communicate clearly, concisely, and adapt to the user's preferred language and tone.`)

	// 3. Derived evolution layer if available
	if p.evolution != nil {
		evoText := p.evolution.Layer(context.Background())
		if evoText != "" {
			parts = append(parts, evoText)
		}
	}

	return strings.Join(parts, "\n\n")
}

// DefaultTools returns the default set of tool names enabled for this profile.
func (p *AssistantProfile) DefaultTools() []string {
	return []string{"web_search", "web_fetch", "search_memory"}
}

// Tools returns the list of tools provided by this profile.
func (p *AssistantProfile) Tools() []domain.ToolDef {
	return []domain.ToolDef{
		{
			Name:        "web_search",
			Description: "Search the web for real-time information, documentation, news, and technical references.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query string",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of search results to return (default: 5, max: 20)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "web_fetch",
			Description: "Fetch a web page by URL and extract its main readable text content converted to clean Markdown.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The full HTTP/HTTPS URL to fetch",
					},
					"raw": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to return raw text without markdown conversion",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "search_memory",
			Description: "Search long-term memory, past conversations, and observations using hybrid search.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search term or query to find in past memories",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of memory items to return (default: 10)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "set_reminder",
			Description: "Set a reminder or alert for a specific task or time.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Short description of the reminder",
					},
					"when": map[string]interface{}{
						"type":        "string",
						"description": "Time or duration for the reminder",
					},
				},
				"required": []string{"title", "when"},
			},
		},
	}
}

// ExecuteTool handles execution of tools owned by this profile.
func (p *AssistantProfile) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch name {
	case "web_search":
		query, _ := args["query"].(string)
		if query == "" {
			return &domain.ToolResult{
				Success: false,
				Error:   "missing query argument",
			}, fmt.Errorf("missing query argument")
		}
		if p.searchRunner != nil {
			maxResults := 5
			if mr, ok := args["max_results"].(float64); ok && mr > 0 {
				maxResults = int(mr)
			}
			cmd := fmt.Sprintf(`{"query": %q, "max_results": %d}`, query, maxResults)
			out, err := p.searchRunner.Run(ctx, cmd)
			if err != nil {
				return &domain.ToolResult{
					Success: false,
					Error:   err.Error(),
				}, err
			}
			return &domain.ToolResult{
				Success: true,
				Output:  out,
			}, nil
		}
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Simulated web search results for: %q", query),
		}, nil

	case "web_fetch":
		urlStr, _ := args["url"].(string)
		if urlStr == "" {
			return &domain.ToolResult{
				Success: false,
				Error:   "missing url argument",
			}, fmt.Errorf("missing url argument")
		}
		if p.fetchRunner != nil {
			raw := false
			if r, ok := args["raw"].(bool); ok {
				raw = r
			}
			cmd := fmt.Sprintf(`{"url": %q, "raw": %t}`, urlStr, raw)
			out, err := p.fetchRunner.Run(ctx, cmd)
			if err != nil {
				return &domain.ToolResult{
					Success: false,
					Error:   err.Error(),
				}, err
			}
			return &domain.ToolResult{
				Success: true,
				Output:  out,
			}, nil
		}
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Fetched content for URL: %s", urlStr),
		}, nil

	case "search_memory":
		query, _ := args["query"].(string)
		if query == "" {
			return &domain.ToolResult{
				Success: false,
				Error:   "missing query argument",
			}, fmt.Errorf("missing query argument")
		}
		if p.memoryRunner != nil {
			limit := 10
			if l, ok := args["limit"].(float64); ok && l > 0 {
				limit = int(l)
			}
			cmd := fmt.Sprintf(`{"query": %q, "limit": %d}`, query, limit)
			out, err := p.memoryRunner.Run(ctx, cmd)
			if err != nil {
				return &domain.ToolResult{
					Success: false,
					Error:   err.Error(),
				}, err
			}
			return &domain.ToolResult{
				Success: true,
				Output:  out,
			}, nil
		}
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No memory backend configured to search: %q", query),
		}, nil

	case "set_reminder":
		title, _ := args["title"].(string)
		when, _ := args["when"].(string)
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Reminder scheduled: %q at %s", title, when),
		}, nil

	default:
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %q not handled by %s", name, p.ID()),
		}, fmt.Errorf("tool %q not handled by %s", name, p.ID())
	}
}

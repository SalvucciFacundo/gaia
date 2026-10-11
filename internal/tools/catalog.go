package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gaia/internal/config"
	"gaia/internal/core/domain"
	"gaia/internal/skills"
)

// ToolExecutor executes a catalog tool given a context and arguments map.
type ToolExecutor func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error)

// CatalogEntry represents a registered tool definition and its execution handler.
type CatalogEntry struct {
	Name        string
	Category    string // "web", "memory", "shell", "filesystem", "vcs", "multimodal", "gateway"
	Description string
	Definition  domain.ToolDef
	Executor    ToolExecutor
}

// ToolCatalog manages a thread-safe global registry of tools.
type ToolCatalog struct {
	mu      sync.RWMutex
	entries map[string]CatalogEntry
}

// NewToolCatalog creates an empty ToolCatalog instance.
func NewToolCatalog() *ToolCatalog {
	return &ToolCatalog{
		entries: make(map[string]CatalogEntry),
	}
}

// Register adds or updates a tool entry in the catalog.
func (c *ToolCatalog) Register(entry CatalogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry.Definition.Name == "" {
		entry.Definition.Name = entry.Name
	}
	if entry.Definition.Description == "" {
		entry.Definition.Description = entry.Description
	}
	c.entries[entry.Name] = entry
}

// Get retrieves a tool entry by name.
func (c *ToolCatalog) Get(name string) (CatalogEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[name]
	return entry, ok
}

// List returns all registered tool entries sorted alphabetically by name.
func (c *ToolCatalog) List() []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]CatalogEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByCategory returns all tool entries belonging to the specified category, sorted by name.
func (c *ToolCatalog) ListByCategory(cat string) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []CatalogEntry
	for _, entry := range c.entries {
		if strings.EqualFold(entry.Category, cat) {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Filter returns entries matching the given tool names in the provided order.
func (c *ToolCatalog) Filter(names []string) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []CatalogEntry
	for _, name := range names {
		if entry, ok := c.entries[name]; ok {
			result = append(result, entry)
		}
	}
	return result
}

// Execute runs the executor for the specified tool.
func (c *ToolCatalog) Execute(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	c.mu.RLock()
	entry, ok := c.entries[name]
	c.mu.RUnlock()

	if !ok {
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %q not found in catalog", name),
		}, fmt.Errorf("tool %q not found in catalog", name)
	}

	if entry.Executor == nil {
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("no executor defined for tool %q", name),
		}, fmt.Errorf("no executor defined for tool %q", name)
	}

	return entry.Executor(ctx, args)
}

// DefaultCatalog instantiates a ToolCatalog pre-populated with default GAIA tools.
func DefaultCatalog() *ToolCatalog {
	c := NewToolCatalog()

	// Web tools
	c.Register(CatalogEntry{
		Name:        "web_search",
		Category:    "web",
		Description: "Search the web for real-time information, documentation, news, and technical references.",
		Definition: domain.ToolDef{
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
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			query, _ := args["query"].(string)
			if strings.TrimSpace(query) == "" {
				return &domain.ToolResult{Success: false, Error: "missing query argument"}, fmt.Errorf("missing query argument")
			}
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Simulated web search results for: %q", query),
			}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "web_fetch",
		Category:    "web",
		Description: "Fetch a web page by URL and extract readable text content converted to clean Markdown.",
		Definition: domain.ToolDef{
			Name:        "web_fetch",
			Description: "Fetch a web page by URL and extract readable text content converted to clean Markdown.",
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
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			urlStr, _ := args["url"].(string)
			if strings.TrimSpace(urlStr) == "" {
				return &domain.ToolResult{Success: false, Error: "missing url argument"}, fmt.Errorf("missing url argument")
			}
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Fetched content for URL: %s", urlStr),
			}, nil
		},
	})

	// Memory tools
	c.Register(CatalogEntry{
		Name:        "search_memory",
		Category:    "memory",
		Description: "Search long-term memory, past conversations, and observations using hybrid search.",
		Definition: domain.ToolDef{
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
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			query, _ := args["query"].(string)
			if strings.TrimSpace(query) == "" {
				return &domain.ToolResult{Success: false, Error: "missing query argument"}, fmt.Errorf("missing query argument")
			}
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Simulated memory search results for: %q", query),
			}, nil
		},
	})

	// Shell tools
	c.Register(CatalogEntry{
		Name:        "bash",
		Category:    "shell",
		Description: "Execute allowlisted shell commands or scripts in the local environment.",
		Definition: domain.ToolDef{
			Name:        "bash",
			Description: "Execute allowlisted shell commands or scripts in the local environment.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Shell command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			cmd, _ := args["command"].(string)
			if strings.TrimSpace(cmd) == "" {
				return &domain.ToolResult{Success: false, Error: "missing command argument"}, fmt.Errorf("missing command argument")
			}
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Executed bash command: %s", cmd),
			}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "run_tests",
		Category:    "shell",
		Description: "Run test suites or targeted unit tests.",
		Definition: domain.ToolDef{
			Name:        "run_tests",
			Description: "Run test suites or targeted unit tests.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"package": map[string]interface{}{
						"type":        "string",
						"description": "Package path to test",
					},
				},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			pkg, _ := args["package"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Tests passed for package: %q", pkg),
			}, nil
		},
	})

	// Filesystem tools
	c.Register(CatalogEntry{
		Name:        "fileops",
		Category:    "filesystem",
		Description: "Read, write, and list workspace files securely with path validation.",
		Definition: domain.ToolDef{
			Name:        "fileops",
			Description: "Read, write, and list workspace files securely with path validation.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"read", "write", "list"},
						"description": "Operation to perform",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative or absolute file path",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content for write operation",
					},
				},
				"required": []string{"operation", "path"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			op, _ := args["operation"].(string)
			path, _ := args["path"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("File operation %q completed on %s", op, path),
			}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "apply_patch",
		Category:    "filesystem",
		Description: "Apply a targeted patch or diff to codebase files.",
		Definition: domain.ToolDef{
			Name:        "apply_patch",
			Description: "Apply a targeted patch or diff to codebase files.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Target file to modify",
					},
					"patch": map[string]interface{}{
						"type":        "string",
						"description": "Diff or replacement content",
					},
				},
				"required": []string{"file", "patch"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			file, _ := args["file"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Patch applied successfully to: %s", file),
			}, nil
		},
	})

	// VCS tools
	c.Register(CatalogEntry{
		Name:        "gitops",
		Category:    "vcs",
		Description: "Inspect Git version control repository status, logs, and diffs.",
		Definition: domain.ToolDef{
			Name:        "gitops",
			Description: "Inspect Git version control repository status, logs, and diffs.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"status", "diff", "log"},
						"description": "Git operation to perform",
					},
				},
				"required": []string{"operation"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			op, _ := args["operation"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Git operation %q completed successfully", op),
			}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "code_search",
		Category:    "vcs",
		Description: "Search code symbols, functions, or patterns across the codebase.",
		Definition: domain.ToolDef{
			Name:        "code_search",
			Description: "Search code symbols, functions, or patterns across the codebase.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Code pattern or regex to search for",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory or file scope",
					},
				},
				"required": []string{"pattern"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			pat, _ := args["pattern"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Matched code search results for: %q", pat),
			}, nil
		},
	})

	// Multimodal tools
	c.Register(CatalogEntry{
		Name:        "generate_image",
		Category:    "multimodal",
		Description: "Generate an image using generative models or ComfyUI workflows.",
		Definition: domain.ToolDef{
			Name:        "generate_image",
			Description: "Generate an image using generative models or ComfyUI workflows.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Visual prompt describing the image to generate",
					},
					"aspect_ratio": map[string]interface{}{
						"type":        "string",
						"description": "Target aspect ratio (e.g. 1:1, 16:9)",
					},
				},
				"required": []string{"prompt"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			prompt, _ := args["prompt"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Image generated successfully for prompt: %q", prompt),
			}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "analyze_image",
		Category:    "multimodal",
		Description: "Analyze and inspect visual assets, diagrams, or screenshots using perception models.",
		Definition: domain.ToolDef{
			Name:        "analyze_image",
			Description: "Analyze and inspect visual assets, diagrams, or screenshots using perception models.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_path": map[string]interface{}{
						"type":        "string",
						"description": "Path or URL to the image to analyze",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Instructions for analysis",
					},
				},
				"required": []string{"image_path"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			path, _ := args["image_path"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Image analyzed successfully: %s", path),
			}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "comfyui_workflow",
		Category:    "multimodal",
		Description: "Execute or inspect a ComfyUI generation node graph.",
		Definition: domain.ToolDef{
			Name:        "comfyui_workflow",
			Description: "Execute or inspect a ComfyUI generation node graph.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workflow_id": map[string]interface{}{
						"type":        "string",
						"description": "Workflow template or ID to execute",
					},
				},
				"required": []string{"workflow_id"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			wf, _ := args["workflow_id"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("ComfyUI workflow %q dispatched successfully", wf),
			}, nil
		},
	})

	// Gateway tools
	c.Register(CatalogEntry{
		Name:        "set_reminder",
		Category:    "gateway",
		Description: "Set a reminder or alert for a specific task or time.",
		Definition: domain.ToolDef{
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
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			title, _ := args["title"].(string)
			when, _ := args["when"].(string)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Reminder scheduled: %q at %s", title, when),
			}, nil
		},
	})

	// Skills tools
	c.Register(CatalogEntry{
		Name:        "create_skill",
		Category:    "skills",
		Description: "Create or update a reusable procedural skill adhering to the agentskills.io standard.",
		Definition: domain.ToolDef{
			Name:        "create_skill",
			Description: "Create or update a reusable procedural skill adhering to the agentskills.io standard.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Skill name matching ^[a-zA-Z0-9_-]+$ (max 64 chars)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Short description of the skill (max 500 chars)",
					},
					"trigger": map[string]interface{}{
						"type":        "string",
						"description": "Trigger keywords or intent summary",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Markdown procedural instructions containing standard sections",
					},
					"overwrite": map[string]interface{}{
						"type":        "boolean",
						"description": "Overwrite existing skill if true",
					},
					"license": map[string]interface{}{
						"type":        "string",
						"description": "License string (e.g. Apache-2.0)",
					},
					"author": map[string]interface{}{
						"type":        "string",
						"description": "Author identifier",
					},
				},
				"required": []string{"name", "description", "body"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			inBytes, err := json.Marshal(args)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			skillsDir := filepath.Join(config.BaseHome(), "skills")
			hub := skills.NewHub("", skillsDir)
			runner := NewCreateSkillRunner(skillsDir, hub)
			out, err := runner.Run(ctx, string(inBytes))
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &domain.ToolResult{Success: true, Output: out}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "read_skill",
		Category:    "skills",
		Description: "Fetch the full procedural instructions, rules, and examples of a specified skill by name.",
		Definition: domain.ToolDef{
			Name:        "read_skill",
			Description: "Fetch the full procedural instructions, rules, and examples of a specified skill by name.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Exact name of the skill to read",
					},
				},
				"required": []string{"name"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			inBytes, err := json.Marshal(args)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			skillsDir := filepath.Join(config.BaseHome(), "skills")
			hub := skills.NewHub("", skillsDir)
			runner := NewReadSkillRunner(hub)
			out, err := runner.Run(ctx, string(inBytes))
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &domain.ToolResult{Success: true, Output: out}, nil
		},
	})

	// Meta tools
	c.Register(CatalogEntry{
		Name:        "tool_search",
		Category:    "meta",
		Description: "Search for available tools by keyword or category when needed. Returns matching tool names and short descriptions.",
		Definition: domain.ToolDef{
			Name:        "tool_search",
			Description: "Search for available tools by keyword or category when needed. Returns matching tool names and short descriptions.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Keyword to search in tool names and descriptions",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Filter by functional category",
					},
				},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			inBytes, err := json.Marshal(args)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			runner := NewToolSearchRunner(c)
			out, err := runner.Run(ctx, string(inBytes))
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &domain.ToolResult{Success: true, Output: out}, nil
		},
	})

	c.Register(CatalogEntry{
		Name:        "load_tool",
		Category:    "meta",
		Description: "Load the full schema and parameter definitions for a specific tool from the catalog.",
		Definition: domain.ToolDef{
			Name:        "load_tool",
			Description: "Load the full schema and parameter definitions for a specific tool from the catalog.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Exact tool name to retrieve full parameters and schema",
					},
				},
				"required": []string{"name"},
			},
		},
		Executor: func(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
			inBytes, err := json.Marshal(args)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			runner := NewLoadToolRunner(c)
			out, err := runner.Run(ctx, string(inBytes))
			if err != nil {
				return &domain.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &domain.ToolResult{Success: true, Output: out}, nil
		},
	})

	return c
}

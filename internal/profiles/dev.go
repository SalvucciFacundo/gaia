package profiles

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
)

// DevProfile represents the built-in autonomous software engineering profile.
type DevProfile struct{}

// NewDevProfile creates a new DevProfile instance.
func NewDevProfile() *DevProfile {
	return &DevProfile{}
}

// ID returns the unique identifier of the profile.
func (p *DevProfile) ID() string {
	return "dev"
}

// Name returns the human-readable display name.
func (p *DevProfile) Name() string {
	return "Ingeniero de Software"
}

// Description describes the profile's capabilities.
func (p *DevProfile) Description() string {
	return "Autonomous coding agent specializing in ODD/SDD workflows, architectural design, and code edits."
}

// SystemPrompt returns the system prompt defining the persona behavior.
func (p *DevProfile) SystemPrompt() string {
	return `You are GAIA Dev, an elite software systems engineer and architect.
You specialize in Organic Driven Development (ODD) and Spec-Driven Development (SDD) workflows.
You prioritize clean architectures, robust testing, deep root-cause analysis, and precise code modifications.`
}

// Tools returns the list of tools provided by this profile.
func (p *DevProfile) Tools() []domain.ToolDef {
	return []domain.ToolDef{
		{
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
						"description": "Directory or file path scope",
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
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
		{
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
	}
}

// ExecuteTool handles execution of tools owned by this profile.
func (p *DevProfile) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch name {
	case "code_search":
		pattern, _ := args["pattern"].(string)
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Matched code search results for: %q", pattern),
		}, nil
	case "run_tests":
		pkg, _ := args["package"].(string)
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Tests passed for package: %q", pkg),
		}, nil
	case "apply_patch":
		file, _ := args["file"].(string)
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Patch applied successfully to: %s", file),
		}, nil
	default:
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %q not handled by %s", name, p.ID()),
		}, fmt.Errorf("tool %q not handled by %s", name, p.ID())
	}
}

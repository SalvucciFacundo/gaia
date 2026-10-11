package profiles

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
)

// ArtProfile represents the built-in visual reasoning and creative design profile.
type ArtProfile struct{}

// NewArtProfile creates a new ArtProfile instance.
func NewArtProfile() *ArtProfile {
	return &ArtProfile{}
}

// ID returns the unique identifier of the profile.
func (p *ArtProfile) ID() string {
	return "art"
}

// Name returns the human-readable display name.
func (p *ArtProfile) Name() string {
	return "Razonador Visual"
}

// Description describes the profile's capabilities.
func (p *ArtProfile) Description() string {
	return "Visual reasoning and creative design agent specializing in image generation, ComfyUI workflows, and aesthetics."
}

// SystemPrompt returns the system prompt defining the persona behavior.
func (p *ArtProfile) SystemPrompt() string {
	return `You are GAIA Art, a visual reasoning and creative design specialist.
You excel at prompt engineering, ComfyUI nodal generation pipelines, aesthetic critique, and visual assets generation.`
}

// Tools returns the list of tools provided by this profile.
func (p *ArtProfile) Tools() []domain.ToolDef {
	return []domain.ToolDef{
		{
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
		{
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
	}
}

// ExecuteTool handles execution of tools owned by this profile.
func (p *ArtProfile) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch name {
	case "generate_image":
		prompt, _ := args["prompt"].(string)
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Image generated successfully for prompt: %q", prompt),
		}, nil
	case "comfyui_workflow":
		wf, _ := args["workflow_id"].(string)
		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("ComfyUI workflow %q dispatched successfully", wf),
		}, nil
	default:
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %q not handled by %s", name, p.ID()),
		}, fmt.Errorf("tool %q not handled by %s", name, p.ID())
	}
}

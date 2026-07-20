package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaia/internal/agent/memory"
	"gaia/internal/core/domain"
)

// SubagentDef holds the user-defined configuration for a dynamic subagent.
// It is the persistence model for the subagent_defs SQLite table.
type SubagentDef struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	AllowedTools []string        `json:"allowed_tools"`
	Skills       []string        `json:"skills"`
	SystemPrompt string          `json:"system_prompt"`
	Personality  string          `json:"personality"`
	MoA          domain.MoAConfig `json:"moa"`
	CreatedAt    time.Time       `json:"created_at"`
}

// DefRepository is the persistence contract for dynamic subagent definitions.
type DefRepository interface {
	CreateDef(ctx context.Context, def SubagentDef) error
	GetDef(ctx context.Context, name string) (*SubagentDef, error)
	ListDefs(ctx context.Context) ([]SubagentDef, error)
	UpdateDef(ctx context.Context, def SubagentDef) error
	DeleteDef(ctx context.Context, name string) error
}

// DynamicSubagent implements the Subagent interface using a user-defined
// SubagentDef. It behaves identically to compiled subagents — the system
// prompt, personality, and allowed tools come from the persisted definition.
type DynamicSubagent struct {
	def     SubagentDef
	spawner *Spawner
}

// NewDynamicSubagent creates a dynamic subagent from a SubagentDef and Spawner.
func NewDynamicSubagent(def SubagentDef, spawner *Spawner) *DynamicSubagent {
	return &DynamicSubagent{def: def, spawner: spawner}
}

// Name returns the user-assigned subagent name.
func (d *DynamicSubagent) Name() string { return d.def.Name }

// Description returns the user-assigned description.
func (d *DynamicSubagent) Description() string { return d.def.Description }

// Execute runs the dynamic subagent. It enforces the AllowedTools filter
// from the definition and assembles a system prompt from def.SystemPrompt
// and def.Personality. It delegates the agent loop to Spawner.RunLoop().
func (d *DynamicSubagent) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = d.def.AllowedTools

	var nsInstr string
	if ns := d.spawner.Namespace(); ns != nil {
		nsInstr = ns.SaveInstructions(d.def.Name)
	}

	prompt := buildDynamicPrompt(d.def, task, nsInstr)
	resp, err := d.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         fmt.Sprintf("Dynamic subagent %q execution failed: %s", d.def.Name, err.Error()),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	return parseDynamicResult(resp, d.def.Name)
}

// DynamicLoader handles loading persisted SubagentDefs from the repository
// on startup, and registering new dynamic subagents at runtime.
type DynamicLoader struct {
	repo      DefRepository
	registry  *Registry
	spawner   *Spawner
	namespace *memory.NamespaceManager
	validator ToolValidator
}

// ToolValidator is a function that checks a list of tool names against the
// available tool set. Returns an error for unknown tools.
type ToolValidator func(allowed []string) error

// NewDynamicLoader creates a DynamicLoader with the given dependencies.
func NewDynamicLoader(repo DefRepository, registry *Registry, spawner *Spawner, ns *memory.NamespaceManager) *DynamicLoader {
	return &DynamicLoader{
		repo:      repo,
		registry:  registry,
		spawner:   spawner,
		namespace: ns,
	}
}

// SetValidator configures the tool validation function used by CreateFromDef.
func (dl *DynamicLoader) SetValidator(v ToolValidator) {
	dl.validator = v
}

// LoadAll loads all persisted SubagentDefs and registers them in the Registry.
// Called once at startup.
func (dl *DynamicLoader) LoadAll(ctx context.Context) error {
	defs, err := dl.repo.ListDefs(ctx)
	if err != nil {
		return fmt.Errorf("list dynamic subagent defs: %w", err)
	}

	for _, def := range defs {
		if err := dl.register(def); err != nil {
			return fmt.Errorf("register dynamic subagent %q: %w", def.Name, err)
		}
	}

	return nil
}

// CreateFromDef persists a SubagentDef and registers the subagent in the Registry.
// It validates the AllowedTools before persisting.
func (dl *DynamicLoader) CreateFromDef(ctx context.Context, def SubagentDef) error {
	// Validate allowed tools
	if dl.validator != nil {
		if err := dl.validator(def.AllowedTools); err != nil {
			return fmt.Errorf("tool validation: %w", err)
		}
	}

	if err := dl.repo.CreateDef(ctx, def); err != nil {
		return fmt.Errorf("persist def: %w", err)
	}

	return dl.register(def)
}

// register adds a factory closure to the Registry that creates a DynamicSubagent
// from the given definition.
func (dl *DynamicLoader) register(def SubagentDef) error {
	factory := func(spawner *Spawner) Subagent {
		return NewDynamicSubagent(def, spawner)
	}
	return dl.registry.Register(def.Name, factory)
}

// RemoveDynamic removes a dynamic subagent from both the registry and persistence.
func (dl *DynamicLoader) RemoveDynamic(ctx context.Context, name string) error {
	if err := dl.registry.Unregister(name); err != nil {
		return fmt.Errorf("unregister: %w", err)
	}
	if err := dl.repo.DeleteDef(ctx, name); err != nil {
		return fmt.Errorf("delete def: %w", err)
	}
	return nil
}

// buildDynamicPrompt constructs the system prompt for a dynamic subagent from
// def.SystemPrompt, def.Personality, task context, and optional Engram namespace instructions.
func buildDynamicPrompt(def SubagentDef, task domain.SubagentTask, namespaceInstr string) string {
	var sb strings.Builder

	// System prompt is the core identity and instructions.
	if def.SystemPrompt != "" {
		sb.WriteString(def.SystemPrompt)
		sb.WriteString("\n\n")
	}

	// Personality adds tone and behavior instructions.
	if def.Personality != "" {
		sb.WriteString("PERSONALITY:\n")
		sb.WriteString(def.Personality)
		sb.WriteString("\n\n")
	}

	sb.WriteString("You are a dynamic subagent named ")
	sb.WriteString(def.Name)
	sb.WriteString(".\n")

	if task.Description != "" {
		sb.WriteString("\nTASK:\n")
		sb.WriteString(task.Description)
		sb.WriteString("\n")
	}

	if len(task.KGContext) > 0 {
		sb.WriteString("\nRELEVANT CONTEXT:\n")
		for _, fact := range task.KGContext {
			sb.WriteString("- ")
			sb.WriteString(fact)
			sb.WriteString("\n")
		}
	}

	if len(task.Skills) > 0 {
		sb.WriteString("\nSKILLS TO LOAD:\n")
		for _, s := range task.Skills {
			sb.WriteString("- ")
			sb.WriteString(s)
			sb.WriteString("\n")
		}
	}

	// Inject Engram namespace instructions so the dynamic subagent knows
	// its isolated memory scope and can persist discoveries across sessions.
	if namespaceInstr != "" {
		sb.WriteString("\n")
		sb.WriteString(namespaceInstr)
		sb.WriteString("\n")
	}

	sb.WriteString("\nReturn your result as a structured summary with these sections:\n")
	sb.WriteString("- Status: success, partial, or blocked\n")
	sb.WriteString("- Summary: concise description of what was accomplished\n")
	sb.WriteString("- Artifacts: list of artifacts produced\n")
	sb.WriteString("- NextRecommended: suggested next action or \"none\"\n")
	sb.WriteString("- Risks: any risks or issues encountered (or \"none\")\n")
	sb.WriteString("- SkillResolution: how skills were loaded (or \"none\")\n")

	return sb.String()
}

// parseDynamicResult interprets the LLM response for a dynamic subagent
// and extracts a structured SubagentResult envelope.
func parseDynamicResult(resp *domain.Message, name string) *domain.SubagentResult {
	if resp == nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "No response from LLM.",
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := &domain.SubagentResult{
		Status:          domain.SubagentSuccess,
		Summary:         firstNLinesDynamic(resp.Content, 3),
		NextRecommended: "none",
		SkillResolution: "none",
	}

	content := resp.Content
	lines := strings.Split(content, "\n")

	var currentSection string
	var summaryLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lowerTrimmed := strings.ToLower(trimmed)

		switch {
		case strings.HasPrefix(lowerTrimmed, "status:"):
			currentSection = "status"
			val := strings.TrimSpace(strings.TrimPrefix(lowerTrimmed, "status:"))
			switch strings.ToLower(val) {
			case "partial":
				result.Status = domain.SubagentPartial
			case "blocked":
				result.Status = domain.SubagentBlocked
			default:
				result.Status = domain.SubagentSuccess
			}

		case strings.HasPrefix(lowerTrimmed, "summary:"):
			currentSection = "summary"
			val := strings.TrimSpace(strings.TrimPrefix(lowerTrimmed, "summary:"))
			if val != "" {
				summaryLines = append(summaryLines, val)
			}

		case strings.HasPrefix(lowerTrimmed, "artifacts:"):
			currentSection = "artifacts"

		case strings.HasPrefix(lowerTrimmed, "nextrecommended:") ||
			strings.HasPrefix(lowerTrimmed, "next recommended:"):
			currentSection = "next"
			val := strings.TrimSpace(trimAfterPrefixDynamic(lowerTrimmed, "nextrecommended:", "next recommended:"))
			if val != "" {
				result.NextRecommended = val
			}

		case strings.HasPrefix(lowerTrimmed, "risks:"):
			currentSection = "risks"

		case strings.HasPrefix(lowerTrimmed, "skillresolution:") ||
			strings.HasPrefix(lowerTrimmed, "skill resolution:"):
			currentSection = "skills"
			val := strings.TrimSpace(trimAfterPrefixDynamic(lowerTrimmed, "skillresolution:", "skill resolution:"))
			if val != "" {
				result.SkillResolution = val
			}

		default:
			if trimmed == "" {
				continue
			}
			switch currentSection {
			case "summary":
				summaryLines = append(summaryLines, trimmed)
			case "artifacts":
				a := strings.TrimPrefix(trimmed, "-")
				a = strings.TrimPrefix(a, "*")
				a = strings.TrimSpace(a)
				if a != "" {
					result.Artifacts = append(result.Artifacts, a)
				}
			case "risks":
				r := strings.TrimPrefix(trimmed, "-")
				r = strings.TrimPrefix(r, "*")
				r = strings.TrimSpace(r)
				if r != "" && !strings.EqualFold(r, "none") {
					result.Risks = append(result.Risks, r)
				}
			}
		}
	}

	if len(summaryLines) > 0 {
		result.Summary = strings.Join(summaryLines, " ")
	} else {
		result.Summary = fmt.Sprintf("[%s] %s", name, firstNLinesDynamic(content, 3))
	}

	return result
}

// firstNLinesDynamic truncates text to the first n lines, joined by spaces.
func firstNLinesDynamic(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

// trimAfterPrefixDynamic strips one of the given prefixes from the line (case-insensitive).
func trimAfterPrefixDynamic(line string, prefixes ...string) string {
	lower := strings.ToLower(line)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(line[len(p):])
		}
	}
	return strings.TrimSpace(line)
}

// Compile-time check that DynamicSubagent satisfies the Subagent interface.
var _ Subagent = (*DynamicSubagent)(nil)

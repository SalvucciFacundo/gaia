package core

import (
	"sort"
	"strings"
)

// PolicyCategoryID identifies a high-level functional permission group.
type PolicyCategoryID string

const (
	CategoryFilesystem PolicyCategoryID = "filesystem"
	CategoryExecution  PolicyCategoryID = "execution"
	CategoryNetwork    PolicyCategoryID = "network"
	CategoryState      PolicyCategoryID = "state"
)

// SimplifiedState represents the transparent 3-state traffic light permission.
type SimplifiedState string

const (
	StateAllow SimplifiedState = "allow" // 🟢 Always Allow
	StateAsk   SimplifiedState = "ask"   // 🟡 Ask User
	StateBlock SimplifiedState = "block" // 🔴 Block
)

// CategoryMeta describes a high-level permission category and its blast radius.
type CategoryMeta struct {
	ID           PolicyCategoryID `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	BlastRadius  string           `json:"blast_radius"`
	DefaultTools []string         `json:"default_tools"`
}

// CategorySummary provides the computed status and tool list of a category for UI rendering.
type CategorySummary struct {
	ID          PolicyCategoryID `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	BlastRadius string           `json:"blast_radius"`
	State       SimplifiedState  `json:"state"`
	Tools       []ToolState      `json:"tools"`
}

// ToolState captures the individual tool name and its effective policy state.
type ToolState struct {
	Name  string          `json:"name"`
	State SimplifiedState `json:"state"`
}

// StandardCategories defines the 4 functional categories of GAIA.
var StandardCategories = []CategoryMeta{
	{
		ID:          CategoryFilesystem,
		Name:        "Filesystem & Disk",
		Description: "Read, create, edit, and delete files on local disk.",
		BlastRadius: "Project Workspace",
		DefaultTools: []string{
			"file_read", "file_write", "file_edit", "file_patch", "file_delete", "file_tree",
			"read_file", "write_file", "edit_file", "delete_file",
		},
	},
	{
		ID:          CategoryExecution,
		Name:        "Terminal & Commands",
		Description: "Execute shell commands, sub-processes, and dynamic Go scripts.",
		BlastRadius: "Local Machine (OS)",
		DefaultTools: []string{
			"shell_exec", "bash", "execute_code", "docker_run", "git",
		},
	},
	{
		ID:          CategoryNetwork,
		Name:        "Network & Internet",
		Description: "Search web, fetch remote URLs, and browser automation.",
		BlastRadius: "External Internet",
		DefaultTools: []string{
			"web_search", "web_fetch", "webfetch", "browser_open", "browser_click", "browser_fill",
		},
	},
	{
		ID:          CategoryState,
		Name:        "Memory & State",
		Description: "Mutate database records, persistent memories, and blackboard checkpoints.",
		BlastRadius: "GAIA Storage",
		DefaultTools: []string{
			"mem_save", "mem_update", "mem_delete", "blackboard_set", "blackboard_snapshot",
		},
	},
}

// CategorizeTool maps an arbitrary tool name to its primary functional category.
func CategorizeTool(toolName string) PolicyCategoryID {
	lower := strings.ToLower(toolName)

	if strings.Contains(lower, "mem_") || strings.Contains(lower, "blackboard") || strings.Contains(lower, "knowledge") {
		return CategoryState
	}
	if strings.Contains(lower, "web") || strings.Contains(lower, "browser") || strings.Contains(lower, "http") {
		return CategoryNetwork
	}
	if strings.Contains(lower, "shell") || strings.Contains(lower, "bash") || strings.Contains(lower, "exec") ||
		strings.Contains(lower, "docker") || lower == "git" {
		return CategoryExecution
	}
	// Default to filesystem
	return CategoryFilesystem
}

// GetCategorySummaries compiles the 4 functional categories with active states from PolicyGuard.
func GetCategorySummaries(guard *PolicyGuard, registeredTools []string) []CategorySummary {
	summaries := make([]CategorySummary, len(StandardCategories))

	for i, cat := range StandardCategories {
		// Collect tools belonging to this category
		toolMap := make(map[string]bool, len(cat.DefaultTools)+len(registeredTools))
		for _, t := range cat.DefaultTools {
			toolMap[t] = true
		}
		for _, t := range registeredTools {
			if CategorizeTool(t) == cat.ID {
				toolMap[t] = true
			}
		}

		toolNames := make([]string, 0, len(toolMap))
		for t := range toolMap {
			toolNames = append(toolNames, t)
		}
		sort.Strings(toolNames)

		toolStates := make([]ToolState, 0, len(toolNames))
		blockCount := 0
		askCount := 0
		allowCount := 0

		for _, t := range toolNames {
			state := evaluateToolSimplifiedState(guard, t)
			toolStates = append(toolStates, ToolState{
				Name:  t,
				State: state,
			})
			switch state {
			case StateBlock:
				blockCount++
			case StateAsk:
				askCount++
			case StateAllow:
				allowCount++
			}
		}

		// Aggregate category state
		catState := StateAllow
		if blockCount > 0 && allowCount == 0 && askCount == 0 {
			catState = StateBlock
		} else if askCount > 0 || blockCount > 0 {
			catState = StateAsk
		}

		summaries[i] = CategorySummary{
			ID:          cat.ID,
			Name:        cat.Name,
			Description: cat.Description,
			BlastRadius: cat.BlastRadius,
			State:       catState,
			Tools:       toolStates,
		}
	}

	return summaries
}

// SetCategoryState propagates a simplified state (allow/ask/block) to all tools within a category.
func SetCategoryState(guard *PolicyGuard, catID PolicyCategoryID, state SimplifiedState) {
	if guard == nil {
		return
	}

	var tools []string
	for _, cat := range StandardCategories {
		if cat.ID == catID {
			tools = cat.DefaultTools
			break
		}
	}

	var targetOverride OverridePolicy
	switch state {
	case StateAllow:
		targetOverride = OverrideAllow
	case StateAsk:
		targetOverride = OverrideAskSession
	case StateBlock:
		targetOverride = OverrideDeny
	default:
		targetOverride = OverrideAskSession
	}

	for _, tool := range tools {
		guard.SetOverride(tool, targetOverride)
	}
}

func evaluateToolSimplifiedState(guard *PolicyGuard, toolName string) SimplifiedState {
	if guard == nil {
		return StateAllow
	}

	// Check explicit override first
	if ov, ok := guard.Override(toolName); ok {
		switch ov {
		case OverrideDeny, OverrideSkip:
			return StateBlock
		case OverrideAskOnce, OverrideAskSession, OverrideAskAlways:
			return StateAsk
		case OverrideAllow:
			return StateAllow
		default:
			return StateAsk
		}
	}

	// Evaluate against current tier
	res := guard.Evaluate(toolName, nil)
	if !res.Allowed {
		if res.SuggestedAction == "ask_user" {
			return StateAsk
		}
		return StateBlock
	}

	return StateAllow
}

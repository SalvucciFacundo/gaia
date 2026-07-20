package core

import (
	"strings"
)

// SDDKeywords contains words that signal a substantial code change
// and should trigger the SDD pipeline (Explorer → Proposer → Specifier →
// Implementer → Verifier).
var SDDKeywords = []string{
	"feature", "implement", "refactor", "add",
	"create", "build", "rewrite", "migrate",
	"redesign", "architecture", "restructure",
	"new endpoint", "new service", "new module",
	"breaking change", "api change", "schema change",
}

// SDDCommandPrefix forces SDD pipeline routing regardless of keyword detection.
const SDDCommandPrefix = "/sdd"

// DirectCommandPrefix bypasses SDD pipeline routing.
// Use for quick fixes, questions, or non-code tasks.
const DirectCommandPrefix = "/direct"

// TriggerResult describes the outcome of SDD trigger detection.
type TriggerResult struct {
	ShouldSDD   bool   // Whether to route through SDD pipeline
	ForceDirect bool   // Whether /direct was used to bypass
	ForceSDD    bool   // Whether /sdd was used to force
	Reason      string // Human-readable reason for the decision
}

// DetectSDDTrigger checks whether a user message should trigger the SDD pipeline.
//
// Rules (in priority order):
//  1. /direct → bypass SDD (forceDirect=true)
//  2. /sdd → force SDD pipeline (forceSDD=true)
//  3. Keyword match → trigger SDD (shouldSDD=true)
//  4. No match → normal processing (shouldSDD=false)
func DetectSDDTrigger(content string) TriggerResult {
	trimmed := strings.TrimSpace(content)

	// Priority 1: explicit /direct override bypasses everything
	if strings.HasPrefix(trimmed, DirectCommandPrefix) {
		return TriggerResult{
			ShouldSDD:   false,
			ForceDirect: true,
			ForceSDD:    false,
			Reason:      "user requested /direct — bypassing SDD pipeline",
		}
	}

	// Priority 2: explicit /sdd override forces SDD
	if strings.HasPrefix(trimmed, SDDCommandPrefix) {
		return TriggerResult{
			ShouldSDD:   true,
			ForceDirect: false,
			ForceSDD:    true,
			Reason:      "user requested /sdd — forcing SDD pipeline",
		}
	}

	// Priority 3: keyword heuristic
	lower := strings.ToLower(trimmed)
	for _, kw := range SDDKeywords {
		if strings.Contains(lower, kw) {
			return TriggerResult{
				ShouldSDD:   true,
				ForceDirect: false,
				ForceSDD:    false,
				Reason:      "keyword '" + kw + "' detected in user message",
			}
		}
	}

	// Priority 4: no trigger — normal processing
	return TriggerResult{
		ShouldSDD:   false,
		ForceDirect: false,
		ForceSDD:    false,
		Reason:      "no SDD keywords or commands detected",
	}
}

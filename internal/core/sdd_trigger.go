package core

import (
	"strings"
)

// SDDKeywords contains phrases that signal a mandatory breaking/architectural change.
var SDDKeywords = []string{
	"breaking change", "api change", "schema change",
	"architectural redesign", "architecture redesign",
	"system migration", "database migration",
	"rediseño de arquitectura", "cambio de arquitectura",
	"cambio disruptivo", "migracion de sistema",
}

// SDDProactiveSignals contains signals for multi-module features, new subsystems,
// and substantial changes where proactive SDD suggestion MUST be presented to the user.
var SDDProactiveSignals = []string{
	"subsistema", "subsystem", "pipeline", "nuevo modulo", "new module",
	"motor de", "delivery engine", "review engine", "queue system",
	"sistema de colas", "outbox", "multi-module", "nuevo feature",
	"new feature", "protocol integration", "integracion de protocolo",
	"refactor arquitectonico", "refactor general", "large change",
	"cambio grande", "cambio complejo", "full workflow",
}

// SDDActivePhaseSignals detects phase continuation in active SDD changes.
var SDDActivePhaseSignals = []string{
	"vamos con la fase", "fase 1", "fase 2", "fase 3", "fase 4",
	"phase 1", "phase 2", "phase 3", "phase 4",
	"sdd-apply", "sdd-verify", "sdd-archive", "sdd-tasks", "sdd-design",
	"siguiente fase", "siguiente paso sdd", "siguiente pr",
}

// SDDCommandPrefix forces SDD pipeline routing regardless of keyword detection.
const SDDCommandPrefix = "/sdd"

// DirectCommandPrefix bypasses SDD pipeline routing.
// Use for quick fixes, questions, or non-code tasks.
const DirectCommandPrefix = "/direct"

// TriggerResult describes the outcome of SDD trigger detection.
type TriggerResult struct {
	ShouldSDD   bool   // Whether to route directly through SDD pipeline
	SuggestSDD  bool   // Whether to proactively suggest SDD to the user
	ForceDirect bool   // Whether /direct was used to bypass
	ForceSDD    bool   // Whether /sdd was used to force
	Reason      string // Human-readable reason for the decision
}

// DetectSDDTrigger checks whether a user message should trigger the SDD/ODD feature pipeline.
// Maintained for full backward compatibility with existing tests and callers.
func DetectSDDTrigger(content string) TriggerResult {
	odd := DetectODDRoute(content)
	trimmed := strings.TrimSpace(content)

	if trimmed == "" {
		return TriggerResult{
			ShouldSDD:   false,
			SuggestSDD:  false,
			ForceDirect: false,
			ForceSDD:    false,
			Reason:      "empty input",
		}
	}

	forceDirect := strings.HasPrefix(trimmed, DirectCommandPrefix) || strings.HasPrefix(trimmed, InlineCommandPrefix)
	forceSDD := strings.HasPrefix(trimmed, SDDCommandPrefix) || strings.HasPrefix(trimmed, ODDCommandPrefix)

	shouldSDD := odd.Route == RouteFeatureTracking
	suggestSDD := odd.SuggestFeature

	reason := odd.Reason
	if odd.IsDeprecatedAlias {
		if forceDirect {
			reason = "user requested /direct — bypassing SDD pipeline"
		} else if forceSDD {
			reason = "user requested /sdd — forcing SDD pipeline"
		}
	} else if shouldSDD && strings.HasPrefix(reason, "architectural breaking keyword") {
		// Preserve exact phrasing expected by existing test TestDetectSDDTrigger_ReasonHasContent
		for _, kw := range SDDKeywords {
			if strings.Contains(strings.ToLower(trimmed), kw) {
				reason = "architectural keyword '" + kw + "' detected in user message"
				break
			}
		}
	}

	return TriggerResult{
		ShouldSDD:   shouldSDD,
		SuggestSDD:  suggestSDD,
		ForceDirect: forceDirect,
		ForceSDD:    forceSDD,
		Reason:      reason,
	}
}

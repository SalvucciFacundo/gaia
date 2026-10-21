// Package memory provides memory protocol helpers and namespace management for GAIA.
package memory

import (
	"fmt"
	"strings"
)

// ObservationState represents the lifecycle state of a memory observation in Mneme.
type ObservationState string

const (
	// StateActive indicates verified, actionable memory.
	StateActive ObservationState = "active"
	// StateNeedsReview indicates stale or superseded context requiring re-verification.
	StateNeedsReview ObservationState = "needs_review"
)

// MemoryScope defines the boundary of memory visibility.
type MemoryScope string

const (
	ScopeProject  MemoryScope = "project"
	ScopePersonal MemoryScope = "personal"
)

// MemoryType categorizes the nature of the saved observation.
type MemoryType string

const (
	TypeBugfix         MemoryType = "bugfix"
	TypeDecision       MemoryType = "decision"
	TypeArchitecture   MemoryType = "architecture"
	TypeDiscovery      MemoryType = "discovery"
	TypePattern        MemoryType = "pattern"
	TypeConfig         MemoryType = "config"
	TypePreference     MemoryType = "preference"
	TypeSessionSummary MemoryType = "session_summary"
)

// SaveObservationInput contains arguments for saving an observation into memory.
type SaveObservationInput struct {
	Title         string
	Type          MemoryType
	Scope         MemoryScope
	TopicKey      string
	Content       string
	SessionID     string
	CapturePrompt *bool // False for automated/system artifacts; nil/true for user turns
}

// SessionSummaryInput contains structured data for the mandatory end-of-session summary.
type SessionSummaryInput struct {
	Goal          string
	Instructions  string
	Discoveries   []string
	Accomplished  []string
	NextSteps     []string
	RelevantFiles map[string]string // path -> brief description
}

// ConflictCandidate represents a potential conflict surfaced during memory reconciliation.
type ConflictCandidate struct {
	CandidateID int     `json:"candidate_id"`
	JudgmentID  string  `json:"judgment_id"`
	Relation    string  `json:"relation"`
	Confidence  float64 `json:"confidence"`
	Title       string  `json:"title"`
}

// SaveResult models the response from a mem_save invocation.
type SaveResult struct {
	ID               int                 `json:"id"`
	Status           string              `json:"status"`
	JudgmentRequired bool                `json:"judgment_required"`
	Candidates       []ConflictCandidate `json:"candidates,omitempty"`
}

// MnemeProtocolHelper formats arguments according to the Mneme protocol.
type MnemeProtocolHelper struct {
	project   string
	sessionID string
}

// NewMnemeProtocolHelper initializes a helper with active project and session context.
func NewMnemeProtocolHelper(project, sessionID string) *MnemeProtocolHelper {
	return &MnemeProtocolHelper{
		project:   project,
		sessionID: sessionID,
	}
}

// EngramProtocolHelper is an alias kept for backward compatibility with external tool adapters.
type EngramProtocolHelper = MnemeProtocolHelper

// NewEngramProtocolHelper initializes a protocol helper for backward compatibility.
func NewEngramProtocolHelper(project, sessionID string) *MnemeProtocolHelper {
	return NewMnemeProtocolHelper(project, sessionID)
}

// FormatCurrentProjectArgs returns tool arguments for mem_current_project.
func FormatCurrentProjectArgs(dir string) map[string]any {
	args := make(map[string]any)
	if dir != "" {
		args["directory"] = dir
	}
	return args
}

// FormatSaveArgs builds the payload for a mem_save tool call.
func (h *MnemeProtocolHelper) FormatSaveArgs(input SaveObservationInput) map[string]any {
	args := map[string]any{
		"title":   input.Title,
		"type":    string(input.Type),
		"content": input.Content,
	}

	scope := input.Scope
	if scope == "" {
		scope = ScopeProject
	}
	args["scope"] = string(scope)

	if input.TopicKey != "" {
		args["topic_key"] = input.TopicKey
	}

	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = h.sessionID
	}
	if sessionID != "" {
		args["session_id"] = sessionID
	}

	if input.CapturePrompt != nil {
		args["capture_prompt"] = *input.CapturePrompt
	}

	return args
}

// FormatJudgeArgs builds the payload for a mem_judge call when resolving memory conflicts.
func FormatJudgeArgs(judgmentID, relation, evidence string) map[string]any {
	return map[string]any{
		"judgment_id": judgmentID,
		"relation":    relation,
		"evidence":    evidence,
	}
}

// BuildSessionSummaryText builds standard markdown according to Mneme Session Close Protocol.
func BuildSessionSummaryText(input SessionSummaryInput) string {
	var sb strings.Builder

	sb.WriteString("## Goal\n")
	sb.WriteString(input.Goal)
	sb.WriteString("\n\n")

	if input.Instructions != "" {
		sb.WriteString("## Instructions\n")
		sb.WriteString(input.Instructions)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Discoveries\n")
	if len(input.Discoveries) == 0 {
		sb.WriteString("- None\n")
	} else {
		for _, d := range input.Discoveries {
			sb.WriteString(fmt.Sprintf("- %s\n", d))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Accomplished\n")
	if len(input.Accomplished) == 0 {
		sb.WriteString("- None\n")
	} else {
		for _, a := range input.Accomplished {
			sb.WriteString(fmt.Sprintf("- %s\n", a))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Next Steps\n")
	if len(input.NextSteps) == 0 {
		sb.WriteString("- None\n")
	} else {
		for _, n := range input.NextSteps {
			sb.WriteString(fmt.Sprintf("- %s\n", n))
		}
	}
	sb.WriteString("\n")

	if len(input.RelevantFiles) > 0 {
		sb.WriteString("## Relevant Files\n")
		for path, desc := range input.RelevantFiles {
			sb.WriteString(fmt.Sprintf("- %s — %s\n", path, desc))
		}
	}

	return strings.TrimSpace(sb.String())
}

// FormatSessionSummaryArgs formats arguments for a mem_session_summary call.
func (h *MnemeProtocolHelper) FormatSessionSummaryArgs(summaryText string) map[string]any {
	args := map[string]any{
		"content": summaryText,
	}
	if h.sessionID != "" {
		args["session_id"] = h.sessionID
	}
	return args
}

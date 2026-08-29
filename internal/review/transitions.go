package review

import (
	"fmt"
	"math"
)

// CalculateCorrectionBudget computes the exact mathematical correction ceiling:
// budget = min(200, ceil(original_changed_lines / 2)).
func CalculateCorrectionBudget(originalChangedLines int) int {
	if originalChangedLines <= 0 {
		return 0
	}
	half := math.Ceil(float64(originalChangedLines) / 2.0)
	budget := int(math.Min(200.0, half))
	if budget <= 0 {
		return 1
	}
	return budget
}

// TransitionKind represents the discriminator of a v2 review lifecycle transition.
type TransitionKind string

const (
	TransitionExecute TransitionKind = "execute"
	TransitionCollect TransitionKind = "collect"
	TransitionStop    TransitionKind = "stop"
)

// CollectSlot represents a required evidence capture input.
type CollectSlot struct {
	Name             string            `json:"name"`
	Lens             string            `json:"lens,omitempty"`
	ArtifactSubject  map[string]string `json:"artifact_subject,omitempty"`
	ExpectedRevision string            `json:"expected_revision,omitempty"`
}

// ReviewTransition models the v2 typed transition contract.
type ReviewTransition struct {
	Kind         TransitionKind `json:"kind"`
	Operation    string         `json:"operation,omitempty"`
	Arguments    []string       `json:"arguments,omitempty"`
	CollectSlots []CollectSlot  `json:"collect_slots,omitempty"`
	ReasonCode   string         `json:"reason_code,omitempty"`
	Continuation string         `json:"continuation,omitempty"`
}

// StopContinuations maps canonical stop reason codes to unambiguous user continuations.
var StopContinuations = map[string]string{
	"captured_artifacts_unverifiable":       "A captured reviewer artifact failed verification. Run 'gaia review mode disable --scope clone' to deliver under ordinary policy.",
	"captured_result_selection_unavailable": "Internal invariant violation. File a defect or run 'gaia review mode disable --scope clone'.",
	"captured_verification_evidence_invalid": "Captured verification evidence failed integrity checks. Run 'gaia review mode disable --scope clone'.",
	"corrected_candidate_unavailable":       "Change the candidate content, then re-run review.",
	"correction_repository_verification_failed": "Change the correction candidate within the open budget, then retry review.",
	"corrupted_or_unverifiable_authority":   "Review authority unrecoverable. Run 'gaia review mode disable --scope clone' to deliver under ordinary policy.",
	"manual_intervention_required":          "Review authority state unrecognized. Ask maintainer or run 'gaia review mode disable --scope clone'.",
	"native_stop_required":                  "Escalated lineage requires human decision or review mode disable.",
	"unchanged_or_unverified_authority":     "Candidate unchanged. Change candidate content to start a fresh review or disable review mode.",
}

// GetStopContinuation returns the authoritative continuation for a stop reason code.
func GetStopContinuation(reasonCode string, repoRoot string) string {
	if cont, ok := StopContinuations[reasonCode]; ok {
		return cont
	}
	return fmt.Sprintf("Unknown reason code %q. Run 'gaia review mode disable --scope clone' to deliver under ordinary policy.", reasonCode)
}

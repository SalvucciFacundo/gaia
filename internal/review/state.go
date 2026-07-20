// Package review implements GAIA's formal review engine: risk taxonomy,
// state machine, lens-based analysis, snapshot hashing, and bounded receipts.
package review

import (
	"fmt"

	"gaia/internal/core/domain"
)

// State is an alias for domain.ReviewState, representing a review
// transaction state in the formal state machine.
type State = domain.ReviewState

// Re-export state constants for convenience within the review package.
const (
	StateUnreviewed            = domain.ReviewStateUnreviewed
	StateReviewing             = domain.ReviewStateReviewing
	StateJudgesConfirmed       = domain.ReviewStateJudgesConfirmed
	StateFindingsFrozen        = domain.ReviewStateFindingsFrozen
	StateEvidenceClassified    = domain.ReviewStateEvidenceClassified
	StateFixRequired           = domain.ReviewStateFixRequired
	StateFixing                = domain.ReviewStateFixing
	StateFixValidating         = domain.ReviewStateFixValidating
	StateReadyFinalVerification = domain.ReviewStateReadyFinalVerification
	StateFinalVerifying        = domain.ReviewStateFinalVerifying
	StateApproved              = domain.ReviewStateApproved
	StateEscalated             = domain.ReviewStateEscalated
	StateInvalidated           = domain.ReviewStateInvalidated
)

// validTransitions defines all allowed state transitions in the review
// state machine. A transition not listed here will be rejected.
var validTransitions = map[State][]State{
	StateUnreviewed:            {StateReviewing},
	StateReviewing:             {StateJudgesConfirmed, StateFindingsFrozen},
	StateJudgesConfirmed:       {StateFindingsFrozen},
	StateFindingsFrozen:        {StateEvidenceClassified},
	StateEvidenceClassified:    {StateFixRequired, StateReadyFinalVerification},
	StateFixRequired:           {StateFixing},
	StateFixing:                {StateFixValidating},
	StateFixValidating:         {StateEvidenceClassified},
	StateReadyFinalVerification: {StateFinalVerifying},
	StateFinalVerifying:        {StateApproved, StateEscalated, StateInvalidated},
	// Terminal states (approved, escalated, invalidated) have no outgoing transitions.
}

// Transition validates and records a state transition. It returns nil if
// the transition is valid, or an error describing why it is invalid.
func Transition(from, to State) error {
	if from == to {
		return nil // idempotent — staying in the same state is allowed
	}
	valid, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown state: %s", from)
	}
	for _, v := range valid {
		if v == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %s to %s", from, to)
}

// IsTerminal returns true if the state is a terminal state: approved,
// escalated, or invalidated. Terminal states have no outgoing transitions.
func IsTerminal(s State) bool {
	switch s {
	case StateApproved, StateEscalated, StateInvalidated:
		return true
	default:
		return false
	}
}

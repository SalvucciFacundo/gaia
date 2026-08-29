package sdd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"gaia/internal/agent/learn"
	"gaia/internal/agent/memory"
)

// AttemptState represents the operational state returned by the AttemptLedger.
type AttemptState string

const (
	StateProceed  AttemptState = "proceed"
	StateBlocked  AttemptState = "blocked"
	StateComplete AttemptState = "complete"
)

// AttemptRecord holds details of a single execution attempt within a work unit.
type AttemptRecord struct {
	AttemptNumber int          `json:"attempt_number"`
	Token         string       `json:"token"`
	StartTime     time.Time    `json:"start_time"`
	EndTime       time.Time    `json:"end_time,omitempty"`
	State         AttemptState `json:"state"`
	ChangedLines  int          `json:"changed_lines"`
	Evidence      string       `json:"evidence"`
	Error         string       `json:"error,omitempty"`
}

// WorkUnitEntry tracks execution attempts, line budgets, and learning history for a work unit.
type WorkUnitEntry struct {
	ChangeName      string          `json:"change_name"`
	WorkUnit        string          `json:"work_unit"`
	SubagentName    string          `json:"subagent_name"`
	EvidenceGoal    string          `json:"evidence_goal"`
	MaxAttempts     int             `json:"max_attempts"`
	MaxChangedLines int             `json:"max_changed_lines"`
	CurrentAttempt  int             `json:"current_attempt"`
	State           AttemptState    `json:"state"`
	BlockedReason   string          `json:"blocked_reason,omitempty"`
	ActiveToken     string          `json:"active_token,omitempty"`
	Attempts        []AttemptRecord `json:"attempts"`
	LearnedInsights []string        `json:"learned_insights"`
}

// AttemptLedger manages execution budgets and feeds attempt outcomes into permanent subagent learning loops.
type AttemptLedger struct {
	mu        sync.RWMutex
	entries   map[string]*WorkUnitEntry // key: "change:work_unit"
	namespace *memory.NamespaceManager
	learnLoop *learn.LearningLoop
}

// NewAttemptLedger creates a new AttemptLedger instance.
func NewAttemptLedger(ns *memory.NamespaceManager, loop *learn.LearningLoop) *AttemptLedger {
	return &AttemptLedger{
		entries:   make(map[string]*WorkUnitEntry),
		namespace: ns,
		learnLoop: loop,
	}
}

// AcquireRequest contains the parameters to acquire an attempt slot.
type AcquireRequest struct {
	ChangeName      string
	WorkUnit        string
	SubagentName    string
	EvidenceGoal    string
	MaxAttempts     int
	MaxChangedLines int
}

// AcquireResponse contains the authorization result from the ledger.
type AcquireResponse struct {
	State           AttemptState `json:"state"`
	Token           string       `json:"token,omitempty"`
	AttemptNumber   int          `json:"attempt_number"`
	BlockedReason   string       `json:"blocked_reason,omitempty"`
	LearnedContext  []string     `json:"learned_context,omitempty"`
}

// Acquire reserves an attempt execution slot under budget guards.
// If previous attempts failed, it returns learned insights from the permanent agent's memory.
func (l *AttemptLedger) Acquire(ctx context.Context, req AcquireRequest) (*AcquireResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if req.ChangeName == "" || req.WorkUnit == "" {
		return nil, fmt.Errorf("change name and work unit are required")
	}

	if req.MaxAttempts <= 0 {
		req.MaxAttempts = 3
	}
	if req.MaxChangedLines <= 0 {
		req.MaxChangedLines = 400
	}
	if req.SubagentName == "" {
		req.SubagentName = "implementer"
	}

	key := fmt.Sprintf("%s:%s", req.ChangeName, req.WorkUnit)
	entry, exists := l.entries[key]
	if !exists {
		entry = &WorkUnitEntry{
			ChangeName:      req.ChangeName,
			WorkUnit:        req.WorkUnit,
			SubagentName:    req.SubagentName,
			EvidenceGoal:    req.EvidenceGoal,
			MaxAttempts:     req.MaxAttempts,
			MaxChangedLines: req.MaxChangedLines,
			State:           StateProceed,
			Attempts:        make([]AttemptRecord, 0),
			LearnedInsights: make([]string, 0),
		}
		l.entries[key] = entry
	}

	// Check if already completed
	if entry.State == StateComplete {
		return &AcquireResponse{
			State:         StateComplete,
			AttemptNumber: entry.CurrentAttempt,
		}, nil
	}

	// Check if maximum attempts reached
	if entry.CurrentAttempt >= entry.MaxAttempts {
		entry.State = StateBlocked
		entry.BlockedReason = fmt.Sprintf("max attempts (%d) exceeded for work unit %q", entry.MaxAttempts, req.WorkUnit)
		
		// Record reflection in subagent's learning loop
		if l.learnLoop != nil {
			l.learnLoop.RecordExecution(req.SubagentName)
		}

		return &AcquireResponse{
			State:          StateBlocked,
			AttemptNumber:  entry.CurrentAttempt,
			BlockedReason:  entry.BlockedReason,
			LearnedContext: entry.LearnedInsights,
		}, nil
	}

	// Generate opaque attempt token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate attempt token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	entry.CurrentAttempt++
	entry.ActiveToken = token
	entry.State = StateProceed

	entry.Attempts = append(entry.Attempts, AttemptRecord{
		AttemptNumber: entry.CurrentAttempt,
		Token:         token,
		StartTime:     time.Now(),
		State:         StateProceed,
	})

	return &AcquireResponse{
		State:          StateProceed,
		Token:          token,
		AttemptNumber:  entry.CurrentAttempt,
		LearnedContext: entry.LearnedInsights,
	}, nil
}

// SettleRequest holds the settlement evidence of an attempt.
type SettleRequest struct {
	ChangeName   string
	WorkUnit     string
	Token        string
	Outcome      AttemptState
	ChangedLines int
	Evidence     string
	Error        error
}

// Settle finalizes an attempt, checks line budgets, and feeds insights into permanent subagent memory.
func (l *AttemptLedger) Settle(ctx context.Context, req SettleRequest) (*WorkUnitEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := fmt.Sprintf("%s:%s", req.ChangeName, req.WorkUnit)
	entry, exists := l.entries[key]
	if !exists {
		return nil, fmt.Errorf("work unit %q for change %q not found in ledger", req.WorkUnit, req.ChangeName)
	}

	if entry.ActiveToken == "" || entry.ActiveToken != req.Token {
		return nil, fmt.Errorf("invalid or stale attempt token")
	}

	// Find active attempt record
	var record *AttemptRecord
	for i := range entry.Attempts {
		if entry.Attempts[i].Token == req.Token {
			record = &entry.Attempts[i]
			break
		}
	}

	now := time.Now()
	if record != nil {
		record.EndTime = now
		record.ChangedLines = req.ChangedLines
		record.Evidence = req.Evidence
		if req.Error != nil {
			record.Error = req.Error.Error()
		}
	}

	entry.ActiveToken = ""

	// Check line budget violation
	if req.ChangedLines > entry.MaxChangedLines {
		entry.State = StateBlocked
		entry.BlockedReason = fmt.Sprintf("changed lines (%d) exceeded budget ceiling (%d)", req.ChangedLines, entry.MaxChangedLines)
		if record != nil {
			record.State = StateBlocked
		}

		// Permanent subagent learning: capture budget overshoot insight
		insight := fmt.Sprintf("Attempt %d exceeded line budget: %d lines vs %d max. Scope task down.", entry.CurrentAttempt, req.ChangedLines, entry.MaxChangedLines)
		entry.LearnedInsights = append(entry.LearnedInsights, insight)

		if l.learnLoop != nil {
			l.learnLoop.RecordExecution(entry.SubagentName)
		}
		return entry, nil
	}

	// Handle error or blocked outcome
	if req.Outcome == StateBlocked || req.Error != nil {
		entry.State = StateBlocked
		if req.Error != nil {
			entry.BlockedReason = req.Error.Error()
		} else {
			entry.BlockedReason = "attempt blocked by subagent execution"
		}
		if record != nil {
			record.State = StateBlocked
		}

		// Permanent subagent learning: capture failure pattern
		errText := "unknown error"
		if req.Error != nil {
			errText = req.Error.Error()
		}
		insight := fmt.Sprintf("Attempt %d failed on %q: %s", entry.CurrentAttempt, req.WorkUnit, errText)
		entry.LearnedInsights = append(entry.LearnedInsights, insight)

		if l.learnLoop != nil {
			l.learnLoop.RecordExecution(entry.SubagentName)
		}
		return entry, nil
	}

	// Complete successfully
	if req.Outcome == StateComplete {
		entry.State = StateComplete
		entry.BlockedReason = ""
		if record != nil {
			record.State = StateComplete
		}

		insight := fmt.Sprintf("Work unit %q completed in %d attempt(s) (%d lines changed).", req.WorkUnit, entry.CurrentAttempt, req.ChangedLines)
		entry.LearnedInsights = append(entry.LearnedInsights, insight)

		if l.learnLoop != nil {
			l.learnLoop.RecordExecution(entry.SubagentName)
		}
		return entry, nil
	}

	// Still in progress
	entry.State = StateProceed
	return entry, nil
}

// GetStatus returns the current state and metrics for a change's work unit.
func (l *AttemptLedger) GetStatus(changeName, workUnit string) (*WorkUnitEntry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", changeName, workUnit)
	entry, exists := l.entries[key]
	if !exists {
		return nil, false
	}
	// Return a copy to avoid race conditions
	copied := *entry
	copied.Attempts = make([]AttemptRecord, len(entry.Attempts))
	copy(copied.Attempts, entry.Attempts)
	copied.LearnedInsights = make([]string, len(entry.LearnedInsights))
	copy(copied.LearnedInsights, entry.LearnedInsights)
	return &copied, true
}

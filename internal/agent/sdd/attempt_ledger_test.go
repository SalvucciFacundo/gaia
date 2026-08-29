package sdd

import (
	"context"
	"errors"
	"testing"

	"gaia/internal/agent/learn"
	"gaia/internal/agent/memory"
)

func TestAttemptLedger_Acquire_HappyPath(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	ledger := NewAttemptLedger(ns, loop)

	ctx := context.Background()
	req := AcquireRequest{
		ChangeName:      "test-change",
		WorkUnit:        "unit-1",
		SubagentName:    "implementer",
		EvidenceGoal:    "write unit tests and implement logic",
		MaxAttempts:     3,
		MaxChangedLines: 400,
	}

	resp, err := ledger.Acquire(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.State != StateProceed {
		t.Errorf("expected state 'proceed', got %q", resp.State)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.AttemptNumber != 1 {
		t.Errorf("expected attempt 1, got %d", resp.AttemptNumber)
	}
}

func TestAttemptLedger_Acquire_MaxAttemptsExceeded(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	ledger := NewAttemptLedger(ns, loop)

	ctx := context.Background()
	req := AcquireRequest{
		ChangeName:      "test-change",
		WorkUnit:        "unit-2",
		SubagentName:    "implementer",
		MaxAttempts:     2,
		MaxChangedLines: 400,
	}

	// Attempt 1
	resp1, err := ledger.Acquire(ctx, req)
	if err != nil || resp1.State != StateProceed {
		t.Fatalf("attempt 1 failed: %v", err)
	}
	_, _ = ledger.Settle(ctx, SettleRequest{
		ChangeName: req.ChangeName,
		WorkUnit:   req.WorkUnit,
		Token:      resp1.Token,
		Outcome:    StateBlocked,
		Error:      errors.New("build failed"),
	})

	// Attempt 2
	resp2, err := ledger.Acquire(ctx, req)
	if err != nil || resp2.State != StateProceed {
		t.Fatalf("attempt 2 failed: %v", err)
	}
	_, _ = ledger.Settle(ctx, SettleRequest{
		ChangeName: req.ChangeName,
		WorkUnit:   req.WorkUnit,
		Token:      resp2.Token,
		Outcome:    StateBlocked,
		Error:      errors.New("test failed"),
	})

	// Attempt 3 (should be blocked)
	resp3, err := ledger.Acquire(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp3.State != StateBlocked {
		t.Errorf("expected state 'blocked', got %q", resp3.State)
	}
	if resp3.BlockedReason == "" {
		t.Error("expected non-empty blocked reason")
	}
	if len(resp3.LearnedContext) < 2 {
		t.Errorf("expected at least 2 learned insights, got %d", len(resp3.LearnedContext))
	}
}

func TestAttemptLedger_Settle_HappyPath(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	ledger := NewAttemptLedger(ns, loop)

	ctx := context.Background()
	req := AcquireRequest{
		ChangeName:      "test-change",
		WorkUnit:        "unit-3",
		SubagentName:    "implementer",
		MaxAttempts:     3,
		MaxChangedLines: 400,
	}

	resp, err := ledger.Acquire(ctx, req)
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}

	entry, err := ledger.Settle(ctx, SettleRequest{
		ChangeName:   req.ChangeName,
		WorkUnit:     req.WorkUnit,
		Token:        resp.Token,
		Outcome:      StateComplete,
		ChangedLines: 120,
		Evidence:     "all tests passing",
	})
	if err != nil {
		t.Fatalf("settle error: %v", err)
	}

	if entry.State != StateComplete {
		t.Errorf("expected state 'complete', got %q", entry.State)
	}
	if len(entry.LearnedInsights) == 0 {
		t.Error("expected completion insight to be recorded")
	}

	// Future acquire on completed work unit returns complete
	respAfter, err := ledger.Acquire(ctx, req)
	if err != nil || respAfter.State != StateComplete {
		t.Errorf("expected state 'complete' on re-acquire, got %v / %v", respAfter, err)
	}
}

func TestAttemptLedger_Settle_LineBudgetExceeded(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	ledger := NewAttemptLedger(ns, loop)

	ctx := context.Background()
	req := AcquireRequest{
		ChangeName:      "test-change",
		WorkUnit:        "unit-4",
		SubagentName:    "implementer",
		MaxAttempts:     3,
		MaxChangedLines: 400,
	}

	resp, err := ledger.Acquire(ctx, req)
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}

	entry, err := ledger.Settle(ctx, SettleRequest{
		ChangeName:   req.ChangeName,
		WorkUnit:     req.WorkUnit,
		Token:        resp.Token,
		Outcome:      StateComplete,
		ChangedLines: 480, // Exceeds 400
		Evidence:     "completed but oversized",
	})
	if err != nil {
		t.Fatalf("settle error: %v", err)
	}

	if entry.State != StateBlocked {
		t.Errorf("expected state 'blocked' on budget exceed, got %q", entry.State)
	}
	if entry.BlockedReason == "" {
		t.Error("expected non-empty blocked reason")
	}
}

func TestAttemptLedger_Settle_InvalidToken(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	ledger := NewAttemptLedger(ns, loop)

	ctx := context.Background()
	req := AcquireRequest{
		ChangeName: "test-change",
		WorkUnit:   "unit-5",
	}

	_, _ = ledger.Acquire(ctx, req)

	_, err := ledger.Settle(ctx, SettleRequest{
		ChangeName: req.ChangeName,
		WorkUnit:   req.WorkUnit,
		Token:      "invalid-stale-token",
		Outcome:    StateComplete,
	})
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}

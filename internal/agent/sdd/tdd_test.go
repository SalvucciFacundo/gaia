package sdd

import (
	"testing"

	"gaia/internal/agent/learn"
	"gaia/internal/agent/memory"
)

func TestTDDEngine_FullCycle_HappyPath(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	engine := NewTDDEngine(ns, loop)

	change := "feature-auth"

	// 1. Initial phase is RED
	if phase := engine.CurrentPhase(change); phase != TDDPhaseRed {
		t.Fatalf("expected initial phase RED, got %q", phase)
	}

	// 2. Advance RED phase with failing test
	phase, err := engine.Advance(TDDTransitionInput{
		ChangeName:         change,
		TestFilesChanged:   []string{"auth_test.go"},
		TestRunPassed:      false,
		TestOutput:         "--- FAIL: TestAuth_Login (0.00s)\nassertion failed: expected true, got false",
		IsAssertionFailure: true,
	})
	if err != nil {
		t.Fatalf("RED advance failed: %v", err)
	}
	if phase != TDDPhaseGreen {
		t.Errorf("expected transition to GREEN, got %q", phase)
	}

	// 3. Advance GREEN phase with passing implementation
	phase, err = engine.Advance(TDDTransitionInput{
		ChangeName:       change,
		CodeFilesChanged: []string{"auth.go"},
		TestRunPassed:    true,
		TestOutput:       "PASS\nok  gaia/auth 0.01s",
	})
	if err != nil {
		t.Fatalf("GREEN advance failed: %v", err)
	}
	if phase != TDDPhaseRefactor {
		t.Errorf("expected transition to REFACTOR, got %q", phase)
	}

	// 4. Advance REFACTOR phase with tests still green
	phase, err = engine.Advance(TDDTransitionInput{
		ChangeName:       change,
		CodeFilesChanged: []string{"auth.go"},
		TestRunPassed:    true,
		TestOutput:       "PASS\nok  gaia/auth 0.01s",
	})
	if err != nil {
		t.Fatalf("REFACTOR advance failed: %v", err)
	}
	if phase != TDDPhaseComplete {
		t.Errorf("expected transition to COMPLETE, got %q", phase)
	}

	state, ok := engine.GetState(change)
	if !ok || state.CurrentPhase != TDDPhaseComplete {
		t.Errorf("expected state complete, got %v", state)
	}
	if len(state.LearnedInsights) == 0 {
		t.Error("expected successful TDD cycle insight to be recorded")
	}
}

func TestTDDEngine_RedPhase_ViolationNoTestFiles(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	engine := NewTDDEngine(ns, loop)

	change := "feature-billing"

	// Attempt to advance without test files
	phase, err := engine.Advance(TDDTransitionInput{
		ChangeName:       change,
		CodeFilesChanged: []string{"billing.go"},
		TestRunPassed:    false,
	})
	if err == nil {
		t.Error("expected error for missing test file in RED phase, got nil")
	}
	if phase != TDDPhaseRed {
		t.Errorf("expected to remain in RED phase, got %q", phase)
	}

	state, _ := engine.GetState(change)
	if state.ViolationsCount != 1 {
		t.Errorf("expected 1 violation count, got %d", state.ViolationsCount)
	}
}

func TestTDDEngine_RedPhase_ViolationTestPassedImmediately(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	engine := NewTDDEngine(ns, loop)

	change := "feature-billing"

	// Test passes immediately in RED phase (doesn't fail)
	phase, err := engine.Advance(TDDTransitionInput{
		ChangeName:       change,
		TestFilesChanged: []string{"billing_test.go"},
		TestRunPassed:    true, // Violation
	})
	if err == nil {
		t.Error("expected error when test passes immediately in RED phase, got nil")
	}
	if phase != TDDPhaseRed {
		t.Errorf("expected to remain in RED phase, got %q", phase)
	}
}

func TestTDDEngine_GreenPhase_TestsStillFailing(t *testing.T) {
	ns := memory.NewNamespaceManager("test-project")
	loop := learn.NewLearningLoop(5)
	engine := NewTDDEngine(ns, loop)

	change := "feature-payments"

	// Transition to GREEN
	_, _ = engine.Advance(TDDTransitionInput{
		ChangeName:       change,
		TestFilesChanged: []string{"payments_test.go"},
		TestRunPassed:    false,
	})

	// Try GREEN phase with still failing tests
	phase, err := engine.Advance(TDDTransitionInput{
		ChangeName:       change,
		CodeFilesChanged: []string{"payments.go"},
		TestRunPassed:    false,
		TestOutput:       "FAIL: payments_test.go:20",
	})
	if err == nil {
		t.Error("expected error when tests fail in GREEN phase, got nil")
	}
	if phase != TDDPhaseGreen {
		t.Errorf("expected to remain in GREEN phase, got %q", phase)
	}
}

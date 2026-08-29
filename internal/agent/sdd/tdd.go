package sdd

import (
	"fmt"
	"strings"
	"sync"

	"gaia/internal/agent/learn"
	"gaia/internal/agent/memory"
)

// TDDPhase represents the active phase in the Strict TDD cycle.
type TDDPhase string

const (
	TDDPhaseRed      TDDPhase = "red"
	TDDPhaseGreen    TDDPhase = "green"
	TDDPhaseRefactor TDDPhase = "refactor"
	TDDPhaseComplete TDDPhase = "complete"
)

// TDDState tracks the current phase and history of a TDD cycle for a change.
type TDDState struct {
	ChangeName       string    `json:"change_name"`
	CurrentPhase     TDDPhase  `json:"current_phase"`
	TestFiles        []string  `json:"test_files"`
	CodeFiles        []string  `json:"code_files"`
	RedTestOutput    string    `json:"red_test_output,omitempty"`
	GreenTestOutput  string    `json:"green_test_output,omitempty"`
	ViolationsCount  int       `json:"violations_count"`
	LearnedInsights  []string  `json:"learned_insights"`
}

// TDDEngine coordinates Strict TDD enforcement and feeds test discipline into permanent subagent memory.
type TDDEngine struct {
	mu        sync.RWMutex
	states    map[string]*TDDState
	namespace *memory.NamespaceManager
	learnLoop *learn.LearningLoop
}

// NewTDDEngine creates a new TDDEngine.
func NewTDDEngine(ns *memory.NamespaceManager, loop *learn.LearningLoop) *TDDEngine {
	return &TDDEngine{
		states:    make(map[string]*TDDState),
		namespace: ns,
		learnLoop: loop,
	}
}

// TDDTransitionInput carries the artifacts and test execution results for a phase transition.
type TDDTransitionInput struct {
	ChangeName         string   `json:"change_name"`
	TestFilesChanged   []string `json:"test_files_changed"`
	CodeFilesChanged   []string `json:"code_files_changed"`
	TestRunPassed      bool     `json:"test_run_passed"`
	TestOutput         string   `json:"test_output"`
	IsAssertionFailure bool     `json:"is_assertion_failure"`
}

// CurrentPhase returns the active TDD phase for a change (defaults to TDDPhaseRed).
func (e *TDDEngine) CurrentPhase(changeName string) TDDPhase {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, exists := e.states[changeName]
	if !exists {
		return TDDPhaseRed
	}
	return state.CurrentPhase
}

// GetState returns the full TDD state for a change.
func (e *TDDEngine) GetState(changeName string) (*TDDState, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, exists := e.states[changeName]
	if !exists {
		return nil, false
	}
	copied := *state
	copied.TestFiles = append([]string(nil), state.TestFiles...)
	copied.CodeFiles = append([]string(nil), state.CodeFiles...)
	copied.LearnedInsights = append([]string(nil), state.LearnedInsights...)
	return &copied, true
}

// Advance validates and moves the TDD state machine to the next phase.
func (e *TDDEngine) Advance(input TDDTransitionInput) (TDDPhase, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if input.ChangeName == "" {
		return TDDPhaseRed, fmt.Errorf("change name is required")
	}

	state, exists := e.states[input.ChangeName]
	if !exists {
		state = &TDDState{
			ChangeName:      input.ChangeName,
			CurrentPhase:    TDDPhaseRed,
			TestFiles:       make([]string, 0),
			CodeFiles:       make([]string, 0),
			LearnedInsights: make([]string, 0),
		}
		e.states[input.ChangeName] = state
	}

	switch state.CurrentPhase {
	case TDDPhaseRed:
		// Rule 1: Must author test files first
		if len(input.TestFilesChanged) == 0 {
			state.ViolationsCount++
			insight := "Strict TDD violation: attempted to implement code without authoring a test file first (RED phase)."
			state.LearnedInsights = append(state.LearnedInsights, insight)
			if e.learnLoop != nil {
				e.learnLoop.RecordExecution("implementer")
			}
			return TDDPhaseRed, fmt.Errorf("%s", insight)
		}

		// Rule 2: Test MUST fail initially (proving assertion coverage)
		if input.TestRunPassed {
			state.ViolationsCount++
			insight := "Strict TDD violation: new test passed immediately without failing (RED phase requires a failing test)."
			state.LearnedInsights = append(state.LearnedInsights, insight)
			if e.learnLoop != nil {
				e.learnLoop.RecordExecution("implementer")
			}
			return TDDPhaseRed, fmt.Errorf("%s", insight)
		}

		// Valid RED phase: record test files and transition to GREEN
		state.TestFiles = append(state.TestFiles, input.TestFilesChanged...)
		state.RedTestOutput = input.TestOutput
		state.CurrentPhase = TDDPhaseGreen
		return TDDPhaseGreen, nil

	case TDDPhaseGreen:
		// Rule 1: Must author production code
		if len(input.CodeFilesChanged) == 0 {
			return TDDPhaseGreen, fmt.Errorf("Strict TDD: production code required to satisfy failing test (GREEN phase)")
		}

		// Rule 2: Tests must now pass 100%
		if !input.TestRunPassed {
			insight := fmt.Sprintf("Tests still failing in GREEN phase: %s", summarizeError(input.TestOutput))
			state.LearnedInsights = append(state.LearnedInsights, insight)
			return TDDPhaseGreen, fmt.Errorf("Strict TDD: tests still failing in GREEN phase")
		}

		// Valid GREEN phase: transition to REFACTOR
		state.CodeFiles = append(state.CodeFiles, input.CodeFilesChanged...)
		state.GreenTestOutput = input.TestOutput
		state.CurrentPhase = TDDPhaseRefactor
		return TDDPhaseRefactor, nil

	case TDDPhaseRefactor:
		// Tests must remain passing during refactoring
		if !input.TestRunPassed {
			insight := "Refactoring broke existing tests; tests must remain green."
			state.LearnedInsights = append(state.LearnedInsights, insight)
			return TDDPhaseRefactor, fmt.Errorf("Strict TDD: refactoring broke tests")
		}

		// Cycle Complete: record positive TDD habit in permanent subagent memory
		state.CurrentPhase = TDDPhaseComplete
		successInsight := fmt.Sprintf("Strict TDD cycle completed successfully for %q (tests: %v, code: %v).",
			input.ChangeName, state.TestFiles, state.CodeFiles)
		state.LearnedInsights = append(state.LearnedInsights, successInsight)

		if e.learnLoop != nil {
			e.learnLoop.RecordExecution("implementer")
			e.learnLoop.RecordExecution("verifier")
		}
		return TDDPhaseComplete, nil

	case TDDPhaseComplete:
		return TDDPhaseComplete, nil
	}

	return state.CurrentPhase, nil
}

// Reset clears the TDD state for a change.
func (e *TDDEngine) Reset(changeName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.states, changeName)
}

func summarizeError(output string) string {
	lines := strings.Split(output, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.Contains(trimmed, "FAIL:") || strings.Contains(trimmed, "error:") {
			return trimmed
		}
	}
	if len(lines) > 0 {
		return lines[0]
	}
	return "test execution failure"
}

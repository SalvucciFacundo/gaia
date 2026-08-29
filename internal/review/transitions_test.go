package review

import (
	"testing"
)

func TestCalculateCorrectionBudget(t *testing.T) {
	tests := []struct {
		lines    int
		expected int
	}{
		{0, 0},
		{-10, 0},
		{1, 1},
		{2, 1},
		{9, 5},
		{10, 5},
		{100, 50},
		{399, 200},
		{400, 200},
		{800, 200}, // Capped at 200
	}

	for _, tt := range tests {
		got := CalculateCorrectionBudget(tt.lines)
		if got != tt.expected {
			t.Errorf("CalculateCorrectionBudget(%d) = %d, expected %d", tt.lines, got, tt.expected)
		}
	}
}

func TestGetStopContinuation_KnownAndUnknown(t *testing.T) {
	cont := GetStopContinuation("unchanged_or_unverified_authority", "/repo")
	if cont == "" {
		t.Error("expected non-empty continuation for known reason code")
	}

	unknownCont := GetStopContinuation("unknown_code", "/repo")
	if unknownCont == "" {
		t.Error("expected non-empty continuation for unknown reason code")
	}
}

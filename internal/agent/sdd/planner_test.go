package sdd

import (
	"testing"
)

func TestAnalyzeWorkload(t *testing.T) {
	tests := []struct {
		name                string
		lines               int
		expectedChained     bool
		expectedRisk        string
		expectedStrategy    string
		expectedDecisionReq bool
	}{
		{"small task", 100, false, "Low", "single-pr", false},
		{"medium task", 300, false, "Medium", "single-pr", false},
		{"oversized task", 550, true, "High", "stacked-to-main", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := AnalyzeWorkload(tt.lines)
			if res.ChainedPRsRequired != tt.expectedChained {
				t.Errorf("ChainedPRsRequired = %v, expected %v", res.ChainedPRsRequired, tt.expectedChained)
			}
			if res.BudgetRisk != tt.expectedRisk {
				t.Errorf("BudgetRisk = %q, expected %q", res.BudgetRisk, tt.expectedRisk)
			}
			if res.RecommendedStrategy != tt.expectedStrategy {
				t.Errorf("RecommendedStrategy = %q, expected %q", res.RecommendedStrategy, tt.expectedStrategy)
			}
			if res.DecisionNeeded != tt.expectedDecisionReq {
				t.Errorf("DecisionNeeded = %v, expected %v", res.DecisionNeeded, tt.expectedDecisionReq)
			}
		})
	}
}

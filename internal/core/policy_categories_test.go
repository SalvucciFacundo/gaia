package core

import (
	"testing"
)

func TestPolicyCategories_ClassifyTool(t *testing.T) {
	cases := []struct {
		tool     string
		expected PolicyCategoryID
	}{
		{"file_read", CategoryFilesystem},
		{"write_file", CategoryFilesystem},
		{"shell_exec", CategoryExecution},
		{"bash", CategoryExecution},
		{"execute_code", CategoryExecution},
		{"git", CategoryExecution},
		{"web_search", CategoryNetwork},
		{"browser_open", CategoryNetwork},
		{"mem_save", CategoryState},
		{"blackboard_snapshot", CategoryState},
	}

	for _, tc := range cases {
		got := CategorizeTool(tc.tool)
		if got != tc.expected {
			t.Errorf("tool %q: expected category %s, got %s", tc.tool, tc.expected, got)
		}
	}
}

func TestPolicyCategories_SetAndGetState(t *testing.T) {
	guard := NewPolicyGuard(TierFull, nil, nil)

	// 1. Initial full tier summaries
	summaries := GetCategorySummaries(guard, []string{"shell_exec", "file_write", "web_search", "blackboard_set"})
	if len(summaries) != 4 {
		t.Fatalf("expected 4 category summaries, got %d", len(summaries))
	}

	// 2. Block Execution category
	SetCategoryState(guard, CategoryExecution, StateBlock)

	// Verify tools in execution are now blocked
	res := guard.Evaluate("shell_exec", nil)
	if res.Allowed {
		t.Errorf("expected shell_exec to be blocked")
	}
	resYaegi := guard.Evaluate("execute_code", nil)
	if resYaegi.Allowed {
		t.Errorf("expected execute_code to be blocked")
	}

	// Verify summaries reflect block in Execution
	updated := GetCategorySummaries(guard, []string{"shell_exec", "file_write"})
	for _, sum := range updated {
		if sum.ID == CategoryExecution {
			if sum.State != StateBlock {
				t.Errorf("expected Execution category to be StateBlock, got %v", sum.State)
			}
		}
	}

	// 3. Set Execution to Ask
	SetCategoryState(guard, CategoryExecution, StateAsk)
	resAsk := guard.Evaluate("shell_exec", nil)
	if resAsk.Allowed || resAsk.SuggestedAction != "ask_user" {
		t.Errorf("expected shell_exec to require ask_user, got %+v", resAsk)
	}

	// 4. Set Execution to Allow
	SetCategoryState(guard, CategoryExecution, StateAllow)
	resAllow := guard.Evaluate("shell_exec", nil)
	if !resAllow.Allowed {
		t.Errorf("expected shell_exec to be allowed, got %+v", resAllow)
	}
}

package judgment

import (
	"os"
	"path/filepath"
	"testing"

	"gaia/internal/core/domain"
)

func TestCompareFindingsDedup(t *testing.T) {
	a := []domain.ReviewFinding{
		{Lens: "judge-a", Severity: "BLOCKER", File: "main.go", Line: 10, Message: "nil pointer deref", Suggestion: "add nil check"},
	}
	b := []domain.ReviewFinding{
		{Lens: "judge-b", Severity: "BLOCKER", File: "main.go", Line: 10, Message: "nil pointer deref", Suggestion: "add nil check"},
	}

	merged, err := CompareFindings(a, b)
	if err != nil {
		t.Fatalf("CompareFindings: %v", err)
	}
	if len(merged) != 1 {
		t.Errorf("expected 1 merged finding, got %d", len(merged))
	}
	if merged[0].Severity != "BLOCKER" {
		t.Errorf("expected BLOCKER, got %s", merged[0].Severity)
	}
}

func TestCompareFindingsHigherSeverity(t *testing.T) {
	a := []domain.ReviewFinding{
		{Lens: "judge-a", Severity: "WARNING", File: "auth.go", Line: 42, Message: "missing error check", Suggestion: "add if err != nil"},
	}
	b := []domain.ReviewFinding{
		{Lens: "judge-b", Severity: "BLOCKER", File: "auth.go", Line: 42, Message: "panic on nil input", Suggestion: "validate input"},
	}

	merged, err := CompareFindings(a, b)
	if err != nil {
		t.Fatalf("CompareFindings: %v", err)
	}
	if len(merged) != 1 {
		t.Errorf("expected 1 merged finding, got %d", len(merged))
	}
	if merged[0].Severity != "BLOCKER" {
		t.Errorf("expected BLOCKER (higher severity wins), got %s", merged[0].Severity)
	}
}

func TestCompareFindingsUnique(t *testing.T) {
	a := []domain.ReviewFinding{
		{Lens: "judge-a", Severity: "BLOCKER", File: "auth.go", Line: 10, Message: "secret in code"},
	}
	b := []domain.ReviewFinding{
		{Lens: "judge-b", Severity: "WARNING", File: "main.go", Line: 50, Message: "missing context cancel"},
	}

	merged, err := CompareFindings(a, b)
	if err != nil {
		t.Fatalf("CompareFindings: %v", err)
	}
	if len(merged) != 2 {
		t.Errorf("expected 2 unique findings, got %d", len(merged))
	}
}

func TestCompareFindingsEmpty(t *testing.T) {
	merged, err := CompareFindings(nil, nil)
	if err != nil {
		t.Fatalf("CompareFindings: %v", err)
	}
	if len(merged) != 0 {
		t.Errorf("expected 0 findings, got %d", len(merged))
	}
}

func TestParseJudgeResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		judge    string
		wantLen  int
	}{
		{
			name:     "FINDINGS: NONE",
			response: "FINDINGS: NONE",
			judge:    "judge-a",
			wantLen:  0,
		},
		{
			name: "single finding",
			response: `FINDING: BLOCKER main.go:42 nil pointer dereference | SUGGESTION: add nil check`,
			judge:  "judge-a",
			wantLen: 1,
		},
		{
			name: "multiple findings",
			response: `FINDING: WARNING auth.go:10 secret in code | SUGGESTION: use env var
FINDING: SUGGESTION main.go:5 missing doc comment`,
			judge:  "judge-b",
			wantLen: 2,
		},
		{
			name: "severity prefix without FINDING",
			response: `BLOCKER: main.go:42 nil pointer | SUGGESTION: add check`,
			judge:  "judge-a",
			wantLen: 1,
		},
		{
			name:     "no findings",
			response: "Some random text with no finding lines.",
			judge:    "judge-b",
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := parseJudgeResponse(tt.response, tt.judge)
			if len(findings) != tt.wantLen {
				t.Errorf("parseJudgeResponse(): got %d findings, want %d", len(findings), tt.wantLen)
				for i, f := range findings {
					t.Logf("  finding[%d]: lens=%s sev=%s file=%s line=%d msg=%s", i, f.Lens, f.Severity, f.File, f.Line, f.Message)
				}
			}
		})
	}
}

func TestClassifyFindings(t *testing.T) {
	findings := []domain.ReviewFinding{
		{Severity: "BLOCKER", File: "main.go", Line: 10},
		{Severity: "WARNING", File: "auth.go", Line: 20},
		{Severity: "SUGGESTION", File: "README.md", Line: 5},
		{Severity: "BLOCKER", File: "db.go", Line: 30},
	}

	blockers, warnings := classifyFindings(findings)
	if len(blockers) != 2 {
		t.Errorf("expected 2 blockers, got %d", len(blockers))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestFilterFixable(t *testing.T) {
	findings := []domain.ReviewFinding{
		{Severity: "BLOCKER", File: "main.go", Line: 10},
		{Severity: "WARNING", File: "auth.go", Line: 20},
		{Severity: "SUGGESTION", File: "README.md", Line: 5},
	}

	fixable := filterFixable(findings)
	if len(fixable) != 2 {
		t.Errorf("expected 2 fixable (BLOCKER+WARNING), got %d", len(fixable))
	}
	if fixable[0].Severity != "BLOCKER" {
		t.Errorf("first fixable should be BLOCKER")
	}
	if fixable[1].Severity != "WARNING" {
		t.Errorf("second fixable should be WARNING")
	}
}

func TestApplyFixesSingleFinding(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	original := "package main\nfunc main() { x := nil; fmt.Println(*x) }\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Change to the temp directory so ApplyFixes can find the file.
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	findings := []domain.ReviewFinding{
		{
			Severity:   "BLOCKER",
			File:       "main.go",
			Line:       2,
			Message:    "nil pointer deref",
			Suggestion: "func main() { var x *int; if x != nil { fmt.Println(*x) } }",
		},
	}

	result, err := ApplyFixes(nil, nil, findings, 85)
	if err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}
	if result.AppliedFixCount != 1 {
		t.Errorf("expected 1 fix applied, got %d", result.AppliedFixCount)
	}
	if result.SkippedCount != 0 {
		t.Errorf("expected 0 skipped, got %d", result.SkippedCount)
	}

	// Verify file was modified.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read modified file: %v", err)
	}
	modified := string(data)
	if modified == original {
		t.Errorf("file was not modified by fix")
	}
}

func TestApplyFixesSkipsSuggestion(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	original := "package main\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	findings := []domain.ReviewFinding{
		{
			Severity:   "SUGGESTION",
			File:       "main.go",
			Line:       1,
			Message:    "add doc comment",
			Suggestion: "// Package main is the entry point.",
		},
	}

	result, err := ApplyFixes(nil, nil, findings, 85)
	if err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}
	if result.SkippedCount != 1 {
		t.Errorf("expected 1 skipped (SUGGESTION), got %d", result.SkippedCount)
	}
	if result.AppliedFixCount != 0 {
		t.Errorf("expected 0 applied, got %d", result.AppliedFixCount)
	}
}

func TestMaxRounds(t *testing.T) {
	jd := NewJudgmentDay(nil)
	if jd.maxRounds != MaxRounds {
		t.Errorf("default maxRounds: got %d, want %d", jd.maxRounds, MaxRounds)
	}

	jd.SetMaxRounds(3)
	if jd.maxRounds != 3 {
		t.Errorf("after SetMaxRounds(3): got %d, want 3", jd.maxRounds)
	}

	// Negative values should not change.
	jd.SetMaxRounds(0)
	if jd.maxRounds != 3 {
		t.Errorf("SetMaxRounds(0) should not change: got %d, want 3", jd.maxRounds)
	}
	jd.SetMaxRounds(-1)
	if jd.maxRounds != 3 {
		t.Errorf("SetMaxRounds(-1) should not change: got %d, want 3", jd.maxRounds)
	}
}

func TestJudgeSystemPrompts(t *testing.T) {
	promptA := judgeASystemPrompt()
	promptB := judgeBSystemPrompt()

	if promptA == "" {
		t.Error("judgeA prompt is empty")
	}
	if promptB == "" {
		t.Error("judgeB prompt is empty")
	}
	if promptA == promptB {
		t.Error("judge A and B prompts should be different")
	}
	if !contains(promptA, "SECURITY") {
		t.Error("judge A prompt should mention SECURITY")
	}
	if !contains(promptB, "RESILIENCE") {
		t.Error("judge B prompt should mention RESILIENCE")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

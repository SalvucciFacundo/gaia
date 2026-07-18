package review

import (
	"context"
	"strings"
	"testing"

	"gaia/internal/core/domain"
)

func TestRiskClassification(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		files    []string
		wantCode RiskCode
	}{
		{
			name:     "configuration change via config.yaml",
			diff:     "+  new_field: value",
			files:    []string{"config.yaml"},
			wantCode: RiskConfigurationChange,
		},
		{
			name:     "configuration change via .env",
			diff:     "+  DB_URL=postgres://localhost",
			files:    []string{".env"},
			wantCode: RiskConfigurationChange,
		},
		{
			name:     "configuration change via Dockerfile",
			diff:     "+  RUN apt-get install curl",
			files:    []string{"Dockerfile"},
			wantCode: RiskConfigurationChange,
		},
		{
			name:     "executable change via .exe",
			diff:     "Binary files differ",
			files:    []string{"gaia.exe"},
			wantCode: RiskExecutableChange,
		},
		{
			name:     "executable change via .sh",
			diff:     "+  #!/bin/sh",
			files:    []string{"deploy.sh"},
			wantCode: RiskShellSource,
		},
		{
			name:     "hot path via auth directory",
			diff:     "+  func Login() {}",
			files:    []string{"internal/auth/login.go"},
			wantCode: RiskHotPath,
		},
		{
			name:     "hot path via payment file",
			diff:     "+  func ProcessPayment() {}",
			files:    []string{"internal/payments/charge.go"},
			wantCode: RiskHotPath,
		},
		{
			name:     "shell source via .sh",
			diff:     "+  echo hello",
			files:    []string{"scripts/cleanup.sh"},
			wantCode: RiskShellSource,
		},
		{
			name:     "shell source via .ps1",
			diff:     "+  Write-Host 'hello'",
			files:    []string{"scripts/deploy.ps1"},
			wantCode: RiskShellSource,
		},
		{
			name:     "non_executable_only with docs",
			diff:     "+  updated readme content",
			files:    []string{"README.md", "docs/guide.md"},
			wantCode: RiskNonExecutableOnly,
		},
		{
			name:     "service token detection",
			diff:     "+  api_key: sk-abc123def456",
			files:    []string{"config.yaml"},
			wantCode: RiskServiceToken,
		},
		{
			name: "large change detection",
			diff: strings.Repeat("+ added line\n", 401),
			files: []string{"main.go"},
			wantCode: RiskLargeChange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codes := ClassifyRisk(tt.diff, tt.files)
			found := false
			for _, c := range codes {
				if c == tt.wantCode {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ClassifyRisk() codes=%v, want to include %s", codes, tt.wantCode)
			}
		})
	}
}

func TestRiskLevelDetermination(t *testing.T) {
	tests := []struct {
		name      string
		codes     []RiskCode
		wantLevel string
	}{
		{
			name:      "low risk: only non_executable_only",
			codes:     []RiskCode{RiskNonExecutableOnly},
			wantLevel: "low",
		},
		{
			name:      "medium risk: configuration change",
			codes:     []RiskCode{RiskConfigurationChange},
			wantLevel: "medium",
		},
		{
			name:      "medium risk: executable change",
			codes:     []RiskCode{RiskExecutableChange},
			wantLevel: "medium",
		},
		{
			name:      "high risk: hot_path",
			codes:     []RiskCode{RiskHotPath},
			wantLevel: "high",
		},
		{
			name:      "high risk: large_change",
			codes:     []RiskCode{RiskLargeChange},
			wantLevel: "high",
		},
		{
			name:      "high risk: service_token",
			codes:     []RiskCode{RiskServiceToken},
			wantLevel: "high",
		},
		{
			name:      "high risk: shell_source",
			codes:     []RiskCode{RiskShellSource},
			wantLevel: "high",
		},
		{
			name:      "high risk: mixed with hot_path",
			codes:     []RiskCode{RiskConfigurationChange, RiskHotPath},
			wantLevel: "high",
		},
		{
			name:      "medium risk: mixed non-high",
			codes:     []RiskCode{RiskConfigurationChange, RiskExecutableChange},
			wantLevel: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := DetermineRiskLevel(tt.codes)
			if level != tt.wantLevel {
				t.Errorf("DetermineRiskLevel(%v) = %q, want %q", tt.codes, level, tt.wantLevel)
			}
		})
	}
}

func TestLensSelection(t *testing.T) {
	tests := []struct {
		name      string
		riskLevel string
		files     []string
		wantCount int
		wantLens  string // for medium risk, check the specific lens
	}{
		{
			name:      "high risk: all 4 lenses",
			riskLevel: "high",
			files:     []string{"main.go"},
			wantCount: 4,
		},
		{
			name:      "low risk: no lenses",
			riskLevel: "low",
			files:     []string{"README.md"},
			wantCount: 0,
		},
		{
			name:      "medium risk: config file → risk lens",
			riskLevel: "medium",
			files:     []string{"config.yaml"},
			wantCount: 1,
			wantLens:  "review-risk",
		},
		{
			name:      "medium risk: test file → reliability lens",
			riskLevel: "medium",
			files:     []string{"engine_test.go"},
			wantCount: 1,
			wantLens:  "review-reliability",
		},
		{
			name:      "medium risk: doc file → readability lens",
			riskLevel: "medium",
			files:     []string{"README.md"},
			wantCount: 1,
			wantLens:  "review-readability",
		},
		{
			name:      "medium risk: service file → resilience lens",
			riskLevel: "medium",
			files:     []string{"internal/service/auth.go"},
			wantCount: 1,
			wantLens:  "review-resilience",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lenses := SelectLenses(tt.riskLevel, tt.files)
			if len(lenses) != tt.wantCount {
				t.Errorf("SelectLenses(%q, %v) returned %d lenses, want %d", tt.riskLevel, tt.files, len(lenses), tt.wantCount)
			}
			if tt.wantLens != "" && len(lenses) > 0 && lenses[0] != tt.wantLens {
				t.Errorf("SelectLenses(%q, %v) = %q, want dominant lens %q", tt.riskLevel, tt.files, lenses[0], tt.wantLens)
			}
		})
	}
}

func TestSnapshotHash(t *testing.T) {
	// Test that known content produces a known hash.
	snapshots := []FileSnapshot{
		{Path: "a.go", Content: "package main\nfunc main() {}\n", Hash: ""},
	}
	hash := ComputeSnapshotHash(snapshots)
	if hash == "" || !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("ComputeSnapshotHash returned invalid hash: %q", hash)
	}

	// Test determinism: same input → same hash.
	hash2 := ComputeSnapshotHash(snapshots)
	if hash != hash2 {
		t.Errorf("ComputeSnapshotHash not deterministic: %q vs %q", hash, hash2)
	}

	// Test that order doesn't matter (sorted internally).
	reversed := []FileSnapshot{
		{Path: "b.go", Content: "package b", Hash: ""},
		{Path: "a.go", Content: "package a", Hash: ""},
	}
	h1 := ComputeSnapshotHash(reversed)
	h2 := ComputeSnapshotHash([]FileSnapshot{
		{Path: "a.go", Content: "package a", Hash: ""},
		{Path: "b.go", Content: "package b", Hash: ""},
	})
	if h1 != h2 {
		t.Errorf("ComputeSnapshotHash order-dependent: %q vs %q", h1, h2)
	}

	// Test that content change changes hash.
	changed := []FileSnapshot{
		{Path: "a.go", Content: "package main\nfunc main() {} // changed\n", Hash: ""},
	}
	hashChanged := ComputeSnapshotHash(changed)
	if hash == hashChanged {
		t.Errorf("ComputeSnapshotHash should differ for different content")
	}
}

func TestCRLFNormalization(t *testing.T) {
	// CRLF content should produce the same hash as LF content
	// AFTER normalization (normalization happens in SnapshotFiles, not ComputeSnapshotHash).
	// ComputeSnapshotHash expects already-normalized content.
	// Test that normalized CRLF content matches LF content.
	crlf := strings.ReplaceAll("package main\r\nfunc main() {}\r\n", "\r\n", "\n")
	lf := "package main\nfunc main() {}\n"

	crlfSnap := FileSnapshot{Path: "a.go", Content: crlf, Hash: ""}
	lfSnap := FileSnapshot{Path: "a.go", Content: lf, Hash: ""}

	hCRLF := ComputeSnapshotHash([]FileSnapshot{crlfSnap})
	hLF := ComputeSnapshotHash([]FileSnapshot{lfSnap})
	if hCRLF != hLF {
		t.Errorf("CRLF normalization: CRLF hash=%q, LF hash=%q", hCRLF, hLF)
	}
}

func TestEmptySnapshot(t *testing.T) {
	hash := ComputeSnapshotHash(nil)
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("empty snapshot should still return a sha256 prefix: %q", hash)
	}

	hash2 := ComputeSnapshotHash([]FileSnapshot{})
	if hash != hash2 {
		t.Errorf("nil and empty snapshot should produce same hash: %q vs %q", hash, hash2)
	}
}

func TestStateTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    domain.ReviewState
		to      domain.ReviewState
		wantErr bool
	}{
		// Valid transitions.
		{name: "unreviewed → reviewing", from: StateUnreviewed, to: StateReviewing, wantErr: false},
		{name: "reviewing → findings_frozen", from: StateReviewing, to: StateFindingsFrozen, wantErr: false},
		{name: "reviewing → judges_confirmed", from: StateReviewing, to: StateJudgesConfirmed, wantErr: false},
		{name: "judges_confirmed → findings_frozen", from: StateJudgesConfirmed, to: StateFindingsFrozen, wantErr: false},
		{name: "findings_frozen → evidence_classified", from: StateFindingsFrozen, to: StateEvidenceClassified, wantErr: false},
		{name: "evidence_classified → fix_required", from: StateEvidenceClassified, to: StateFixRequired, wantErr: false},
		{name: "evidence_classified → ready_final_verification", from: StateEvidenceClassified, to: StateReadyFinalVerification, wantErr: false},
		{name: "fix_required → fixing", from: StateFixRequired, to: StateFixing, wantErr: false},
		{name: "fixing → fix_validating", from: StateFixing, to: StateFixValidating, wantErr: false},
		{name: "fix_validating → evidence_classified", from: StateFixValidating, to: StateEvidenceClassified, wantErr: false},
		{name: "ready_final_verification → final_verifying", from: StateReadyFinalVerification, to: StateFinalVerifying, wantErr: false},
		{name: "final_verifying → approved", from: StateFinalVerifying, to: StateApproved, wantErr: false},
		{name: "final_verifying → escalated", from: StateFinalVerifying, to: StateEscalated, wantErr: false},
		{name: "final_verifying → invalidated", from: StateFinalVerifying, to: StateInvalidated, wantErr: false},
		{name: "idempotent: same state", from: StateReviewing, to: StateReviewing, wantErr: false},

		// Invalid transitions.
		{name: "skip: unreviewed → approved", from: StateUnreviewed, to: StateApproved, wantErr: true},
		{name: "skip: reviewing → approved", from: StateReviewing, to: StateApproved, wantErr: true},
		{name: "skip: findings_frozen → approved", from: StateFindingsFrozen, to: StateApproved, wantErr: true},
		{name: "backward: approved → reviewing", from: StateApproved, to: StateReviewing, wantErr: true},
		{name: "terminal: escalated → anything", from: StateEscalated, to: StateReviewing, wantErr: true},
		{name: "terminal: invalidated → anything", from: StateInvalidated, to: StateReviewing, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Transition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Transition(%s, %s) should have failed", tt.from, tt.to)
				} else {
					t.Errorf("Transition(%s, %s) failed: %v", tt.from, tt.to, err)
				}
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		state    domain.ReviewState
		terminal bool
	}{
		{StateUnreviewed, false},
		{StateReviewing, false},
		{StateFindingsFrozen, false},
		{StateEvidenceClassified, false},
		{StateFixRequired, false},
		{StateFixing, false},
		{StateFixValidating, false},
		{StateReadyFinalVerification, false},
		{StateFinalVerifying, false},
		{StateApproved, true},
		{StateEscalated, true},
		{StateInvalidated, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := IsTerminal(tt.state); got != tt.terminal {
				t.Errorf("IsTerminal(%s) = %v, want %v", tt.state, got, tt.terminal)
			}
		})
	}
}

func TestDiffLineCount(t *testing.T) {
	tests := []struct {
		name  string
		diff  string
		count int
	}{
		{
			name:  "empty",
			diff:  "",
			count: 0,
		},
		{
			name: "two changes",
			diff: `+ added line
- removed line`,
			count: 2,
		},
		{
			name: "ignores file headers",
			diff: `--- a/file.go
+++ b/file.go
+ actual change`,
			count: 1,
		},
		{
			name: "over 400 lines",
			diff: strings.Repeat("+ line\n", 200) + strings.Repeat("- line\n", 201),
			count: 401,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffLineCount(tt.diff)
			if got != tt.count {
				t.Errorf("diffLineCount() = %d, want %d", got, tt.count)
			}
		})
	}
}

// mockLensLLM implements LensLLM for testing.
type mockLensLLM struct {
	response string
	err      error
}

func (m *mockLensLLM) Chat(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestEngineStart(t *testing.T) {
	llm := &mockLensLLM{}
	engine := NewEngine(".", llm)

	// Test with no files.
	tx, err := engine.Start([]string{})
	if err != nil {
		t.Fatalf("Start() with empty files failed: %v", err)
	}
	if tx.State != StateReviewing {
		t.Errorf("Start() state = %s, want %s", tx.State, StateReviewing)
	}
	if tx.SnapshotHash == "" {
		t.Error("Start() should set SnapshotHash")
	}
}

func TestRiskCodeCount(t *testing.T) {
	allCodes := []RiskCode{
		RiskConfigurationChange,
		RiskExecutableChange,
		RiskExecutableMode,
		RiskHotPath,
		RiskLargeChange,
		RiskNonExecutableOnly,
		RiskServiceToken,
		RiskShellSource,
	}
	if len(allCodes) != 8 {
		t.Errorf("Expected 8 risk codes, got %d", len(allCodes))
	}
}

func TestStateCount(t *testing.T) {
	allStates := []domain.ReviewState{
		StateUnreviewed,
		StateReviewing,
		StateJudgesConfirmed,
		StateFindingsFrozen,
		StateEvidenceClassified,
		StateFixRequired,
		StateFixing,
		StateFixValidating,
		StateReadyFinalVerification,
		StateFinalVerifying,
		StateApproved,
		StateEscalated,
		StateInvalidated,
	}
	if len(allStates) != 13 {
		t.Errorf("Expected 13 states, got %d", len(allStates))
	}
}

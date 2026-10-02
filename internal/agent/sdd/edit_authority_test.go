package sdd

import (
	"context"
	"strings"
	"testing"
)

func TestValidateEditRoots(t *testing.T) {
	tests := []struct {
		name    string
		roots   []string
		wantErr bool
	}{
		{
			name:    "valid relative roots",
			roots:   []string{"internal/review/", "internal/agent/ops/"},
			wantErr: false,
		},
		{
			name:    "empty roots list",
			roots:   []string{},
			wantErr: true,
		},
		{
			name:    "dot wildcard rejected",
			roots:   []string{"."},
			wantErr: true,
		},
		{
			name:    "bare slash root rejected",
			roots:   []string{"/"},
			wantErr: true,
		},
		{
			name:    "absolute path rejected",
			roots:   []string{"/home/user/project/internal"},
			wantErr: true,
		},
		{
			name:    "path traversal rejected",
			roots:   []string{"../parent/dir"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEditRoots(tt.roots)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEditRoots(%v) err = %v, wantErr = %v", tt.roots, err, tt.wantErr)
			}
		})
	}
}

func TestIsPathWithinRoots(t *testing.T) {
	allowed := []string{"internal/review/", "internal/agent/ops/"}

	tests := []struct {
		path    string
		allowed bool
	}{
		{"internal/review/engine.go", true},
		{"internal/review/judgment/fix.go", true},
		{"internal/agent/ops/reviewer.go", true},
		{"cmd/gaia/main.go", false},
		{"internal/review_extended.go", false}, // boundary check: must not match prefix without slash
		{"README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsPathWithinRoots(tt.path, allowed)
			if got != tt.allowed {
				t.Errorf("IsPathWithinRoots(%q, %v) = %v, want %v", tt.path, allowed, got, tt.allowed)
			}
		})
	}
}

func TestDeriveMissingRoots(t *testing.T) {
	allowed := []string{"internal/review/"}
	targets := []string{
		"internal/review/engine.go",
		"cmd/gaia/main.go",
		"pkg/util/helper.go",
		"cmd/gaia/cli.go",
	}

	missing := DeriveMissingRoots(targets, allowed)
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing root directories, got %d: %v", len(missing), missing)
	}

	hasCmd := false
	hasPkg := false
	for _, m := range missing {
		if strings.Contains(m, "cmd/gaia") {
			hasCmd = true
		}
		if strings.Contains(m, "pkg/util") {
			hasPkg = true
		}
	}
	if !hasCmd || !hasPkg {
		t.Errorf("missing roots should contain cmd/gaia and pkg/util, got %v", missing)
	}
}

func TestEvaluateEditAuthority(t *testing.T) {
	allowed := []string{"internal/review/"}

	// 1. Authorized paths -> returns nil
	consentNil := EvaluateEditAuthority([]string{"internal/review/engine.go"}, allowed)
	if consentNil != nil {
		t.Errorf("expected nil consent for authorized paths, got: %+v", consentNil)
	}

	// 2. Unauthorized paths -> returns typed consent envelope
	targets := []string{"internal/review/engine.go", "cmd/gaia/main.go"}
	consent := EvaluateEditAuthority(targets, allowed)
	if consent == nil {
		t.Fatal("expected non-nil consent envelope for unauthorized paths")
	}

	if consent.Schema != "gaia.sdd.consent/v1" {
		t.Errorf("expected schema 'gaia.sdd.consent/v1', got %q", consent.Schema)
	}
	if len(consent.MissingRoots) != 1 {
		t.Errorf("expected 1 missing root, got %d", len(consent.MissingRoots))
	}
	if len(consent.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(consent.Choices))
	}
	if consent.Choices[0].Token != "granted" || consent.Choices[1].Token != "declined" {
		t.Errorf("choices tokens should be 'granted' and 'declined', got %+v", consent.Choices)
	}
}

func TestAttemptLedger_EditAuthorityEnforcement(t *testing.T) {
	ledger := NewAttemptLedger(nil, nil)
	ctx := context.Background()

	change := "feat-auth"
	workUnit := "task-1"
	allowed := []string{"internal/auth/"}

	req := AcquireRequest{
		ChangeName:       change,
		WorkUnit:         workUnit,
		SubagentName:     "implementer",
		EvidenceGoal:     "implement auth login",
		AllowedEditRoots: allowed,
		MaxAttempts:      3,
		MaxChangedLines:  100,
	}

	// Initial acquire should proceed
	resp, err := ledger.Acquire(ctx, req)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if resp.State != StateProceed {
		t.Fatalf("expected StateProceed, got %s", resp.State)
	}

	// Check unauthorized file edit
	unauthorizedTargets := []string{"cmd/gaia/main.go"}
	consent := ledger.CheckEditAuthority(change, workUnit, unauthorizedTargets)
	if consent == nil {
		t.Fatal("expected consent envelope for unauthorized file edit")
	}

	// Grant edit authority for missing roots
	err = ledger.GrantEditRoots(change, workUnit, consent.MissingRoots)
	if err != nil {
		t.Fatalf("GrantEditRoots failed: %v", err)
	}

	// Re-check: now it should be authorized
	consentAfterGrant := ledger.CheckEditAuthority(change, workUnit, unauthorizedTargets)
	if consentAfterGrant != nil {
		t.Errorf("expected nil consent after grant, got %+v", consentAfterGrant)
	}
}

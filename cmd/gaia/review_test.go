package main

import (
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/review"
	"gaia/internal/review/gates"
)

func TestReviewCLIModeHandling(t *testing.T) {
	tempDir := t.TempDir()

	// Initially disabled
	status := review.IsEnabled(tempDir)
	if status.Mode != review.ModeDisabled {
		t.Fatalf("expected disabled, got %s", status.Mode)
	}

	// Test enable
	if err := review.SetMode(review.ModeEnabled, review.ScopeClone, tempDir); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}
	status = review.IsEnabled(tempDir)
	if status.Mode != review.ModeEnabled {
		t.Errorf("expected enabled, got %s", status.Mode)
	}

	// Test disable
	if err := review.SetMode(review.ModeDisabled, review.ScopeClone, tempDir); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}
	status = review.IsEnabled(tempDir)
	if status.Mode != review.ModeDisabled {
		t.Errorf("expected disabled, got %s", status.Mode)
	}
}

func TestReviewCLIStatusAndListFiltering(t *testing.T) {
	tempDir := t.TempDir()
	store := gates.NewCASReceiptStore(tempDir)

	receipt1 := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "lin1",
		SnapshotHash: "sha256:hash1",
		State:        domain.ReviewStateApproved,
		RiskLevel:    "low",
	}
	receipt2 := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "lin2",
		SnapshotHash: "sha256:hash2",
		State:        domain.ReviewStateEscalated,
		RiskLevel:    "high",
	}

	if err := store.SaveReceipt(receipt1, "change-auth"); err != nil {
		t.Fatalf("save receipt1: %v", err)
	}
	if err := store.SaveReceipt(receipt2, "change-billing"); err != nil {
		t.Fatalf("save receipt2: %v", err)
	}

	// Test status --change lookup
	r, err := store.LatestReceipt("change-auth")
	if err != nil || r == nil {
		t.Fatalf("failed to find receipt for change-auth: %v", err)
	}
	if r.State != domain.ReviewStateApproved {
		t.Errorf("expected approved, got %s", r.State)
	}

	// Test list filtering by state
	summaries, err := store.ListReceipts()
	if err != nil {
		t.Fatalf("ListReceipts failed: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 receipts, got %d", len(summaries))
	}

	var approved []gates.ReceiptSummary
	for _, s := range summaries {
		if s.State == string(domain.ReviewStateApproved) {
			approved = append(approved, s)
		}
	}
	if len(approved) != 1 || approved[0].ChangeName != "change-auth" {
		t.Errorf("expected 1 approved receipt for change-auth, got %v", approved)
	}
}

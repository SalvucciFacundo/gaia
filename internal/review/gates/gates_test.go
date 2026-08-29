package gates

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/review"
)

func TestGateValidation_DisabledReviewMode_BypassesGate(t *testing.T) {
	dir := t.TempDir()
	// Review mode is default disabled
	store := &memReceiptStore{receipt: nil} // No receipt
	gate := GatePreCommit

	result, err := gate.Validate(dir, []string{"test.go"}, store)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected gate to pass when review mode is disabled, got: %s", result.Reason)
	}
}

func TestGateValidationPass(t *testing.T) {
	dir := t.TempDir()
	_ = review.SetMode(review.ModeEnabled, review.ScopeClone, dir)

	filePath := filepath.Join(dir, "test.go")
	content := "package main\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Compute snapshot hash.
	snapshots, err := review.SnapshotFiles(dir, []string{"test.go"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshotHash := review.ComputeSnapshotHash(snapshots)

	// Create an approved receipt with the matching hash.
	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "test-lineage-id",
		SnapshotHash: snapshotHash,
		State:        domain.ReviewStateApproved,
		RiskLevel:    "low",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	store := &memReceiptStore{receipt: receipt}
	gate := GatePreCommit

	result, err := gate.Validate(dir, []string{"test.go"}, store)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected gate to pass, got: %s", result.Reason)
	}
}

func TestGateValidationFailContentChanged(t *testing.T) {
	dir := t.TempDir()
	_ = review.SetMode(review.ModeEnabled, review.ScopeClone, dir)

	filePath := filepath.Join(dir, "test.go")
	content := "package main\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Compute hash with different content than what's on disk.
	snapshots, err := review.SnapshotFiles(dir, []string{"test.go"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	_ = review.ComputeSnapshotHash(snapshots) // current hash

	// Create receipt with a DIFFERENT hash (simulating content change).
	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "test-lineage-id",
		SnapshotHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		State:        domain.ReviewStateApproved,
		RiskLevel:    "low",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	store := &memReceiptStore{receipt: receipt}
	gate := GatePreCommit

	result, err := gate.Validate(dir, []string{"test.go"}, store)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Passed {
		t.Error("expected gate to fail when content changed")
	}
}

func TestGateValidationNoReceipt(t *testing.T) {
	dir := t.TempDir()
	_ = review.SetMode(review.ModeEnabled, review.ScopeClone, dir)

	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	store := &memReceiptStore{receipt: nil} // no receipt
	gate := GatePreCommit

	result, err := gate.Validate(dir, []string{"test.go"}, store)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Passed {
		t.Error("expected gate to fail when no receipt exists")
	}
}

func TestGateValidationWrongState(t *testing.T) {
	dir := t.TempDir()
	_ = review.SetMode(review.ModeEnabled, review.ScopeClone, dir)

	filePath := filepath.Join(dir, "test.go")
	content := "package main\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	snapshots, err := review.SnapshotFiles(dir, []string{"test.go"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshotHash := review.ComputeSnapshotHash(snapshots)

	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "test-lineage-id",
		SnapshotHash: snapshotHash,
		State:        domain.ReviewStateReviewing, // not approved
		RiskLevel:    "low",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	store := &memReceiptStore{receipt: receipt}
	gate := GatePreCommit

	result, err := gate.Validate(dir, []string{"test.go"}, store)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Passed {
		t.Error("expected gate to fail when receipt is not approved")
	}
}

func TestGateValidationUnreviewedFiles(t *testing.T) {
	dir := t.TempDir()
	_ = review.SetMode(review.ModeEnabled, review.ScopeClone, dir)

	// Create file a.go and file b.go
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Receipt only covers a.go
	snapshots, err := review.SnapshotFiles(dir, []string{"a.go"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshotHash := review.ComputeSnapshotHash(snapshots)

	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "test-lineage-id",
		SnapshotHash: snapshotHash,
		State:        domain.ReviewStateApproved,
		RiskLevel:    "low",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	store := &memReceiptStore{receipt: receipt}
	gate := GatePreCommit

	// Validate both files -> hash mismatch because b.go wasn't in receipt
	result, err := gate.Validate(dir, []string{"a.go", "b.go"}, store)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Passed {
		t.Error("expected gate to fail when unreviewed files are included")
	}
}

func TestGatePrePushValidate(t *testing.T) {
	dir := t.TempDir()
	_ = review.SetMode(review.ModeEnabled, review.ScopeClone, dir)

	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	snapshots, err := review.SnapshotFiles(dir, []string{"main.go"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshotHash := review.ComputeSnapshotHash(snapshots)

	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "push-lineage",
		SnapshotHash: snapshotHash,
		State:        domain.ReviewStateApproved,
		RiskLevel:    "medium",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	store := &memReceiptStore{receipt: receipt}
	result, err := GatePrePush.Validate(dir, []string{"main.go"}, store)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pre-push to pass, got: %s", result.Reason)
	}
}

func TestGatePrePRValidate(t *testing.T) {
	dir := t.TempDir()
	_ = review.SetMode(review.ModeEnabled, review.ScopeClone, dir)

	filePath := filepath.Join(dir, "api.go")
	if err := os.WriteFile(filePath, []byte("package api"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	snapshots, err := review.SnapshotFiles(dir, []string{"api.go"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshotHash := review.ComputeSnapshotHash(snapshots)

	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "pr-lineage",
		SnapshotHash: snapshotHash,
		State:        domain.ReviewStateApproved,
		RiskLevel:    "high",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	store := &memReceiptStore{receipt: receipt}
	result, err := GatePrePR.Validate(dir, []string{"api.go"}, store)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pre-pr to pass, got: %s", result.Reason)
	}
}

// memReceiptStore is an in-memory ReceiptStore for testing.
type memReceiptStore struct {
	receipt *domain.ReviewReceipt
}

func (m *memReceiptStore) LatestReceipt(changeName string) (*domain.ReviewReceipt, error) {
	return m.receipt, nil
}

func (m *memReceiptStore) SaveReceipt(receipt *domain.ReviewReceipt, changeName string) error {
	m.receipt = receipt
	return nil
}

func (m *memReceiptStore) ListReceipts() ([]ReceiptSummary, error) {
	if m.receipt == nil {
		return nil, nil
	}
	return []ReceiptSummary{
		{
			ChangeName: "test-change",
			State:      string(m.receipt.State),
			RiskLevel:  m.receipt.RiskLevel,
			CreatedAt:  m.receipt.CreatedAt,
		},
	}, nil
}

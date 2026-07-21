package gates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/review"
)

func TestGateValidationPass(t *testing.T) {
	// Create a temp directory with a test file.
	dir := t.TempDir()
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
	// Validate should return nil error and non-passed result for missing receipt.
}

func TestGateValidationWrongState(t *testing.T) {
	dir := t.TempDir()
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
		State:        domain.ReviewStateEscalated, // NOT approved
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
		t.Error("expected gate to fail when receipt is escalated (not approved)")
	}
}

func TestHookGeneration(t *testing.T) {
	hook := PreCommitHook
	script := hook.scriptContent()

	// Verify the generated script contains expected elements.
	if !contains(script, "GAIA Review Gate") {
		t.Error("hook script missing GAIA marker")
	}
	if !contains(script, "pre-commit") {
		t.Error("hook script missing gate name")
	}
	if !contains(script, "review validate") {
		t.Error("hook script missing validate command")
	}
	if !contains(script, "--gate") {
		t.Error("hook script missing --gate flag")
	}

	hook2 := PrePushHook
	script2 := hook2.scriptContent()
	if !contains(script2, "pre-push") {
		t.Error("pre-push hook script missing gate name")
	}
}

func TestHookAppend(t *testing.T) {
	dir := t.TempDir()
	hookDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	existingContent := "#!/bin/sh\necho 'existing hook'\n"
	hookPath := filepath.Join(hookDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(existingContent), 0755); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	// Install hooks — should append GAIA content.
	if err := WriteHooks(dir); err != nil {
		t.Fatalf("WriteHooks: %v", err)
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}

	content := string(data)
	if !contains(content, existingContent) {
		t.Error("existing hook content was not preserved")
	}
	if !contains(content, "GAIA Review Gate") {
		t.Error("GAIA hook content was not appended")
	}
}

func TestHookIdempotent(t *testing.T) {
	dir := t.TempDir()
	hookDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	// First install.
	if err := WriteHooks(dir); err != nil {
		t.Fatalf("first WriteHooks: %v", err)
	}

	data1, _ := os.ReadFile(filepath.Join(hookDir, "pre-commit"))

	// Second install should be idempotent.
	if err := WriteHooks(dir); err != nil {
		t.Fatalf("second WriteHooks: %v", err)
	}

	data2, _ := os.ReadFile(filepath.Join(hookDir, "pre-commit"))

	if string(data1) != string(data2) {
		t.Error("WriteHooks is not idempotent — second call changed content")
	}
}

func TestFSReceiptStore(t *testing.T) {
	dir := t.TempDir()
	store := &FSReceiptStore{reviewDir: filepath.Join(dir, ".gaia", "reviews")}

	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "abc123def4567890",
		SnapshotHash: "sha256:test",
		State:        domain.ReviewStateApproved,
		RiskLevel:    "high",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Save receipt.
	if err := store.SaveReceipt(receipt, "test-change"); err != nil {
		t.Fatalf("SaveReceipt: %v", err)
	}

	// Load back.
	loaded, err := store.LatestReceipt("test-change")
	if err != nil {
		t.Fatalf("LatestReceipt: %v", err)
	}
	if loaded == nil {
		t.Fatal("LatestReceipt returned nil")
	}
	if loaded.LineageID != receipt.LineageID {
		t.Errorf("lineage ID mismatch: got %q, want %q", loaded.LineageID, receipt.LineageID)
	}
	if loaded.State != domain.ReviewStateApproved {
		t.Errorf("state mismatch: got %q, want %q", loaded.State, domain.ReviewStateApproved)
	}
}

func TestUninstallHooks(t *testing.T) {
	dir := t.TempDir()
	hookDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	// Install.
	if err := WriteHooks(dir); err != nil {
		t.Fatalf("WriteHooks: %v", err)
	}

	// Uninstall.
	if err := UninstallHooks(dir); err != nil {
		t.Fatalf("UninstallHooks: %v", err)
	}

	// Check that GAIA content is removed.
	data, err := os.ReadFile(filepath.Join(hookDir, "pre-commit"))
	if err != nil {
		t.Fatalf("read hook after uninstall: %v", err)
	}
	if contains(string(data), "GAIA Review Gate") {
		t.Error("GAIA content was not removed by uninstall")
	}
}

// memReceiptStore is an in-memory receipt store for testing.
type memReceiptStore struct {
	receipt  *domain.ReviewReceipt
	receipts []ReceiptSummary
}

func (m *memReceiptStore) LatestReceipt(changeName string) (*domain.ReviewReceipt, error) {
	return m.receipt, nil
}

func (m *memReceiptStore) SaveReceipt(receipt *domain.ReviewReceipt, changeName string) error {
	m.receipt = receipt
	m.receipts = append(m.receipts, ReceiptSummary{
		ChangeName: changeName,
		State:      string(receipt.State),
		RiskLevel:  receipt.RiskLevel,
		CreatedAt:  receipt.CreatedAt,
	})
	return nil
}

func (m *memReceiptStore) ListReceipts() ([]ReceiptSummary, error) {
	return m.receipts, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure interface is satisfied.
var _ ReceiptStore = (*memReceiptStore)(nil)

// Ensure unused imports compile for json if needed.
var _ = json.Marshal


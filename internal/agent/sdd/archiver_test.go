package sdd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/delivery"
	"gaia/internal/review/gates"
)

func TestArchiver_DeferredMode_EnqueuesStandbyItem(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup mock approved review receipt in temp dir
	receiptStore := gates.NewFSReceiptStore(tmpDir)
	approvedReceipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "sha256:abc123lineagehash",
		SnapshotHash: "sha256:snap123",
		State:        domain.ReviewStateApproved,
		RiskLevel:    "low",
		CreatedAt:    time.Now(),
	}
	if err := receiptStore.SaveReceipt(approvedReceipt, "auth-models"); err != nil {
		t.Fatalf("failed to save test receipt: %v", err)
	}

	// 2. Setup delivery queue
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	fileQueue, err := delivery.NewFileQueue(queuePath)
	if err != nil {
		t.Fatalf("failed to create file queue: %v", err)
	}

	// 3. Initialize archiver with custom queue & receipt store
	spawner := newSDDSpawner()
	sa := NewArchiverWithDeps(spawner, fileQueue, receiptStore, tmpDir)

	task := domain.SubagentTask{
		ID:          "task-arch-01",
		Description: "Archive completed auth-models change\ndelivery_mode: deferred",
		WorkDir:     tmpDir,
	}

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Fatalf("expected success, got %q (summary: %s)", result.Status, result.Summary)
	}

	// 4. Verify item was enqueued to delivery queue with StatusStandby
	items, err := fileQueue.List()
	if err != nil {
		t.Fatalf("failed to list queue: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 queued delivery item, got %d", len(items))
	}

	item := items[0]
	if item.ChangeName != "auth-models" {
		t.Errorf("expected change name 'auth-models', got %q", item.ChangeName)
	}
	if item.Status != delivery.StatusStandby {
		t.Errorf("expected status standby, got %q", item.Status)
	}
	if item.PRTitle == "" {
		t.Error("expected generated PR title, got empty")
	}
	if item.PRBody == "" {
		t.Error("expected generated PR body, got empty")
	}
	if item.ReceiptLineage != "sha256:abc123lineagehash" {
		t.Errorf("expected receipt lineage 'sha256:abc123lineagehash', got %q", item.ReceiptLineage)
	}
}

func TestArchiver_InteractiveMode_DoesNotEnqueue(t *testing.T) {
	tmpDir := t.TempDir()

	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	fileQueue, err := delivery.NewFileQueue(queuePath)
	if err != nil {
		t.Fatalf("failed to create file queue: %v", err)
	}

	receiptStore := gates.NewFSReceiptStore(tmpDir)

	spawner := newSDDSpawner()
	sa := NewArchiverWithDeps(spawner, fileQueue, receiptStore, tmpDir)

	task := domain.SubagentTask{
		ID:          "task-arch-02",
		Description: "Archive completed auth-models change\ndelivery_mode: interactive",
		WorkDir:     tmpDir,
	}

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Fatalf("expected success, got %q", result.Status)
	}

	items, err := fileQueue.List()
	if err != nil {
		t.Fatalf("failed to list queue: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 queued items in interactive mode, got %d", len(items))
	}
}

func TestArchiver_ReviewGate_BlocksUnapprovedReceipt(t *testing.T) {
	tmpDir := t.TempDir()

	receiptStore := gates.NewFSReceiptStore(tmpDir)
	unapprovedReceipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "sha256:unapproved",
		SnapshotHash: "sha256:snap456",
		State:        domain.ReviewStateFixRequired,
		RiskLevel:    "high",
		CreatedAt:    time.Now(),
	}
	if err := receiptStore.SaveReceipt(unapprovedReceipt, "auth-models"); err != nil {
		t.Fatalf("failed to save test receipt: %v", err)
	}

	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	fileQueue, _ := delivery.NewFileQueue(queuePath)

	spawner := newSDDSpawner()
	sa := NewArchiverWithDeps(spawner, fileQueue, receiptStore, tmpDir)

	task := domain.SubagentTask{
		ID:          "task-arch-03",
		Description: "Archive change auth-models\ndelivery_mode: deferred",
		WorkDir:     tmpDir,
	}

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked status due to unapproved receipt, got %q", result.Status)
	}
}

func TestExtractDeliveryModeAndChangeName(t *testing.T) {
	task1 := domain.SubagentTask{
		Description: "Archive change payment-service\ndelivery_mode: deferred",
	}
	if mode := extractDeliveryMode(task1); mode != delivery.DeliveryModeDeferred {
		t.Errorf("expected deferred mode, got %q", mode)
	}
	if change := extractChangeName(task1); change != "payment-service" {
		t.Errorf("expected payment-service change, got %q", change)
	}

	task2 := domain.SubagentTask{
		Description: "Finalize auth-slice change",
		KGContext:   []string{"delivery: auto"},
	}
	if mode := extractDeliveryMode(task2); mode != delivery.DeliveryModeAuto {
		t.Errorf("expected auto mode, got %q", mode)
	}
	if change := extractChangeName(task2); change != "auth-slice" {
		t.Errorf("expected auth-slice change, got %q", change)
	}

	title, body := generatePRTitleAndBody("user-profile", "Completed profile enhancements")
	if !strings.Contains(title, "feat(user-profile)") {
		t.Errorf("expected feat(user-profile) in title, got %q", title)
	}
	if !strings.Contains(body, "Strict TDD verified") || !strings.Contains(body, "user-profile") {
		t.Errorf("expected body to contain verification notes, got %q", body)
	}
}

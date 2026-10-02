package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/delivery"
	"gaia/internal/review/gates"
)

func TestDeliveryCLI_List(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	q, err := delivery.NewFileQueue(queuePath)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}

	// Enqueue test items
	item1 := delivery.DeliveryItem{
		ID:             "del-1",
		ChangeName:     "auth-models",
		Branch:         "feature/auth-models",
		BaseBranch:     "main",
		CommitSHA:      "sha-111",
		ReceiptLineage: "sha256:lineage1",
		PRTitle:        "feat(auth): domain models",
		PRBody:         "body 1",
		Status:         delivery.StatusStandby,
		CreatedAt:      time.Now(),
	}
	item2 := delivery.DeliveryItem{
		ID:             "del-2",
		ChangeName:     "user-profile",
		Branch:         "feature/user-profile",
		BaseBranch:     "feature/auth-models",
		CommitSHA:      "sha-222",
		ReceiptLineage: "sha256:lineage2",
		PRTitle:        "feat(user): user profile",
		PRBody:         "body 2",
		Status:         delivery.StatusReleased,
		CreatedAt:      time.Now(),
	}

	if err := q.Enqueue(item1); err != nil {
		t.Fatalf("enqueue item1: %v", err)
	}
	if err := q.Enqueue(item2); err != nil {
		t.Fatalf("enqueue item2: %v", err)
	}

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	// 1. List without filter -> shows items
	err = runDeliveryCLI([]string{"list"}, &buf, &errBuf, q, nil, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "del-1") || !strings.Contains(out, "auth-models") {
		t.Errorf("expected del-1 in list output, got: %s", out)
	}

	// 2. List with --status standby -> only shows standby
	buf.Reset()
	err = runDeliveryCLI([]string{"list", "--status", "standby"}, &buf, &errBuf, q, nil, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "del-1") {
		t.Errorf("expected del-1 in filtered output, got: %s", out)
	}
	if strings.Contains(out, "del-2") {
		t.Errorf("did not expect del-2 in standby list output, got: %s", out)
	}
}

func TestDeliveryCLI_Status(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	q, _ := delivery.NewFileQueue(queuePath)

	item1 := delivery.DeliveryItem{
		ID:             "del-1",
		ChangeName:     "auth-models",
		Branch:         "feature/auth-models",
		CommitSHA:      "sha-111",
		ReceiptLineage: "sha256:lineage1",
		Status:         delivery.StatusStandby,
	}
	item2 := delivery.DeliveryItem{
		ID:             "del-2",
		ChangeName:     "user-profile",
		Branch:         "feature/user-profile",
		CommitSHA:      "sha-222",
		ReceiptLineage: "sha256:lineage2",
		Status:         delivery.StatusReleased,
	}
	_ = q.Enqueue(item1)
	_ = q.Enqueue(item2)

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	err := runDeliveryCLI([]string{"status"}, &buf, &errBuf, q, nil, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Total:") || !strings.Contains(out, "Standby:") || !strings.Contains(out, "Released:") {
		t.Errorf("status summary output incorrect, got: %s", out)
	}
	if !strings.Contains(out, "2") || !strings.Contains(out, "1") {
		t.Errorf("status counts incorrect, got: %s", out)
	}
}

func TestDeliveryCLI_Release(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	q, _ := delivery.NewFileQueue(queuePath)

	receiptStore := gates.NewFSReceiptStore(tmpDir)
	receipt := &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    "sha256:approvedlineage",
		SnapshotHash: "sha256:snap1",
		State:        domain.ReviewStateApproved,
		CreatedAt:    time.Now(),
	}
	_ = receiptStore.SaveReceipt(receipt, "auth-models")

	item := delivery.DeliveryItem{
		ID:             "del-1",
		ChangeName:     "auth-models",
		Branch:         "feature/auth-models",
		BaseBranch:     "main",
		CommitSHA:      "sha-111",
		ReceiptLineage: "sha256:approvedlineage",
		PRTitle:        "feat(auth): models",
		PRBody:         "body",
		Status:         delivery.StatusStandby,
		CreatedAt:      time.Now(),
	}
	_ = q.Enqueue(item)

	mockTransport := delivery.NewMockTransport()
	eng := delivery.NewEngine(q, mockTransport, receiptStore, tmpDir)

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	// 1. Release single item
	err := runDeliveryCLI([]string{"release", "del-1"}, &buf, &errBuf, q, eng, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "del-1") || !strings.Contains(out, "https://github.com/example/repo/pull/") {
		t.Errorf("expected successful release output, got: %s", out)
	}

	// Verify item is now released
	updatedItem, _ := q.Get("del-1")
	if updatedItem.Status != delivery.StatusReleased {
		t.Errorf("expected status released, got %s", updatedItem.Status)
	}
}

func TestDeliveryCLI_ReleaseAll(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	q, _ := delivery.NewFileQueue(queuePath)

	receiptStore := gates.NewFSReceiptStore(tmpDir)
	receipt1 := &domain.ReviewReceipt{
		Schema:    "gaia.review-receipt/v1",
		LineageID: "sha256:lin1",
		State:     domain.ReviewStateApproved,
		CreatedAt: time.Now(),
	}
	receipt2 := &domain.ReviewReceipt{
		Schema:    "gaia.review-receipt/v1",
		LineageID: "sha256:lin2",
		State:     domain.ReviewStateApproved,
		CreatedAt: time.Now(),
	}
	_ = receiptStore.SaveReceipt(receipt1, "pr1")
	_ = receiptStore.SaveReceipt(receipt2, "pr2")

	item1 := delivery.DeliveryItem{
		ID:             "del-1",
		ChangeName:     "pr1",
		Branch:         "feature/pr1",
		BaseBranch:     "main",
		CommitSHA:      "sha-1",
		ReceiptLineage: "sha256:lin1",
		PRTitle:        "pr1",
		PRBody:         "body",
		Status:         delivery.StatusStandby,
		CreatedAt:      time.Now().Add(-10 * time.Minute),
	}
	item2 := delivery.DeliveryItem{
		ID:             "del-2",
		ChangeName:     "pr2",
		Branch:         "feature/pr2",
		BaseBranch:     "feature/pr1",
		CommitSHA:      "sha-2",
		ReceiptLineage: "sha256:lin2",
		PRTitle:        "pr2",
		PRBody:         "body",
		Status:         delivery.StatusStandby,
		CreatedAt:      time.Now(),
	}
	_ = q.Enqueue(item1)
	_ = q.Enqueue(item2)

	mockTransport := delivery.NewMockTransport()
	eng := delivery.NewEngine(q, mockTransport, receiptStore, tmpDir)

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	// Release all
	err := runDeliveryCLI([]string{"release", "--all"}, &buf, &errBuf, q, eng, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Released 2 item(s)") {
		t.Errorf("expected 2 released items in output, got: %s", out)
	}
}

func TestDeliveryCLI_Discard(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	q, _ := delivery.NewFileQueue(queuePath)

	item := delivery.DeliveryItem{
		ID:             "del-1",
		ChangeName:     "auth-models",
		Branch:         "feature/auth-models",
		CommitSHA:      "sha-111",
		ReceiptLineage: "sha256:lineage1",
		Status:         delivery.StatusStandby,
	}
	_ = q.Enqueue(item)

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	err := runDeliveryCLI([]string{"discard", "del-1"}, &buf, &errBuf, q, nil, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "del-1") || !strings.Contains(out, "discarded") {
		t.Errorf("expected discard output, got: %s", out)
	}

	updated, _ := q.Get("del-1")
	if updated.Status != delivery.StatusDiscarded {
		t.Errorf("expected status discarded, got %s", updated.Status)
	}
}

func TestDeliveryCLI_Diff(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	q, _ := delivery.NewFileQueue(queuePath)

	item := delivery.DeliveryItem{
		ID:             "del-1",
		ChangeName:     "auth-models",
		Branch:         "feature/auth-models",
		BaseBranch:     "main",
		CommitSHA:      "sha-111",
		ReceiptLineage: "sha256:lineage1",
		PRTitle:        "feat(auth): domain models",
		PRBody:         "## Summary\nDomain models implemented",
		Status:         delivery.StatusStandby,
		CreatedAt:      time.Now(),
	}
	_ = q.Enqueue(item)

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	err := runDeliveryCLI([]string{"diff", "del-1"}, &buf, &errBuf, q, nil, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "del-1") || !strings.Contains(out, "feat(auth): domain models") || !strings.Contains(out, "feature/auth-models") {
		t.Errorf("expected diff/inspection output, got: %s", out)
	}
}

func TestDeliveryCLI_ErrorsAndUsage(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, ".gaia", "delivery", "queue.json")
	q, _ := delivery.NewFileQueue(queuePath)

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	// Empty args -> usage
	err := runDeliveryCLI([]string{}, &buf, &errBuf, q, nil, tmpDir)
	if err != nil {
		t.Errorf("expected no error on empty args, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage: gaia delivery") {
		t.Errorf("expected usage output, got: %s", buf.String())
	}

	// Unknown command
	buf.Reset()
	errBuf.Reset()
	err = runDeliveryCLI([]string{"unknown-cmd"}, &buf, &errBuf, q, nil, tmpDir)
	if err == nil {
		t.Error("expected error for unknown command")
	}

	// Release without ID or --all
	buf.Reset()
	errBuf.Reset()
	err = runDeliveryCLI([]string{"release"}, &buf, &errBuf, q, nil, tmpDir)
	if err == nil {
		t.Error("expected error for release with no args")
	}

	// Discard without ID
	buf.Reset()
	errBuf.Reset()
	err = runDeliveryCLI([]string{"discard"}, &buf, &errBuf, q, nil, tmpDir)
	if err == nil {
		t.Error("expected error for discard with no args")
	}

	// Diff without ID
	buf.Reset()
	errBuf.Reset()
	err = runDeliveryCLI([]string{"diff"}, &buf, &errBuf, q, nil, tmpDir)
	if err == nil {
		t.Error("expected error for diff with no args")
	}
}

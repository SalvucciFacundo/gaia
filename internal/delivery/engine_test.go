package delivery

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/review/gates"
)

// memReceiptStore provides an in-memory gates.ReceiptStore for engine testing.
type memReceiptStore struct {
	receipts map[string]*domain.ReviewReceipt
}

func newMemReceiptStore() *memReceiptStore {
	return &memReceiptStore{
		receipts: make(map[string]*domain.ReviewReceipt),
	}
}

func (m *memReceiptStore) LatestReceipt(changeName string) (*domain.ReviewReceipt, error) {
	return m.receipts[changeName], nil
}

func (m *memReceiptStore) SaveReceipt(receipt *domain.ReviewReceipt, changeName string) error {
	m.receipts[changeName] = receipt
	return nil
}

func (m *memReceiptStore) ListReceipts() ([]gates.ReceiptSummary, error) {
	return nil, nil
}

func newTestEngine(t *testing.T) (*Engine, *FileQueue, *MockTransport, *memReceiptStore) {
	dir := t.TempDir()
	queuePath := filepath.Join(dir, "queue.json")
	q, err := NewFileQueue(queuePath)
	if err != nil {
		t.Fatalf("NewFileQueue failed: %v", err)
	}
	transport := NewMockTransport()
	store := newMemReceiptStore()
	engine := NewEngine(q, transport, store, dir)
	return engine, q, transport, store
}

func validReceipt(changeName, lineageID string) *domain.ReviewReceipt {
	return &domain.ReviewReceipt{
		Schema:       "gaia.review-receipt/v1",
		LineageID:    lineageID,
		SnapshotHash: "sha256:valid-hash",
		State:        domain.ReviewStateApproved,
		RiskLevel:    "low",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestEngine_Release_Success(t *testing.T) {
	engine, queue, transport, store := newTestEngine(t)
	ctx := context.Background()

	lineage := "sha256:abc123lineage"
	store.receipts["feat-auth"] = validReceipt("feat-auth", lineage)

	item := DeliveryItem{
		ID:             "del-1",
		ChangeName:     "feat-auth",
		Branch:         "feature/auth",
		BaseBranch:     "main",
		CommitSHA:      "commit-sha-1",
		ReceiptLineage: lineage,
		PRTitle:        "feat: auth system",
		PRBody:         "pr body",
		Status:         StatusStandby,
		CreatedAt:      time.Now(),
	}
	if err := queue.Enqueue(item); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	res, err := engine.Release(ctx, "del-1")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success, got failure: %s", res.Error)
	}
	if res.PRURL == "" {
		t.Errorf("expected non-empty PRURL")
	}

	// Verify queue item updated to released
	updated, err := queue.Get("del-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if updated.Status != StatusReleased {
		t.Errorf("expected StatusReleased, got %s", updated.Status)
	}
	if updated.PRURL != res.PRURL {
		t.Errorf("expected PRURL %s, got %s", res.PRURL, updated.PRURL)
	}

	// Verify transport operations
	if len(transport.PushedBranches) != 1 || transport.PushedBranches[0] != "feature/auth" {
		t.Errorf("expected pushed branch feature/auth, got %v", transport.PushedBranches)
	}
	if len(transport.CreatedPRs) != 1 || transport.CreatedPRs[0].ID != "del-1" {
		t.Errorf("expected 1 created PR, got %v", transport.CreatedPRs)
	}
}

func TestEngine_Release_MissingItem(t *testing.T) {
	engine, _, _, _ := newTestEngine(t)
	ctx := context.Background()

	_, err := engine.Release(ctx, "nonexistent-id")
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestEngine_Release_ContentDriftProtection(t *testing.T) {
	ctx := context.Background()

	// 1. Missing receipt (unreviewed)
	t.Run("unreviewed_missing_receipt", func(t *testing.T) {
		engine, queue, transport, _ := newTestEngine(t)
		item := DeliveryItem{
			ID:             "del-unreviewed",
			ChangeName:     "feat-unreviewed",
			Branch:         "feature/unreviewed",
			CommitSHA:      "sha1",
			ReceiptLineage: "lineage-1",
			Status:         StatusStandby,
			CreatedAt:      time.Now(),
		}
		_ = queue.Enqueue(item)

		res, err := engine.Release(ctx, "del-unreviewed")
		if !errors.Is(err, ErrContentDrift) {
			t.Errorf("expected ErrContentDrift, got %v", err)
		}
		if res.Success {
			t.Error("expected release to fail")
		}
		if len(transport.PushedBranches) > 0 {
			t.Error("expected no push for drifted/unreviewed item")
		}

		// Verify queue marked as failed
		updated, _ := queue.Get("del-unreviewed")
		if updated.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", updated.Status)
		}
	})

	// 2. Receipt not approved
	t.Run("receipt_not_approved", func(t *testing.T) {
		engine, queue, transport, store := newTestEngine(t)
		receipt := validReceipt("feat-wip", "lineage-wip")
		receipt.State = domain.ReviewStateReviewing
		store.receipts["feat-wip"] = receipt

		item := DeliveryItem{
			ID:             "del-wip",
			ChangeName:     "feat-wip",
			Branch:         "feature/wip",
			CommitSHA:      "sha1",
			ReceiptLineage: "lineage-wip",
			Status:         StatusStandby,
			CreatedAt:      time.Now(),
		}
		_ = queue.Enqueue(item)

		res, err := engine.Release(ctx, "del-wip")
		if !errors.Is(err, ErrContentDrift) {
			t.Errorf("expected ErrContentDrift, got %v", err)
		}
		if res.Success {
			t.Error("expected release to fail")
		}
		if len(transport.PushedBranches) > 0 {
			t.Error("expected no push for unapproved item")
		}
	})

	// 3. Lineage mismatch (content modified after review)
	t.Run("receipt_lineage_mismatch", func(t *testing.T) {
		engine, queue, transport, store := newTestEngine(t)
		store.receipts["feat-auth"] = validReceipt("feat-auth", "sha256:NEW-lineage")

		item := DeliveryItem{
			ID:             "del-drifted",
			ChangeName:     "feat-auth",
			Branch:         "feature/auth",
			CommitSHA:      "sha1",
			ReceiptLineage: "sha256:OLD-lineage",
			Status:         StatusStandby,
			CreatedAt:      time.Now(),
		}
		_ = queue.Enqueue(item)

		res, err := engine.Release(ctx, "del-drifted")
		if !errors.Is(err, ErrContentDrift) {
			t.Errorf("expected ErrContentDrift, got %v", err)
		}
		if res.Success {
			t.Error("expected release to fail")
		}
		if len(transport.PushedBranches) > 0 {
			t.Error("expected no push for mismatched lineage")
		}

		updated, _ := queue.Get("del-drifted")
		if updated.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", updated.Status)
		}
	})
}

func TestEngine_ReleaseAll_TopologicalOrder(t *testing.T) {
	engine, queue, transport, store := newTestEngine(t)
	ctx := context.Background()

	// Setup 3 stacked items:
	// PR1 (base: main) -> PR2 (base: feature/pr1) -> PR3 (base: feature/pr2)
	store.receipts["change-1"] = validReceipt("change-1", "lin-1")
	store.receipts["change-2"] = validReceipt("change-2", "lin-2")
	store.receipts["change-3"] = validReceipt("change-3", "lin-3")

	item1 := DeliveryItem{
		ID:             "del-1",
		ChangeName:     "change-1",
		Branch:         "feature/pr1",
		BaseBranch:     "main",
		CommitSHA:      "sha-1",
		ReceiptLineage: "lin-1",
		PRTitle:        "feat: pr1",
		Status:         StatusStandby,
		CreatedAt:      time.Now(),
	}
	item2 := DeliveryItem{
		ID:             "del-2",
		ChangeName:     "change-2",
		Branch:         "feature/pr2",
		BaseBranch:     "feature/pr1",
		CommitSHA:      "sha-2",
		ReceiptLineage: "lin-2",
		PRTitle:        "feat: pr2",
		Status:         StatusStandby,
		CreatedAt:      time.Now().Add(time.Second),
	}
	item3 := DeliveryItem{
		ID:             "del-3",
		ChangeName:     "change-3",
		Branch:         "feature/pr3",
		BaseBranch:     "feature/pr2",
		CommitSHA:      "sha-3",
		ReceiptLineage: "lin-3",
		PRTitle:        "feat: pr3",
		Status:         StatusStandby,
		CreatedAt:      time.Now().Add(2 * time.Second),
	}

	// Enqueue in REVERSE order to test topological sorting (del-3, then del-2, then del-1)
	_ = queue.Enqueue(item3)
	_ = queue.Enqueue(item2)
	_ = queue.Enqueue(item1)

	results, err := engine.ReleaseAll(ctx)
	if err != nil {
		t.Fatalf("ReleaseAll failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify execution order was topological: del-1, del-2, del-3
	if results[0].ItemID != "del-1" || results[1].ItemID != "del-2" || results[2].ItemID != "del-3" {
		t.Errorf("results not in topological order: %v, %v, %v", results[0].ItemID, results[1].ItemID, results[2].ItemID)
	}

	if len(transport.PushedBranches) != 3 {
		t.Fatalf("expected 3 pushed branches, got %d", len(transport.PushedBranches))
	}
	if transport.PushedBranches[0] != "feature/pr1" ||
		transport.PushedBranches[1] != "feature/pr2" ||
		transport.PushedBranches[2] != "feature/pr3" {
		t.Errorf("transport pushed branches out of topological order: %v", transport.PushedBranches)
	}
}

func TestEngine_Release_TransportFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("push_failure", func(t *testing.T) {
		engine, queue, transport, store := newTestEngine(t)
		store.receipts["feat-fail"] = validReceipt("feat-fail", "lin-fail")
		transport.PushErr = errors.New("network timeout")

		item := DeliveryItem{
			ID:             "del-fail",
			ChangeName:     "feat-fail",
			Branch:         "feature/fail",
			CommitSHA:      "sha",
			ReceiptLineage: "lin-fail",
			Status:         StatusStandby,
			CreatedAt:      time.Now(),
		}
		_ = queue.Enqueue(item)

		res, err := engine.Release(ctx, "del-fail")
		if err == nil || res.Success {
			t.Fatal("expected push failure error")
		}

		updated, _ := queue.Get("del-fail")
		if updated.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", updated.Status)
		}
	})

	t.Run("create_pr_failure", func(t *testing.T) {
		engine, queue, transport, store := newTestEngine(t)
		store.receipts["feat-fail2"] = validReceipt("feat-fail2", "lin-fail2")
		transport.CreatePRErr = errors.New("gh rate limited")

		item := DeliveryItem{
			ID:             "del-fail2",
			ChangeName:     "feat-fail2",
			Branch:         "feature/fail2",
			CommitSHA:      "sha",
			ReceiptLineage: "lin-fail2",
			Status:         StatusStandby,
			CreatedAt:      time.Now(),
		}
		_ = queue.Enqueue(item)

		res, err := engine.Release(ctx, "del-fail2")
		if err == nil || res.Success {
			t.Fatal("expected create PR failure error")
		}

		updated, _ := queue.Get("del-fail2")
		if updated.Status != StatusFailed {
			t.Errorf("expected StatusFailed, got %s", updated.Status)
		}
	})
}

func TestEngine_ReleaseAll_EmptyQueue(t *testing.T) {
	engine, _, _, _ := newTestEngine(t)
	ctx := context.Background()

	results, err := engine.ReleaseAll(ctx)
	if err != nil {
		t.Fatalf("ReleaseAll failed on empty queue: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestDeliveryModes(t *testing.T) {
	if DeliveryModeDeferred != "deferred" {
		t.Errorf("expected DeliveryModeDeferred 'deferred', got %s", DeliveryModeDeferred)
	}
	if DeliveryModeInteractive != "interactive" {
		t.Errorf("expected DeliveryModeInteractive 'interactive', got %s", DeliveryModeInteractive)
	}
	if DeliveryModeAuto != "auto" {
		t.Errorf("expected DeliveryModeAuto 'auto', got %s", DeliveryModeAuto)
	}
}

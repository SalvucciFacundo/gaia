package delivery

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestQueue(t *testing.T) (*FileQueue, string) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "queue.json")
	q, err := NewFileQueue(filePath)
	if err != nil {
		t.Fatalf("NewFileQueue failed: %v", err)
	}
	return q, filePath
}

func sampleItem(id, branch, status string) DeliveryItem {
	return DeliveryItem{
		ID:             id,
		ChangeName:     "feat-auth",
		Branch:         branch,
		BaseBranch:     "main",
		CommitSHA:      "a1b2c3d4e5f678901234567890abcdef12345678",
		ReceiptLineage: "sha256:789abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456",
		PRTitle:        "feat: implement " + branch,
		PRBody:         "## Description\n...",
		Status:         DeliveryStatus(status),
		CreatedAt:      time.Now(),
	}
}

func TestFileQueue_EnqueueAndGet(t *testing.T) {
	q, _ := newTestQueue(t)

	item := sampleItem("del-1", "feature/auth-1", string(StatusStandby))
	err := q.Enqueue(item)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	got, err := q.Get("del-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "del-1" || got.Branch != "feature/auth-1" {
		t.Errorf("Get returned unexpected item: %+v", got)
	}

	// Non-existent item
	_, err = q.Get("del-nonexistent")
	if err != ErrItemNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestFileQueue_ListFilteringAndFIFO(t *testing.T) {
	q, _ := newTestQueue(t)

	_ = q.Enqueue(sampleItem("del-1", "feature/step-1", string(StatusStandby)))
	time.Sleep(5 * time.Millisecond)
	_ = q.Enqueue(sampleItem("del-2", "feature/step-2", string(StatusReleased)))
	time.Sleep(5 * time.Millisecond)
	_ = q.Enqueue(sampleItem("del-3", "feature/step-3", string(StatusStandby)))

	// 1. List all items
	all, err := q.List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 items, got %d", len(all))
	}
	// Check FIFO order
	if all[0].ID != "del-1" || all[1].ID != "del-2" || all[2].ID != "del-3" {
		t.Errorf("items not in FIFO order: %v, %v, %v", all[0].ID, all[1].ID, all[2].ID)
	}

	// 2. Filter by standby
	standbyOnly, err := q.List(StatusStandby)
	if err != nil {
		t.Fatalf("List(StatusStandby) failed: %v", err)
	}
	if len(standbyOnly) != 2 {
		t.Fatalf("expected 2 standby items, got %d", len(standbyOnly))
	}
	if standbyOnly[0].ID != "del-1" || standbyOnly[1].ID != "del-3" {
		t.Errorf("expected del-1 and del-3, got %+v", standbyOnly)
	}
}

func TestFileQueue_StatusTransitions(t *testing.T) {
	q, _ := newTestQueue(t)

	_ = q.Enqueue(sampleItem("del-1", "feature/step-1", string(StatusStandby)))

	// MarkReleased
	err := q.MarkReleased("del-1", "https://github.com/org/repo/pull/42")
	if err != nil {
		t.Fatalf("MarkReleased failed: %v", err)
	}
	item, _ := q.Get("del-1")
	if item.Status != StatusReleased {
		t.Errorf("expected StatusReleased, got %s", item.Status)
	}
	if item.PRURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("expected PRURL, got %q", item.PRURL)
	}
	if item.ReleasedAt == nil {
		t.Error("expected ReleasedAt to be populated")
	}

	// MarkDiscarded
	_ = q.Enqueue(sampleItem("del-2", "feature/step-2", string(StatusStandby)))
	err = q.MarkDiscarded("del-2")
	if err != nil {
		t.Fatalf("MarkDiscarded failed: %v", err)
	}
	item2, _ := q.Get("del-2")
	if item2.Status != StatusDiscarded {
		t.Errorf("expected StatusDiscarded, got %s", item2.Status)
	}

	// MarkFailed
	_ = q.Enqueue(sampleItem("del-3", "feature/step-3", string(StatusStandby)))
	err = q.MarkFailed("del-3", "git push rejected: non-fast-forward")
	if err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	item3, _ := q.Get("del-3")
	if item3.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", item3.Status)
	}
	if item3.ErrorMessage != "git push rejected: non-fast-forward" {
		t.Errorf("expected ErrorMessage, got %q", item3.ErrorMessage)
	}
}

func TestFileQueue_PersistenceAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "queue.json")

	// Instance 1: write
	q1, err := NewFileQueue(filePath)
	if err != nil {
		t.Fatalf("NewFileQueue 1 failed: %v", err)
	}
	_ = q1.Enqueue(sampleItem("del-1", "feature/step-1", string(StatusStandby)))
	_ = q1.Enqueue(sampleItem("del-2", "feature/step-2", string(StatusReleased)))

	// Instance 2: read same file
	q2, err := NewFileQueue(filePath)
	if err != nil {
		t.Fatalf("NewFileQueue 2 failed: %v", err)
	}
	items, err := q2.List()
	if err != nil {
		t.Fatalf("q2.List() failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 persisted items, got %d", len(items))
	}
	if items[0].ID != "del-1" || items[1].ID != "del-2" {
		t.Errorf("persisted items mismatch: %+v", items)
	}
}

func TestFileQueue_ConcurrentAccess(t *testing.T) {
	q, _ := newTestQueue(t)
	var wg sync.WaitGroup

	workerCount := 10
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := "del-" + string(rune('A'+idx))
			_ = q.Enqueue(sampleItem(id, "feature/branch-"+id, string(StatusStandby)))
			_, _ = q.List()
			_ = q.MarkReleased(id, "https://github.com/pull/"+id)
		}(i)
	}
	wg.Wait()

	items, err := q.List()
	if err != nil {
		t.Fatalf("List failed after concurrency: %v", err)
	}
	if len(items) != workerCount {
		t.Fatalf("expected %d items after concurrent enqueue, got %d", workerCount, len(items))
	}
}

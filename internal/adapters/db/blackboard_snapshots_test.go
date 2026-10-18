package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gaia/internal/blackboard"
)

func TestSQLiteRepo_BlackboardSnapshots(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_snapshots.db")
	repo, err := NewSQLiteRepoWithPath(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	ctx := context.Background()

	entries := map[string]blackboard.MemoryEntry{
		"tasks.w1.summary": {
			Key:        "tasks.w1.summary",
			Data:       []byte("\"All unit tests passed cleanly\""),
			ProducerID: "w1",
		},
		"global.config": {
			Key:        "global.config",
			Data:       []byte("{\"env\":\"prod\"}"),
			ProducerID: "",
		},
	}

	// 1. Save snapshot
	id, err := repo.SaveBlackboardSnapshot(ctx, "sess-100", "dev", "pre-merge-checkpoint", entries)
	if err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	// 2. Retrieve snapshot
	gotEntries, meta, err := repo.GetBlackboardSnapshot(ctx, id)
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if meta.Label != "pre-merge-checkpoint" {
		t.Errorf("expected label 'pre-merge-checkpoint', got %q", meta.Label)
	}
	if meta.EntryCount != 2 {
		t.Errorf("expected entry count 2, got %d", meta.EntryCount)
	}
	if string(gotEntries["tasks.w1.summary"].Data) != "\"All unit tests passed cleanly\"" {
		t.Errorf("unexpected entry data: %s", string(gotEntries["tasks.w1.summary"].Data))
	}

	// 3. List snapshots
	list, err := repo.ListBlackboardSnapshots(ctx, "sess-100", "dev")
	if err != nil {
		t.Fatalf("failed to list snapshots: %v", err)
	}
	if len(list) != 1 || list[0].ID != id {
		t.Errorf("expected 1 snapshot with ID %s, got %+v", id, list)
	}

	// 4. Tamper detection
	// Mutate the JSON payload directly in SQLite without updating sha256
	_, err = repo.db.ExecContext(ctx, "UPDATE blackboard_snapshots SET snapshot_json = '{\"tampered\": true}' WHERE id = ?", id)
	if err != nil {
		t.Fatalf("failed to tamper row: %v", err)
	}

	_, _, err = repo.GetBlackboardSnapshot(ctx, id)
	if !errors.Is(err, ErrSnapshotTampered) {
		t.Fatalf("expected ErrSnapshotTampered, got: %v", err)
	}
}

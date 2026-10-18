package blackboard

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestBlackboard_InlineAndSpill(t *testing.T) {
	tmpDir := t.TempDir()
	bb := New(tmpDir)

	// 1. Inline payload (< 4096 bytes)
	inlineData := []byte("{\"status\": \"ok\", \"count\": 42}")
	entry, err := bb.Set("worker-1", "tasks.worker-1.result", inlineData)
	if err != nil {
		t.Fatalf("unexpected error setting inline: %v", err)
	}
	if entry.IsSpilled {
		t.Errorf("expected entry not to be spilled")
	}

	got, err := bb.Get("tasks.worker-1.result")
	if err != nil {
		t.Fatalf("unexpected error getting key: %v", err)
	}
	payload, err := ReadPayload(got)
	if err != nil {
		t.Fatalf("unexpected error reading payload: %v", err)
	}
	if !bytes.Equal(payload, inlineData) {
		t.Errorf("payload mismatch: got %s, want %s", string(payload), string(inlineData))
	}

	// 2. Large payload (> 4096 bytes)
	largeData := bytes.Repeat([]byte("A"), 5000)
	spillEntry, err := bb.Set("worker-1", "tasks.worker-1.large", largeData)
	if err != nil {
		t.Fatalf("unexpected error setting large payload: %v", err)
	}
	if !spillEntry.IsSpilled {
		t.Errorf("expected entry to be spilled")
	}
	if spillEntry.SpillPath == "" {
		t.Errorf("expected non-empty spill path")
	}

	gotSpill, err := bb.Get("tasks.worker-1.large")
	if err != nil {
		t.Fatalf("unexpected error getting spilled key: %v", err)
	}
	spillPayload, err := ReadPayload(gotSpill)
	if err != nil {
		t.Fatalf("unexpected error reading spilled payload: %v", err)
	}
	if !bytes.Equal(spillPayload, largeData) {
		t.Errorf("spilled payload mismatch: got len %d, want len %d", len(spillPayload), len(largeData))
	}
}

func TestBlackboard_NamespaceIsolation(t *testing.T) {
	bb := New("")

	// Invalid prefix for worker-2
	_, err := bb.Set("worker-2", "tasks.worker-1.sneaky", []byte("bad"))
	if err == nil {
		t.Fatal("expected error on namespace violation, got nil")
	}

	// Allowed with worker-2
	_, err = bb.Set("worker-2", "tasks.worker-2.ok", []byte("good"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Global prefix is allowed for all
	_, err = bb.Set("worker-2", "global.config", []byte("shared"))
	if err != nil {
		t.Fatalf("unexpected error on global prefix: %v", err)
	}

	// Empty producerID allows any key
	_, err = bb.Set("", "system.boot", []byte("root"))
	if err != nil {
		t.Fatalf("unexpected error with empty producer: %v", err)
	}
}

func TestBlackboard_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	bb := New(tmpDir)

	var wg sync.WaitGroup
	workers := 10
	iterations := 50

	for w := 0; w < workers; w++ {
		wg.Add(1)
		workerID := fmt.Sprintf("w%d", w)
		go func(id string) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("tasks.%s.item_%d", id, i)
				val := []byte(fmt.Sprintf("val-%d", i))
				if i%10 == 0 {
					// occasional spill
					val = bytes.Repeat([]byte("B"), 4500)
				}
				_, err := bb.Set(id, key, val)
				if err != nil {
					t.Errorf("concurrent set failed: %v", err)
					return
				}

				entry, err := bb.Get(key)
				if err != nil {
					t.Errorf("concurrent get failed: %v", err)
					return
				}
				p, err := ReadPayload(entry)
				if err != nil || len(p) == 0 {
					t.Errorf("concurrent read payload failed: %v", err)
				}
			}
		}(workerID)
	}

	wg.Wait()

	keys := bb.ListKeys()
	expectedCount := workers * iterations
	if len(keys) != expectedCount {
		t.Errorf("expected %d keys, got %d", expectedCount, len(keys))
	}
}

func TestBlackboard_SnapshotRestore(t *testing.T) {
	bb1 := New(t.TempDir())
	_, _ = bb1.Set("", "key1", []byte("val1"))
	_, _ = bb1.Set("", "key2", []byte("val2"))

	snap := bb1.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected snapshot len 2, got %d", len(snap))
	}

	bb2 := New(t.TempDir())
	bb2.Restore(snap)

	keys := bb2.ListKeys()
	if len(keys) != 2 || keys[0] != "key1" || keys[1] != "key2" {
		t.Errorf("restored keys mismatch: %v", keys)
	}

	// Delete and Clear
	if !bb2.Delete("key1") {
		t.Errorf("expected delete to return true")
	}
	if bb2.Delete("key1") {
		t.Errorf("expected subsequent delete to return false")
	}

	bb2.Clear()
	if len(bb2.ListKeys()) != 0 {
		t.Errorf("expected 0 keys after clear, got %d", len(bb2.ListKeys()))
	}
}

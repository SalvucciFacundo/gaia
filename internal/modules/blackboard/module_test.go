package blackboard

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"gaia/internal/adapters/db"
	bb "gaia/internal/blackboard"
)

func TestModule_GetSetList(t *testing.T) {
	tempDir := t.TempDir()
	blackboardInstance := bb.New(tempDir)
	mod := NewModule(blackboardInstance)

	ctx := context.Background()

	// 1. Initial list empty
	res, err := mod.Execute(ctx, "blackboard_list", nil)
	if err != nil || !res.Success {
		t.Fatalf("blackboard_list failed: %v, res: %+v", err, res)
	}
	if res.Output != "[]" {
		t.Errorf("expected empty list, got: %s", res.Output)
	}

	// 2. Set inline key
	setRes, err := mod.Execute(ctx, "blackboard_set", map[string]interface{}{
		"key":   "global.user_id",
		"value": "12345",
	})
	if err != nil || !setRes.Success {
		t.Fatalf("blackboard_set failed: %v, res: %+v", err, setRes)
	}

	// 3. Get key
	getRes, err := mod.Execute(ctx, "blackboard_get", map[string]interface{}{
		"key": "global.user_id",
	})
	if err != nil || !getRes.Success {
		t.Fatalf("blackboard_get failed: %v, res: %+v", err, getRes)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(getRes.Output), &data); err != nil {
		t.Fatalf("failed to parse get output: %v", err)
	}
	if data["data"] != "12345" {
		t.Errorf("expected data '12345', got %v", data["data"])
	}

	// 4. Set large key (> 4096 bytes)
	largeVal := strings.Repeat("X", 5000)
	setLargeRes, err := mod.Execute(ctx, "blackboard_set", map[string]interface{}{
		"key":   "global.large_artifact",
		"value": largeVal,
	})
	if err != nil || !setLargeRes.Success {
		t.Fatalf("blackboard_set large failed: %v, res: %+v", err, setLargeRes)
	}

	// 5. Get spilled key
	getLargeRes, err := mod.Execute(ctx, "blackboard_get", map[string]interface{}{
		"key": "global.large_artifact",
	})
	if err != nil || !getLargeRes.Success {
		t.Fatalf("blackboard_get large failed: %v, res: %+v", err, getLargeRes)
	}

	var largeData map[string]interface{}
	if err := json.Unmarshal([]byte(getLargeRes.Output), &largeData); err != nil {
		t.Fatalf("failed to parse large get output: %v", err)
	}
	if largeData["is_spilled"] != true {
		t.Errorf("expected is_spilled true, got %v", largeData["is_spilled"])
	}
	if largeData["data"] != largeVal {
		t.Errorf("expected complete data match, got len %d", len(largeData["data"].(string)))
	}

	// 6. List keys
	listRes, err := mod.Execute(ctx, "blackboard_list", nil)
	if err != nil || !listRes.Success {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(listRes.Output, "global.user_id") || !strings.Contains(listRes.Output, "global.large_artifact") {
		t.Errorf("list output missing expected keys: %s", listRes.Output)
	}
}

func TestModule_Snapshots(t *testing.T) {
	tempDir := t.TempDir()
	bbInst := bb.New(tempDir)
	mod := NewModule(bbInst)

	dbPath := filepath.Join(tempDir, "snapshots.db")
	repo, err := db.NewSQLiteRepoWithPath(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	mod.SetStore(repo, "sess-test", "dev")

	ctx := context.Background()

	// 1. Snapshot with empty store fails gracefully if not configured
	emptyMod := NewModule(bbInst)
	res, err := emptyMod.Execute(ctx, "blackboard_snapshot", map[string]interface{}{"action": "save"})
	if err != nil || res.Success {
		t.Fatalf("expected failure on unconfigured store, got res=%+v", res)
	}

	// 2. Set state in blackboard
	_, _ = mod.Execute(ctx, "blackboard_set", map[string]interface{}{
		"key":   "global.phase",
		"value": "architecture_review",
	})

	// 3. Save snapshot
	saveRes, err := mod.Execute(ctx, "blackboard_snapshot", map[string]interface{}{
		"action": "save",
		"label":  "milestone-1",
	})
	if err != nil || !saveRes.Success {
		t.Fatalf("save snapshot failed: %v, res=%+v", err, saveRes)
	}

	// 4. List snapshots
	listRes, err := mod.Execute(ctx, "blackboard_snapshot", map[string]interface{}{
		"action": "list",
	})
	if err != nil || !listRes.Success {
		t.Fatalf("list snapshots failed: %v, res=%+v", err, listRes)
	}
	var metas []db.SnapshotMeta
	if err := json.Unmarshal([]byte(listRes.Output), &metas); err != nil {
		t.Fatalf("unmarshal list output failed: %v", err)
	}
	if len(metas) != 1 || metas[0].Label != "milestone-1" {
		t.Fatalf("unexpected snapshot list: %+v", metas)
	}
	snapID := metas[0].ID

	// 5. Mutate working memory (simulate change or loss)
	_, _ = mod.Execute(ctx, "blackboard_set", map[string]interface{}{
		"key":   "global.phase",
		"value": "broken_state",
	})

	// 6. Get snapshot inspect
	getRes, err := mod.Execute(ctx, "blackboard_snapshot", map[string]interface{}{
		"action":      "get",
		"snapshot_id": snapID,
	})
	if err != nil || !getRes.Success {
		t.Fatalf("get snapshot failed: %v, res=%+v", err, getRes)
	}
	if !strings.Contains(getRes.Output, "architecture_review") {
		t.Errorf("expected original value in snapshot get: %s", getRes.Output)
	}

	// 7. Restore snapshot
	restoreRes, err := mod.Execute(ctx, "blackboard_snapshot", map[string]interface{}{
		"action":      "restore",
		"snapshot_id": snapID,
	})
	if err != nil || !restoreRes.Success {
		t.Fatalf("restore snapshot failed: %v, res=%+v", err, restoreRes)
	}

	// 8. Verify restored value in blackboard
	verifyRes, err := mod.Execute(ctx, "blackboard_get", map[string]interface{}{
		"key": "global.phase",
	})
	if err != nil || !verifyRes.Success {
		t.Fatalf("verify get failed: %v", err)
	}
	if !strings.Contains(verifyRes.Output, "architecture_review") {
		t.Errorf("expected restored 'architecture_review', got: %s", verifyRes.Output)
	}
}

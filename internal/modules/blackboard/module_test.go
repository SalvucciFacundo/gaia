package blackboard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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

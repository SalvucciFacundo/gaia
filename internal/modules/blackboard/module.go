package blackboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	bb "gaia/internal/blackboard"
	"gaia/internal/core/domain"
)

// SnapshotStore defines the interface for persisting and querying blackboard snapshots.
type SnapshotStore interface {
	SaveBlackboardSnapshot(ctx context.Context, sessionID, profile, label string, entries map[string]bb.MemoryEntry) (string, error)
	GetBlackboardSnapshot(ctx context.Context, id string) (map[string]bb.MemoryEntry, *bb.SnapshotMeta, error)
	ListBlackboardSnapshots(ctx context.Context, sessionID, profile string) ([]bb.SnapshotMeta, error)
}

// Module implements the blackboard working memory tools plugin.
type Module struct {
	bb      *bb.Blackboard
	store   SnapshotStore
	sessID  string
	profile string
}

// NewModule creates a new blackboard module for GAIA.
func NewModule(blackboardInstance *bb.Blackboard) *Module {
	if blackboardInstance == nil {
		blackboardInstance = bb.New("")
	}
	return &Module{
		bb: blackboardInstance,
	}
}

// SetStore configures the snapshot persistence store, active session, and active profile.
func (m *Module) SetStore(store SnapshotStore, sessionID, profile string) {
	m.store = store
	m.sessID = sessionID
	m.profile = profile
}

// Name returns the module identifier.
func (m *Module) Name() string {
	return "blackboard"
}

// Description returns a concise summary of the module.
func (m *Module) Description() string {
	return "Provides thread-safe working memory (L1 Blackboard) with 4KB automatic disk spillover for sharing state between agents and workflow steps."
}

// GetTools returns the advertised tool definitions for registry registration.
func (m *Module) GetTools() []domain.ToolCall {
	return []domain.ToolCall{
		{Name: "blackboard_get"},
		{Name: "blackboard_set"},
		{Name: "blackboard_list"},
		{Name: "blackboard_snapshot"},
	}
}

// Tools returns detailed tool schemas for LLM provider advertisements.
func (m *Module) Tools() []domain.ToolDef {
	return []domain.ToolDef{
		{
			Name:        "blackboard_get",
			Description: "Retrieve a value from the shared working memory (Blackboard). If the payload exceeded 4KB and was spilled to disk, it automatically reads and returns the full content.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "The exact key to retrieve from the blackboard.",
					},
				},
				"required": []string{"key"},
			},
		},
		{
			Name:        "blackboard_set",
			Description: "Store a value in the shared working memory (Blackboard). Large payloads (>4KB) are automatically spilled to disk pass-by-reference. Keys should follow 'tasks.<task_id>.<name>' for task isolation or 'global.<name>' for shared facts.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "The key to store in the blackboard.",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "The content, string, or JSON payload to store.",
					},
					"producer_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional producer task ID for namespace enforcement.",
					},
				},
				"required": []string{"key", "value"},
			},
		},
		{
			Name:        "blackboard_list",
			Description: "List all keys currently stored in the working memory (Blackboard).",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "blackboard_snapshot",
			Description: "Manage audit-safe, SHA-256 cryptographically verified checkpoints of working memory (Blackboard). Supports action='save' (creates immutable checkpoint), action='list' (lists stored checkpoints), action='get' (inspects verified snapshot contents), and action='restore' (restores verified state to working memory).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"save", "list", "get", "restore"},
						"description": "The checkpoint operation to perform.",
					},
					"label": map[string]interface{}{
						"type":        "string",
						"description": "Descriptive label for the snapshot (used when action='save').",
					},
					"snapshot_id": map[string]interface{}{
						"type":        "string",
						"description": "The unique snapshot ID (required when action='get' or action='restore').",
					},
				},
				"required": []string{"action"},
			},
		},
	}
}

// Execute handles tool dispatch for blackboard operations.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch toolName {
	case "blackboard_get":
		keyVal, ok := args["key"]
		if !ok {
			return &domain.ToolResult{Success: false, Error: "missing required argument 'key'"}, nil
		}
		key, ok := keyVal.(string)
		if !ok || strings.TrimSpace(key) == "" {
			return &domain.ToolResult{Success: false, Error: "argument 'key' must be a non-empty string"}, nil
		}

		entry, err := m.bb.Get(key)
		if err != nil {
			return &domain.ToolResult{Success: false, Error: err.Error()}, nil
		}

		payload, err := bb.ReadPayload(entry)
		if err != nil {
			return &domain.ToolResult{Success: false, Error: fmt.Sprintf("failed to read payload: %v", err)}, nil
		}

		dataStr := string(payload)
		var unquoted string
		if json.Unmarshal(payload, &unquoted) == nil {
			dataStr = unquoted
		}

		resMap := map[string]interface{}{
			"key":         entry.Key,
			"is_spilled":  entry.IsSpilled,
			"producer_id": entry.ProducerID,
			"data":        dataStr,
		}
		if entry.IsSpilled {
			resMap["spill_path"] = entry.SpillPath
		}
		resJSON, _ := json.Marshal(resMap)

		return &domain.ToolResult{
			Success: true,
			Output:  string(resJSON),
		}, nil

	case "blackboard_set":
		keyVal, ok := args["key"]
		if !ok {
			return &domain.ToolResult{Success: false, Error: "missing required argument 'key'"}, nil
		}
		key, ok := keyVal.(string)
		if !ok || strings.TrimSpace(key) == "" {
			return &domain.ToolResult{Success: false, Error: "argument 'key' must be a non-empty string"}, nil
		}

		valVal, ok := args["value"]
		if !ok {
			return &domain.ToolResult{Success: false, Error: "missing required argument 'value'"}, nil
		}
		valStr, ok := valVal.(string)
		if !ok {
			return &domain.ToolResult{Success: false, Error: "argument 'value' must be a string"}, nil
		}

		producerID := ""
		if pVal, ok := args["producer_id"]; ok {
			if pStr, ok := pVal.(string); ok {
				producerID = pStr
			}
		}

		entry, err := m.bb.Set(producerID, key, []byte(valStr))
		if err != nil {
			return &domain.ToolResult{Success: false, Error: err.Error()}, nil
		}

		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Stored key %q (spilled: %v, size: %d bytes)", entry.Key, entry.IsSpilled, len(valStr)),
		}, nil

	case "blackboard_list":
		keys := m.bb.ListKeys()
		keysJSON, _ := json.Marshal(keys)
		return &domain.ToolResult{
			Success: true,
			Output:  string(keysJSON),
		}, nil

	case "blackboard_snapshot":
		if m.store == nil {
			return &domain.ToolResult{Success: false, Error: "blackboard snapshot store is not configured"}, nil
		}
		action, _ := args["action"].(string)
		switch action {
		case "save":
			label, _ := args["label"].(string)
			if label == "" {
				label = "agent-checkpoint"
			}
			snap := m.bb.Snapshot()
			id, err := m.store.SaveBlackboardSnapshot(ctx, m.sessID, m.profile, label, snap)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: fmt.Sprintf("failed to save snapshot: %v", err)}, nil
			}
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Snapshot %q saved successfully with %d entries (label: %q)", id, len(snap), label),
			}, nil

		case "list":
			list, err := m.store.ListBlackboardSnapshots(ctx, m.sessID, m.profile)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: fmt.Sprintf("failed to list snapshots: %v", err)}, nil
			}
			outJSON, _ := json.Marshal(list)
			return &domain.ToolResult{
				Success: true,
				Output:  string(outJSON),
			}, nil

		case "get":
			id, _ := args["snapshot_id"].(string)
			if id == "" {
				return &domain.ToolResult{Success: false, Error: "missing required argument 'snapshot_id'"}, nil
			}
			entries, meta, err := m.store.GetBlackboardSnapshot(ctx, id)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: fmt.Sprintf("failed to get snapshot %q: %v", id, err)}, nil
			}
			resMap := map[string]interface{}{
				"meta":    meta,
				"entries": entries,
			}
			outJSON, _ := json.Marshal(resMap)
			return &domain.ToolResult{
				Success: true,
				Output:  string(outJSON),
			}, nil

		case "restore":
			id, _ := args["snapshot_id"].(string)
			if id == "" {
				return &domain.ToolResult{Success: false, Error: "missing required argument 'snapshot_id'"}, nil
			}
			entries, meta, err := m.store.GetBlackboardSnapshot(ctx, id)
			if err != nil {
				return &domain.ToolResult{Success: false, Error: fmt.Sprintf("security check or restore failed: %v", err)}, nil
			}
			m.bb.Restore(entries)
			return &domain.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Successfully restored %d entries from snapshot %q (SHA-256 verified: %s, label: %q)", meta.EntryCount, meta.ID, meta.SHA256, meta.Label),
			}, nil

		default:
			return &domain.ToolResult{Success: false, Error: fmt.Sprintf("unknown snapshot action: %s (must be save, list, get, or restore)", action)}, nil
		}

	default:
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}
}

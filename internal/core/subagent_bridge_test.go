package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gaia/internal/core/domain"
)

type mockSyncPort struct {
	available []string
	spawnErr  error
	result    *domain.SubagentResult
}

func (m *mockSyncPort) Spawn(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	if m.spawnErr != nil {
		return nil, m.spawnErr
	}
	if m.result != nil {
		return m.result, nil
	}
	return &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "mock success"}, nil
}

func (m *mockSyncPort) Available() []string {
	return m.available
}

type mockAsyncPort struct {
	mockSyncPort
	asyncID  string
	asyncErr error
}

func (m *mockAsyncPort) SpawnAsync(ctx context.Context, name string, task domain.SubagentTask) (string, error) {
	if m.asyncErr != nil {
		return "", m.asyncErr
	}
	return m.asyncID, nil
}

func (m *mockAsyncPort) TaskManager() interface{} {
	return nil
}

func (m *mockAsyncPort) RunPipeline(ctx context.Context, phases []domain.PipelinePhase, baseTask domain.SubagentTask) ([]*domain.SubagentResult, error) {
	return nil, nil
}

type mockKGRecorder struct {
	recordedResult bool
}

func (m *mockKGRecorder) RecordToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result *domain.ToolResult) {}
func (m *mockKGRecorder) RecordResponseFacts(ctx context.Context, response string) {}
func (m *mockKGRecorder) RecordSubagentResult(ctx context.Context, name, taskDesc string, result *domain.SubagentResult) {
	m.recordedResult = true
}

func TestBrain_Delegate_PortNotWired(t *testing.T) {
	b := &Brain{}
	_, err := b.Delegate(context.Background(), "explorer", domain.SubagentTask{Description: "explore"})
	if !errors.Is(err, ErrSubagentPortNotWired) {
		t.Fatalf("expected ErrSubagentPortNotWired, got %v", err)
	}
}

func TestBrain_Delegate_ContextCancelled(t *testing.T) {
	b := &Brain{
		subagentPort: &mockSyncPort{available: []string{"explorer"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := b.Delegate(ctx, "explorer", domain.SubagentTask{Description: "explore"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestBrain_Delegate_SuccessAndKGRecord(t *testing.T) {
	recorder := &mockKGRecorder{}
	b := &Brain{
		subagentPort: &mockSyncPort{
			available: []string{"explorer"},
			result:    &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "found files"},
		},
		kgRecorder: recorder,
		skillLister: func() []string {
			return []string{"golang-testing"}
		},
	}

	res, err := b.Delegate(context.Background(), "explorer", domain.SubagentTask{Description: "explore codebase"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Summary != "found files" {
		t.Fatalf("unexpected summary: %s", res.Summary)
	}
	if !recorder.recordedResult {
		t.Fatalf("expected KG recorder to record subagent result")
	}
}

func TestBrain_HandleDirectSubagent_Routing(t *testing.T) {
	t.Run("Unknown subagent returns error", func(t *testing.T) {
		ui := &stubUI{}
		b := &Brain{
			subagentPort: &mockSyncPort{available: []string{"coder"}},
			ui:           ui,
		}
		err := b.HandleDirectSubagent(context.Background(), "@unknown please help")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(ui.displayed) == 0 || !strings.Contains(ui.displayed[0].Content, "Unknown subagent") {
			t.Fatalf("expected unknown subagent message displayed")
		}
	})

	t.Run("Async spawner short ID safety", func(t *testing.T) {
		ui := &stubUI{}
		b := &Brain{
			subagentPort: &mockAsyncPort{
				mockSyncPort: mockSyncPort{available: []string{"coder"}},
				asyncID:      "short", // length < 8
			},
			ui: ui,
		}
		err := b.HandleDirectSubagent(context.Background(), "@coder do something")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(ui.displayed) == 0 || !strings.Contains(ui.displayed[0].Content, "task short") {
			t.Fatalf("expected short task ID to display without slicing panic, got: %v", ui.displayed)
		}
	})

	t.Run("Synchronous fallback with nil UI returns err", func(t *testing.T) {
		b := &Brain{
			subagentPort: &mockSyncPort{
				available: []string{"coder"},
				spawnErr:  errors.New("spawn failed"),
			},
			ui: nil,
		}
		err := b.HandleDirectSubagent(context.Background(), "@coder do work")
		if err == nil || !strings.Contains(err.Error(), "spawn failed") {
			t.Fatalf("expected spawn error propagated when UI is nil, got %v", err)
		}
	})
}

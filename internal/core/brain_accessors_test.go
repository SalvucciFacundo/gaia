package core

import (
	"context"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type dummyProvider struct{}

func (d *dummyProvider) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	return &domain.Message{Role: domain.RoleAssistant, Content: "ok"}, nil
}

func (d *dummyProvider) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, nil
}

func (d *dummyProvider) Tools() []domain.ToolDef {
	return nil
}

func (d *dummyProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{"dummy-model"}, nil
}

func TestBrain_AccessorsConcurrency(t *testing.T) {
	b := &Brain{
		steerCh: make(chan string, 1),
	}

	p1 := &dummyProvider{}
	p2 := &dummyProvider{}

	done := make(chan bool)

	// Goroutine 1: mutators
	go func() {
		for i := 0; i < 50; i++ {
			b.SetActiveProvider("prov-a", p1)
			b.SetAvailableProviders(map[string]ports.LLMProvider{"p1": p1, "p2": p2})
			b.SetMCPManager(nil)
			b.SetGoal("goal")
			b.AddSubgoal("sub")
			b.SetFastMode(true, "orig", "fast")
			b.SetFastMode(false, "orig", "fast")
			b.SetBusyMode("queue")
			b.SetPendingConfirm("tool")
			b.AddPendingImage(domain.ImageContent{MIMEType: "image/png", Data: "aGVsbG8="})
			b.ClearPendingImages()
			b.ClearGoal()
		}
		done <- true
	}()

	// Goroutine 2: readers
	go func() {
		for i := 0; i < 50; i++ {
			_ = b.ProviderName()
			_ = b.ModelName()
			_ = b.AvailableProviders()
			_ = b.MCPManager()
			_ = b.Goal()
			_ = b.Subgoals()
			_, _, _ = b.FastModeState()
			_ = b.BusyMode()
			_ = b.PendingConfirm()
			_ = b.PendingImages()
		}
		done <- true
	}()

	<-done
	<-done
}

func TestBrain_AvailableProvidersDeepCopy(t *testing.T) {
	b := &Brain{}
	p1 := &dummyProvider{}
	m := map[string]ports.LLMProvider{"p1": p1}

	b.SetAvailableProviders(m)
	m["p2"] = &dummyProvider{}

	got := b.AvailableProviders()
	if len(got) != 1 {
		t.Fatalf("expected 1 provider in Brain, got %d", len(got))
	}

	got["p3"] = &dummyProvider{}
	gotAgain := b.AvailableProviders()
	if len(gotAgain) != 1 {
		t.Fatalf("expected 1 provider after external mutation, got %d", len(gotAgain))
	}
}

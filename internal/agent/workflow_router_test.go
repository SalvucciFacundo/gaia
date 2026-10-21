package agent

import (
	"context"
	"strings"
	"testing"

	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type routerStubProvider struct{}

func (s *routerStubProvider) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	return &domain.Message{Role: domain.RoleAssistant, Content: "ok"}, nil
}
func (s *routerStubProvider) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, nil
}
func (s *routerStubProvider) Tools() []domain.ToolDef                          { return nil }
func (s *routerStubProvider) ListModels(ctx context.Context) ([]string, error) { return nil, nil }

type routerStubRepo struct {
	saved []domain.Message
}

func (r *routerStubRepo) SaveMessage(_ context.Context, msg domain.Message) error {
	r.saved = append(r.saved, msg)
	return nil
}
func (r *routerStubRepo) GetHistory(_ context.Context, _ int) ([]domain.Message, error) {
	return r.saved, nil
}
func (r *routerStubRepo) CreateSession(_ context.Context, _ string) (string, error) {
	return "s1", nil
}
func (r *routerStubRepo) GetMessages(_ context.Context, _ string, _ int) ([]domain.Message, error) {
	return nil, nil
}
func (r *routerStubRepo) GetMessageCount(_ context.Context) (int, error) {
	return len(r.saved), nil
}
func (r *routerStubRepo) GetHistoryFrom(_ context.Context, _, _ int) ([]domain.Message, error) {
	return nil, nil
}
func (r *routerStubRepo) GetLastMessages(_ context.Context, _ int) ([]domain.Message, error) {
	return nil, nil
}
func (r *routerStubRepo) DeleteMessagesAfter(_ context.Context, _ string) error { return nil }
func (r *routerStubRepo) ClearMessages(_ context.Context) error                 { return nil }
func (r *routerStubRepo) ListSessions(_ context.Context) ([]domain.SessionInfo, error) {
	return nil, nil
}
func (r *routerStubRepo) RenameSession(_ context.Context, _, _ string) error { return nil }

type routerStubUI struct {
	displayed []domain.Message
}

func (u *routerStubUI) Display(msg domain.Message) error {
	u.displayed = append(u.displayed, msg)
	return nil
}
func (u *routerStubUI) AppendToken(_ string) error                           { return nil }
func (u *routerStubUI) PromptConfirmation(_ string) (bool, error)            { return true, nil }
func (u *routerStubUI) PromptChoice(_ domain.ChoiceEnvelope) (string, error) { return "", nil }
func (u *routerStubUI) Run() error                                           { return nil }

type routerStubSubagentPort struct {
	available []string
	spawnRes  *domain.SubagentResult
}

func (s *routerStubSubagentPort) Available() []string {
	return s.available
}

func (s *routerStubSubagentPort) Spawn(_ context.Context, _ string, _ domain.SubagentTask) (*domain.SubagentResult, error) {
	if s.spawnRes != nil {
		return s.spawnRes, nil
	}
	return &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "worker finished"}, nil
}

func TestWorkflowRouter_RouteTurn_SuggestFeature(t *testing.T) {
	ui := &routerStubUI{}
	repo := &routerStubRepo{}
	brain := core.NewBrain(&routerStubProvider{}, repo, ui, nil, domain.BudgetConfig{MaxIterations: 5})
	wr := NewWorkflowRouter(brain)

	handled, err := wr.RouteTurn(context.Background(), "crea un nuevo subsistema de colas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true for architectural suggestion")
	}

	if len(ui.displayed) == 0 {
		t.Fatal("expected UI display of suggestion")
	}
	if !strings.Contains(ui.displayed[0].Content, "Architectural Scope Detected") {
		t.Errorf("expected suggestion text, got %q", ui.displayed[0].Content)
	}
}

func TestWorkflowRouter_RouteTurn_DelegatedWorker(t *testing.T) {
	ui := &routerStubUI{}
	repo := &routerStubRepo{}
	brain := core.NewBrain(&routerStubProvider{}, repo, ui, nil, domain.BudgetConfig{MaxIterations: 5})
	brain.SetSubagentPort(&routerStubSubagentPort{
		available: []string{"explorer"},
	})
	wr := NewWorkflowRouter(brain)

	handled, err := wr.RouteTurn(context.Background(), "please map codebase and trace all references")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true for explorer worker")
	}

	if len(ui.displayed) == 0 {
		t.Fatal("expected worker completion message")
	}
	if !strings.Contains(ui.displayed[0].Content, "ODD Delegated Worker (explorer)") {
		t.Errorf("expected explorer completion, got: %s", ui.displayed[0].Content)
	}
}

func TestWorkflowRouter_RouteTurn_InlineFallback(t *testing.T) {
	ui := &routerStubUI{}
	repo := &routerStubRepo{}
	brain := core.NewBrain(&routerStubProvider{}, repo, ui, nil, domain.BudgetConfig{MaxIterations: 5})
	wr := NewWorkflowRouter(brain)

	handled, err := wr.RouteTurn(context.Background(), "fix typo in readme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Error("expected handled=false for simple prompt (should proceed inline)")
	}
}

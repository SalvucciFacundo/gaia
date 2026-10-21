package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type stubProvider struct {
	resp *domain.Message
}

func (s *stubProvider) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	if s.resp != nil {
		return s.resp, nil
	}
	return &domain.Message{Role: domain.RoleAssistant, Content: "stub response"}, nil
}

func (s *stubProvider) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, nil
}

func (s *stubProvider) Tools() []domain.ToolDef                          { return nil }
func (s *stubProvider) ListModels(ctx context.Context) ([]string, error) { return nil, nil }

type stubRepo struct {
	messages []domain.Message
}

func (r *stubRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	r.messages = append(r.messages, msg)
	return nil
}

func (r *stubRepo) GetHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	return r.messages, nil
}

func (r *stubRepo) CreateSession(ctx context.Context, name string) (string, error) {
	return "test-session", nil
}

func (r *stubRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}

func (r *stubRepo) GetMessageCount(ctx context.Context) (int, error) {
	return len(r.messages), nil
}

func (r *stubRepo) ListSessions(ctx context.Context) ([]domain.SessionInfo, error) {
	return nil, nil
}

func (r *stubRepo) GetHistoryFrom(ctx context.Context, limit, offset int) ([]domain.Message, error) {
	if offset >= len(r.messages) {
		return []domain.Message{}, nil
	}
	end := offset + limit
	if end > len(r.messages) {
		end = len(r.messages)
	}
	return r.messages[offset:end], nil
}

func (r *stubRepo) GetLastMessages(ctx context.Context, n int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}

func (r *stubRepo) DeleteMessagesAfter(ctx context.Context, afterID string) error {
	return nil
}

func (r *stubRepo) ClearMessages(ctx context.Context) error {
	r.messages = nil
	return nil
}

func (r *stubRepo) RenameSession(ctx context.Context, sessionID, name string) error {
	return nil
}

type stubUI struct {
	displayed []domain.Message
}

func (u *stubUI) Display(msg domain.Message) error {
	u.displayed = append(u.displayed, msg)
	return nil
}

func (u *stubUI) AppendToken(content string) error               { return nil }
func (u *stubUI) PromptConfirmation(prompt string) (bool, error) { return true, nil }
func (u *stubUI) PromptChoice(envelope domain.ChoiceEnvelope) (string, error) {
	if len(envelope.Questions) > 0 && len(envelope.Questions[0].Options) > 0 {
		return envelope.Questions[0].Options[0].Token, nil
	}
	return "", nil
}
func (u *stubUI) Run() error { return nil }

func TestDispatcher_AttachImage(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F,
		0x00, 0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59,
		0xE7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	_ = os.WriteFile(imgPath, pngData, 0644)

	ui := &stubUI{}
	repo := &stubRepo{}
	brain := core.NewBrain(&stubProvider{}, repo, ui, nil, domain.BudgetConfig{MaxIterations: 25})
	d := New(brain)

	handled, err := d.Dispatch(context.Background(), "/image "+imgPath)
	if !handled || err != nil {
		t.Fatalf("expected handled=true, err=nil, got handled=%v, err=%v", handled, err)
	}

	if len(ui.displayed) != 1 {
		t.Fatalf("expected 1 displayed message, got %d", len(ui.displayed))
	}
	if !strings.Contains(ui.displayed[0].Content, "Image attached") {
		t.Errorf("expected 'Image attached' message, got: %s", ui.displayed[0].Content)
	}
}

func TestDispatcher_FastMode(t *testing.T) {
	prov := &stubProvider{}
	repo := &stubRepo{}
	ui := &stubUI{}
	brain := core.NewBrain(prov, repo, ui, nil, domain.BudgetConfig{MaxIterations: 5})
	brain.SetActiveProvider("gpt-4o", prov)
	brain.SetAvailableProviders(map[string]ports.LLMProvider{
		"gpt-4o":      prov,
		"gpt-4o-mini": &stubProvider{},
	})

	d := New(brain)
	ctx := context.Background()

	// 1. Enable fast mode
	handled, err := d.Dispatch(ctx, "/fast on")
	if !handled || err != nil {
		t.Fatalf("FastMode on failed: %v", err)
	}
	enabled, orig, _ := brain.FastModeState()
	if !enabled {
		t.Error("expected fastModeEnabled to be true")
	}
	if brain.ProviderName() != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %q", brain.ProviderName())
	}
	if orig != "gpt-4o" {
		t.Errorf("expected originalModel 'gpt-4o', got %q", orig)
	}

	// 2. Disable fast mode
	handled, err = d.Dispatch(ctx, "/fast off")
	if !handled || err != nil {
		t.Fatalf("FastMode off failed: %v", err)
	}
	enabled, _, _ = brain.FastModeState()
	if enabled {
		t.Error("expected fastModeEnabled to be false")
	}
	if brain.ProviderName() != "gpt-4o" {
		t.Errorf("expected restored model 'gpt-4o', got %q", brain.ProviderName())
	}
}

func TestDispatcher_BusyMode(t *testing.T) {
	ui := &stubUI{}
	brain := core.NewBrain(&stubProvider{}, &stubRepo{}, ui, nil, domain.BudgetConfig{MaxIterations: 5})
	d := New(brain)
	ctx := context.Background()

	// Default status
	handled, err := d.Dispatch(ctx, "/busy status")
	if !handled || err != nil {
		t.Fatalf("dispatch /busy status failed: %v", err)
	}
	if len(ui.displayed) != 1 || !strings.Contains(ui.displayed[0].Content, "queue") {
		t.Errorf("expected default busy mode 'queue', got: %v", ui.displayed)
	}

	// Set to steer
	handled, err = d.Dispatch(ctx, "/busy steer")
	if !handled || err != nil {
		t.Fatalf("dispatch /busy steer failed: %v", err)
	}
	if brain.BusyMode() != "steer" {
		t.Errorf("expected busyMode 'steer', got %q", brain.BusyMode())
	}

	// Set to ignore
	handled, err = d.Dispatch(ctx, "/busy ignore")
	if !handled || err != nil {
		t.Fatalf("dispatch /busy ignore failed: %v", err)
	}
	if brain.BusyMode() != "ignore" {
		t.Errorf("expected busyMode 'ignore', got %q", brain.BusyMode())
	}
}

func TestDispatcher_ModelsCommand(t *testing.T) {
	prov := &stubProvider{}
	repo := &stubRepo{}
	ui := &stubUI{}
	brain := core.NewBrain(prov, repo, ui, nil, domain.BudgetConfig{MaxIterations: 5})
	brain.SetAvailableProviders(map[string]ports.LLMProvider{
		"primary":  prov,
		"fallback": prov,
	})
	brain.SetActiveProvider("primary", prov)

	d := New(brain)
	ctx := context.Background()

	handled, err := d.Dispatch(ctx, "/models")
	if !handled || err != nil {
		t.Fatalf("dispatch /models failed: %v", err)
	}
	if len(ui.displayed) == 0 {
		t.Fatal("expected UI display for /models")
	}
	lastMsg := ui.displayed[len(ui.displayed)-1]
	if !strings.Contains(lastMsg.Content, "Available models") {
		t.Errorf("expected model listing in output, got: %s", lastMsg.Content)
	}

	// Switch with /models fallback
	handled, err = d.Dispatch(ctx, "/models fallback")
	if !handled || err != nil {
		t.Fatalf("dispatch /models fallback failed: %v", err)
	}
	if brain.ProviderName() != "fallback" {
		t.Errorf("expected providerName 'fallback', got %q", brain.ProviderName())
	}
}

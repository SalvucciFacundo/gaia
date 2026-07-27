package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// stubProvider records calls and returns canned responses.
type stubProvider struct {
	chatCalls   int
	streamCalls int
	resp        *domain.Message
	chatErr     error
	streamErr   error
}

func (s *stubProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	s.chatCalls++
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.resp, nil
}

func (s *stubProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	s.streamCalls++
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		fmt.Fprintf(pw, `{"content":"%s","done":true}`+"\n", s.resp.Content)
	}()
	return pr, nil
}

func (s *stubProvider) Tools() []domain.ToolDef { return nil }
func (s *stubProvider) ListModels(ctx context.Context) ([]string, error) { return nil, nil }

// stubRepo is a no-op repository for tests.
type stubRepo struct{}

func (r *stubRepo) SaveMessage(ctx context.Context, msg domain.Message) error    { return nil }
func (r *stubRepo) GetHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}
func (r *stubRepo) CreateSession(ctx context.Context, name string) (string, error) {
	return "test-session", nil
}
func (r *stubRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}
func (r *stubRepo) GetMessageCount(ctx context.Context) (int, error)     { return 0, nil }
func (r *stubRepo) ListSessions(ctx context.Context) ([]domain.SessionInfo, error) {
	return nil, nil
}
func (r *stubRepo) GetHistoryFrom(ctx context.Context, limit, offset int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}
func (r *stubRepo) GetLastMessages(ctx context.Context, n int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}
func (r *stubRepo) DeleteMessagesAfter(ctx context.Context, afterID string) error {
	return nil
}
func (r *stubRepo) ClearMessages(ctx context.Context) error {
	return nil
}
func (r *stubRepo) RenameSession(ctx context.Context, sessionID, name string) error {
	return nil
}

// stubUI records Display calls.
type stubUI struct {
	displayed []domain.Message
}

func (u *stubUI) Display(msg domain.Message) error {
	u.displayed = append(u.displayed, msg)
	return nil
}
func (u *stubUI) AppendToken(content string) error { return nil }
func (u *stubUI) PromptConfirmation(prompt string) (bool, error)   { return true, nil }
func (u *stubUI) Run() error                                        { return nil }

func TestBrain_BudgetExhausted(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "I'll call a tool",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "fake_tool", Arguments: map[string]interface{}{}},
			},
		},
		// Fail streaming so the brain falls back to Chat (which returns tool calls).
		streamErr: io.EOF,
	}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 3}
	brain := NewBrain(prov, &stubRepo{}, ui, nil, budget)

	err := brain.ProcessMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With 3 iterations and tool calls every time, we should hit budget.
	if len(ui.displayed) == 0 {
		t.Fatal("expected at least one displayed message")
	}
	last := ui.displayed[len(ui.displayed)-1]
	if last.Content != "Iteration budget exhausted (3 iterations). Stopping." {
		t.Errorf("expected budget exhausted message, got %q", last.Content)
	}
}

func TestBrain_SimpleChat(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Hello, how can I help?",
		},
	}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 25}
	brain := NewBrain(prov, &stubRepo{}, ui, nil, budget)

	err := brain.ProcessMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ui.displayed) != 1 {
		t.Fatalf("expected 1 displayed msg, got %d", len(ui.displayed))
	}
	if ui.displayed[0].Content != "Hello, how can I help?" {
		t.Errorf("unexpected response: %q", ui.displayed[0].Content)
	}
}

func TestAttachImage_ValidPNG(t *testing.T) {
	// Create a minimal valid PNG file.
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")

	// Minimal valid PNG file (1x1 red pixel).
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F,
		0x00, 0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59,
		0xE7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(imgPath, pngData, 0644); err != nil {
		t.Fatalf("failed to write test PNG: %v", err)
	}

	ui := &stubUI{}
	brain := NewBrain(&stubProvider{}, &stubRepo{}, ui, nil, domain.BudgetConfig{MaxIterations: 25})

	err := brain.AttachImage(context.Background(), imgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(brain.pendingImages) != 1 {
		t.Fatalf("expected 1 pending image, got %d", len(brain.pendingImages))
	}
	if brain.pendingImages[0].MIMEType != "image/png" {
		t.Errorf("expected image/png, got %s", brain.pendingImages[0].MIMEType)
	}
	if brain.pendingImages[0].Data == "" {
		t.Error("expected non-empty base64 data")
	}

	// Verify confirmation was displayed.
	if len(ui.displayed) != 1 {
		t.Fatalf("expected 1 displayed message, got %d", len(ui.displayed))
	}
	if !contains(ui.displayed[0].Content, "Image attached:") {
		t.Errorf("expected confirmation, got %q", ui.displayed[0].Content)
	}
}

func TestAttachImage_ProcessMessageWiresImages(t *testing.T) {
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
	os.WriteFile(imgPath, pngData, 0644)

	ui := &stubUI{}
	// stub that records messages saved.
	repo := &recordingRepo{
		stubRepo: stubRepo{},
	}
	prov := &stubProvider{
		resp: &domain.Message{Role: domain.RoleAssistant, Content: "I see an image!"},
	}
	brain := NewBrain(prov, repo, ui, nil, domain.BudgetConfig{MaxIterations: 25})

	// Attach an image first.
	if err := brain.AttachImage(context.Background(), imgPath); err != nil {
		t.Fatalf("AttachImage failed: %v", err)
	}

	if len(brain.pendingImages) != 1 {
		t.Fatal("expected 1 pending image after AttachImage")
	}

	// Process a message — images should be wired in.
	err := brain.ProcessMessage(context.Background(), "What do you see?")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// pendingImages should be cleared after ProcessMessage.
	if len(brain.pendingImages) != 0 {
		t.Errorf("expected pendingImages to be cleared, got %d", len(brain.pendingImages))
	}
}

// recordingRepo wraps stubRepo and records saved messages.
type recordingRepo struct {
	stubRepo
	saved []domain.Message
}

func (r *recordingRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	r.saved = append(r.saved, msg)
	return nil
}

func TestAttachImage_FileNotFound(t *testing.T) {
	ui := &stubUI{}
	brain := NewBrain(&stubProvider{}, &stubRepo{}, ui, nil, domain.BudgetConfig{MaxIterations: 25})

	err := brain.AttachImage(context.Background(), "nonexistent.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ui.displayed) != 1 {
		t.Fatalf("expected 1 displayed message, got %d", len(ui.displayed))
	}
	if !contains(ui.displayed[0].Content, "Image not found") {
		t.Errorf("expected 'Image not found', got %q", ui.displayed[0].Content)
	}
}

func TestAttachImage_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.gif")
	os.WriteFile(imgPath, []byte("GIF89a"), 0644)

	ui := &stubUI{}
	brain := NewBrain(&stubProvider{}, &stubRepo{}, ui, nil, domain.BudgetConfig{MaxIterations: 25})

	err := brain.AttachImage(context.Background(), imgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(ui.displayed[0].Content, "Unsupported image format") {
		t.Errorf("expected 'Unsupported image format', got %q", ui.displayed[0].Content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}


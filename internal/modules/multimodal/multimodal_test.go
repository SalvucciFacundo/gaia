package multimodal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type mockLLM struct{}

func (m *mockLLM) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	return &domain.Message{
		Role:    domain.RoleAssistant,
		Content: "Visual Analysis:\n- Found header with cyan text\n- Form input detected\n- Submit button present",
	}, nil
}

func (m *mockLLM) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, nil
}

func (m *mockLLM) Tools() []domain.ToolDef {
	return nil
}

func (m *mockLLM) ListModels(ctx context.Context) ([]string, error) {
	return []string{"mock-vision"}, nil
}

func TestMultimodalModule_GetTools(t *testing.T) {
	cfg := domain.MultimodalConfig{
		Enabled:  true,
		Provider: "gemini",
		Model:    "gemini-2.5-flash",
	}
	mod := NewModule(cfg, nil)

	if mod.Name() != "multimodal" {
		t.Errorf("expected name multimodal, got %s", mod.Name())
	}

	tools := mod.GetTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "inspect_media" {
		t.Errorf("expected tool inspect_media, got %s", tools[0].Name)
	}
}

func TestMultimodalModule_Execute_Validation(t *testing.T) {
	cfg := domain.MultimodalConfig{Enabled: true}
	mod := NewModule(cfg, nil)

	// Missing media_path
	res, err := mod.Execute(context.Background(), "inspect_media", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || len(res.Output) == 0 {
		t.Fatal("expected error JSON response")
	}

	// Invalid media_type
	res, err = mod.Execute(context.Background(), "inspect_media", map[string]interface{}{
		"media_path": "test.png",
		"media_type": "invalid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultimodalModule_Execute_LocalImage(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "sample.png")
	// Write dummy image bytes
	_ = os.WriteFile(imgPath, []byte("fake png content"), 0644)

	cfg := domain.MultimodalConfig{
		Enabled:  true,
		Provider: "gemini",
		Model:    "gemini-2.5-flash",
	}
	mod := NewModule(cfg, &mockLLM{})

	res, err := mod.Execute(context.Background(), "inspect_media", map[string]interface{}{
		"media_path": imgPath,
		"media_type": "image",
		"prompt":     "Analyze UI components",
	})
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if res == nil || len(res.Output) == 0 {
		t.Fatal("expected non-empty JSON string response")
	}
}

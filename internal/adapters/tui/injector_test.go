package tui

import (
	"context"
	"os"
	"strings"
	"testing"
)

type dummyAttacher struct {
	attached []string
}

func (d *dummyAttacher) AttachImage(ctx context.Context, path string) error {
	d.attached = append(d.attached, path)
	return nil
}

func TestIngestMentions_TextFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gaia-injector-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Create test files
	_ = os.WriteFile("config.yaml", []byte("name: gaia\nport: 8080"), 0644)
	_ = os.WriteFile("test.png", []byte("\x89PNG\r\n\x1a\n"), 0644)

	attacher := &dummyAttacher{}

	input := "Check @config.yaml and image @test.png please"
	res, err := IngestMentions(context.Background(), attacher, input, nil)
	if err != nil {
		t.Fatalf("IngestMentions failed: %v", err)
	}

	// Verify text file content was injected
	if !strings.Contains(res, "[Attached File: config.yaml]") {
		t.Errorf("expected attached file banner, got: %s", res)
	}
	if !strings.Contains(res, "port: 8080") {
		t.Errorf("expected file content in prompt, got: %s", res)
	}

	// Verify image was passed to attacher
	if len(attacher.attached) != 1 || attacher.attached[0] != "test.png" {
		t.Errorf("expected test.png to be attached, got: %v", attacher.attached)
	}
}

func TestIngestMentions_IgnoresSubagent(t *testing.T) {
	attacher := &dummyAttacher{}
	subagents := []string{"implementer", "reviewer"}

	input := "Send task to @implementer please"
	res, err := IngestMentions(context.Background(), attacher, input, subagents)
	if err != nil {
		t.Fatal(err)
	}

	if res != input {
		t.Errorf("expected input to remain unmodified for subagent mention, got: %s", res)
	}
	if len(attacher.attached) > 0 {
		t.Errorf("expected no attached images, got: %v", attacher.attached)
	}
}

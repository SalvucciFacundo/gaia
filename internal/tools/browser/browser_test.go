package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gaia/internal/core/domain"
)

type mockDriver struct {
	navFunc        func(ctx context.Context, targetURL string) (*PageResult, error)
	actionFunc     func(ctx context.Context, req ActionRequest) (*ActionResult, error)
	screenshotFunc func(ctx context.Context, targetURL, destPath string) (string, error)
	closeFunc      func() error
}

func (m *mockDriver) Navigate(ctx context.Context, targetURL string) (*PageResult, error) {
	if m.navFunc != nil {
		return m.navFunc(ctx, targetURL)
	}
	return &PageResult{
		URL:     targetURL,
		Title:   "Mock Title",
		Content: "# Mock Content\n\nHello world",
	}, nil
}

func (m *mockDriver) Action(ctx context.Context, req ActionRequest) (*ActionResult, error) {
	if m.actionFunc != nil {
		return m.actionFunc(ctx, req)
	}
	return &ActionResult{
		Success: true,
		Output:  "action completed: " + req.Action,
	}, nil
}

func (m *mockDriver) Screenshot(ctx context.Context, targetURL, destPath string) (string, error) {
	if m.screenshotFunc != nil {
		return m.screenshotFunc(ctx, targetURL, destPath)
	}
	return destPath, nil
}

func (m *mockDriver) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestBrowserNavigateRunner(t *testing.T) {
	mock := &mockDriver{
		navFunc: func(ctx context.Context, targetURL string) (*PageResult, error) {
			if targetURL == "https://fail.com" {
				return nil, errors.New("connection refused")
			}
			return &PageResult{
				URL:     targetURL,
				Title:   "Example Page",
				Content: "Sample page text",
			}, nil
		},
	}

	cfg := domain.BrowserConfig{
		Enabled: true,
		Timeout: 5 * time.Second,
	}

	runner := NewNavigateRunner(cfg, mock)
	if runner.Name() != "browser_navigate" {
		t.Errorf("Name() = %q, want 'browser_navigate'", runner.Name())
	}
	if runner.Backend() != "browser" {
		t.Errorf("Backend() = %q, want 'browser'", runner.Backend())
	}
	if runner.Description() == "" {
		t.Errorf("Description() is empty")
	}

	ctx := context.Background()

	// 1. Successful navigation JSON input
	out, err := runner.Run(ctx, `{"url": "https://example.com"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, "Example Page") {
		t.Errorf("expected output to contain title, got: %s", out)
	}
	if !strings.Contains(out, "Sample page text") {
		t.Errorf("expected output to contain content, got: %s", out)
	}

	// 2. Successful navigation raw string input
	out, err = runner.Run(ctx, `https://example.com`)
	if err != nil {
		t.Fatalf("Run() with raw URL error = %v", err)
	}
	if !strings.Contains(out, "Example Page") {
		t.Errorf("expected output to contain title, got: %s", out)
	}

	// 3. Missing url
	if _, err = runner.Run(ctx, `{}`); err == nil {
		t.Errorf("expected error for empty json url, got nil")
	}
	if _, err = runner.Run(ctx, ""); err == nil {
		t.Errorf("expected error for empty string, got nil")
	}
	if _, err = runner.Run(ctx, `{"url": ""}`); err == nil {
		t.Errorf("expected error for empty url string, got nil")
	}

	// 4. Invalid json
	if _, err = runner.Run(ctx, `{"url": `); err == nil {
		t.Errorf("expected error for invalid json, got nil")
	}

	// 5. Navigation failure
	if _, err = runner.Run(ctx, `https://fail.com`); err == nil {
		t.Errorf("expected error for failed navigation, got nil")
	}

	// 6. Nil driver
	nilRunner := NewNavigateRunner(cfg, nil)
	if _, err = nilRunner.Run(ctx, "https://example.com"); err == nil {
		t.Errorf("expected error for nil driver, got nil")
	}
}

func TestBrowserActionRunner(t *testing.T) {
	mock := &mockDriver{
		actionFunc: func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
			if req.Action == "invalid" {
				return nil, errors.New("unsupported action")
			}
			return &ActionResult{
				Success: true,
				Output:  "clicked button " + req.Selector,
			}, nil
		},
	}

	cfg := domain.BrowserConfig{Enabled: true}
	runner := NewActionRunner(cfg, mock)
	if runner.Name() != "browser_action" {
		t.Errorf("Name() = %q, want 'browser_action'", runner.Name())
	}
	if runner.Backend() != "browser" {
		t.Errorf("Backend() = %q, want 'browser'", runner.Backend())
	}
	if runner.Description() == "" {
		t.Errorf("Description() is empty")
	}

	ctx := context.Background()

	// 1. Success with JSON
	out, err := runner.Run(ctx, `{"action": "click", "selector": "#submit-btn"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, "clicked button #submit-btn") {
		t.Errorf("unexpected output: %s", out)
	}

	// 2. Success with plain string action
	out, err = runner.Run(ctx, "scroll")
	if err != nil {
		t.Fatalf("Run() with plain string action error = %v", err)
	}
	if !strings.Contains(out, "clicked button") {
		t.Errorf("unexpected output: %s", out)
	}

	// 3. Invalid action
	if _, err = runner.Run(ctx, `{"action": "invalid"}`); err == nil {
		t.Errorf("expected error for invalid action, got nil")
	}

	// 4. Missing action
	if _, err = runner.Run(ctx, `{}`); err == nil {
		t.Errorf("expected error for missing action, got nil")
	}
	if _, err = runner.Run(ctx, ""); err == nil {
		t.Errorf("expected error for empty action, got nil")
	}

	// 5. Invalid JSON
	if _, err = runner.Run(ctx, `{"action": `); err == nil {
		t.Errorf("expected error for invalid json, got nil")
	}

	// 6. Nil driver
	nilRunner := NewActionRunner(cfg, nil)
	if _, err = nilRunner.Run(ctx, "click"); err == nil {
		t.Errorf("expected error for nil driver, got nil")
	}
}

func TestBrowserScreenshotRunner(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "screenshot.png")

	mock := &mockDriver{
		screenshotFunc: func(ctx context.Context, targetURL, path string) (string, error) {
			if targetURL == "https://fail.com" {
				return "", errors.New("screenshot failed")
			}
			return destPath, nil
		},
	}

	cfg := domain.BrowserConfig{Enabled: true}
	runner := NewScreenshotRunner(cfg, mock)
	if runner.Name() != "browser_screenshot" {
		t.Errorf("Name() = %q, want 'browser_screenshot'", runner.Name())
	}
	if runner.Backend() != "browser" {
		t.Errorf("Backend() = %q, want 'browser'", runner.Backend())
	}
	if runner.Description() == "" {
		t.Errorf("Description() is empty")
	}

	ctx := context.Background()

	// 1. Success
	out, err := runner.Run(ctx, `{"url": "https://example.com", "path": "`+destPath+`"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, destPath) {
		t.Errorf("expected output to contain path %q, got: %s", destPath, out)
	}

	// 2. Missing url
	if _, err = runner.Run(ctx, `{"path": "foo.png"}`); err == nil {
		t.Errorf("expected error for missing url, got nil")
	}

	// 3. Missing path
	if _, err = runner.Run(ctx, `{"url": "https://example.com"}`); err == nil {
		t.Errorf("expected error for missing path, got nil")
	}
	if _, err = runner.Run(ctx, `https://example.com`); err == nil {
		t.Errorf("expected error for raw url with missing path, got nil")
	}

	// 4. Invalid json
	if _, err = runner.Run(ctx, `{"url": `); err == nil {
		t.Errorf("expected error for invalid json, got nil")
	}

	// 5. Driver error
	if _, err = runner.Run(ctx, `{"url": "https://fail.com", "path": "`+destPath+`"}`); err == nil {
		t.Errorf("expected error for driver failure, got nil")
	}

	// 6. Nil driver
	nilRunner := NewScreenshotRunner(cfg, nil)
	if _, err = nilRunner.Run(ctx, `{"url": "https://example.com", "path": "p.png"}`); err == nil {
		t.Errorf("expected error for nil driver, got nil")
	}
}

func TestBrowserCloseRunner(t *testing.T) {
	closed := false
	mock := &mockDriver{
		closeFunc: func() error {
			closed = true
			return nil
		},
	}

	cfg := domain.BrowserConfig{Enabled: true}
	runner := NewCloseRunner(cfg, mock)
	if runner.Name() != "browser_close" {
		t.Errorf("Name() = %q, want 'browser_close'", runner.Name())
	}
	if runner.Backend() != "browser" {
		t.Errorf("Backend() = %q, want 'browser'", runner.Backend())
	}
	if runner.Description() == "" {
		t.Errorf("Description() is empty")
	}

	out, err := runner.Run(context.Background(), "")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !closed {
		t.Errorf("expected driver.Close() to be called")
	}
	if !strings.Contains(out, "closed") {
		t.Errorf("unexpected output: %s", out)
	}

	// Test close error
	failMock := &mockDriver{
		closeFunc: func() error {
			return errors.New("close failure")
		},
	}
	failRunner := NewCloseRunner(cfg, failMock)
	if _, err := failRunner.Run(context.Background(), ""); err == nil {
		t.Errorf("expected error on close failure, got nil")
	}

	// Test nil driver
	nilRunner := NewCloseRunner(cfg, nil)
	out, err = nilRunner.Run(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error with nil driver: %v", err)
	}
	if !strings.Contains(out, "closed") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestCLIDriver_Actions(t *testing.T) {
	driver := &CLIDriver{
		cfg: domain.BrowserConfig{},
	}
	ctx := context.Background()

	// Click success & error
	res, err := driver.Action(ctx, ActionRequest{Action: "click", Selector: "#btn"})
	if err != nil || !res.Success || !strings.Contains(res.Output, "#btn") {
		t.Fatalf("click action failed: %v, %v", err, res)
	}
	if _, err := driver.Action(ctx, ActionRequest{Action: "click"}); err == nil {
		t.Errorf("expected error for click without selector")
	}

	// Type success & error
	res, err = driver.Action(ctx, ActionRequest{Action: "type", Selector: "#input", Text: "hello"})
	if err != nil || !res.Success || !strings.Contains(res.Output, "#input") {
		t.Fatalf("type action failed: %v, %v", err, res)
	}
	if _, err := driver.Action(ctx, ActionRequest{Action: "type"}); err == nil {
		t.Errorf("expected error for type without selector")
	}

	// Scroll success
	res, err = driver.Action(ctx, ActionRequest{Action: "scroll"})
	if err != nil || !res.Success || !strings.Contains(res.Output, "scrolled") {
		t.Fatalf("scroll action failed: %v, %v", err, res)
	}

	// Evaluate success & error
	res, err = driver.Action(ctx, ActionRequest{Action: "evaluate", Script: "document.title"})
	if err != nil || !res.Success || !strings.Contains(res.Output, "document.title") {
		t.Fatalf("evaluate action failed: %v, %v", err, res)
	}
	if _, err := driver.Action(ctx, ActionRequest{Action: "evaluate"}); err == nil {
		t.Errorf("expected error for evaluate without script")
	}

	// Unsupported action
	if _, err := driver.Action(ctx, ActionRequest{Action: "unknown"}); err == nil {
		t.Errorf("expected error for unknown action")
	}

	// Close
	if err := driver.Close(); err != nil {
		t.Errorf("CLIDriver.Close() error = %v", err)
	}
}

func TestCLIDriver_MockedCLI_NavigateAndScreenshot(t *testing.T) {
	tempDir := t.TempDir()
	mockScript := filepath.Join(tempDir, "mock_browser.sh")

	// Create a mock executable shell script
	scriptContent := `#!/bin/sh
for arg in "$@"; do
    if [ "$arg" = "https://fail.com" ]; then
        echo "process error" >&2
        exit 1
    fi
done

for arg in "$@"; do
    case "$arg" in
        --dump-dom)
            echo '<html><head><title>Mock CLI Page</title></head><body><h1>Heading</h1><p>Paragraph text.</p></body></html>'
            exit 0
            ;;
        --screenshot=*)
            dest="${arg#--screenshot=}"
            echo "mock image data" > "$dest"
            exit 0
            ;;
    esac
done
`
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := domain.BrowserConfig{
		ExecutablePath: mockScript,
		Timeout:        5 * time.Second,
		ViewportWidth:  1024,
		ViewportHeight: 768,
		UserDataDir:    filepath.Join(tempDir, "user-data"),
	}

	driver, err := NewCLIDriver(cfg)
	if err != nil {
		t.Fatalf("NewCLIDriver error: %v", err)
	}

	ctx := context.Background()

	// 1. Test Navigate with mock CLI
	res, err := driver.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate error: %v", err)
	}
	if res.Title != "Mock CLI Page" {
		t.Errorf("Title = %q, want 'Mock CLI Page'", res.Title)
	}
	if !strings.Contains(res.Content, "Heading") {
		t.Errorf("Content missing Heading: %q", res.Content)
	}

	// 2. Test Navigate failure
	if _, err := driver.Navigate(ctx, "https://fail.com"); err == nil {
		t.Errorf("expected error for fail url, got nil")
	}

	// 3. Test Screenshot with mock CLI
	screenshotPath := filepath.Join(tempDir, "screen.png")
	savedPath, err := driver.Screenshot(ctx, "https://example.com", screenshotPath)
	if err != nil {
		t.Fatalf("Screenshot error: %v", err)
	}
	if savedPath != screenshotPath {
		t.Errorf("savedPath = %q, want %q", savedPath, screenshotPath)
	}
	if _, err := os.Stat(screenshotPath); err != nil {
		t.Errorf("screenshot file not created: %v", err)
	}

	// 4. Test Screenshot missing path
	if _, err := driver.Screenshot(ctx, "https://example.com", ""); err == nil {
		t.Errorf("expected error for empty destination screenshot path")
	}

	// 5. Test Screenshot failure
	if _, err := driver.Screenshot(ctx, "https://fail.com", filepath.Join(tempDir, "fail.png")); err == nil {
		t.Errorf("expected error for failed screenshot process")
	}
}

func TestBrowserLocator(t *testing.T) {
	tempDir := t.TempDir()
	dummyBin := filepath.Join(tempDir, "my-browser")
	if err := os.WriteFile(dummyBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd, args, err := FindBrowserCommand(dummyBin)
	if err != nil {
		t.Fatalf("FindBrowserCommand(%q) error = %v", dummyBin, err)
	}
	if cmd != dummyBin {
		t.Errorf("cmd = %q, want %q", cmd, dummyBin)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}

	// Test nonexistent custom path
	_, _, err = FindBrowserCommand(filepath.Join(tempDir, "nonexistent"))
	if err == nil {
		t.Errorf("expected error for nonexistent custom browser path")
	}
}

func TestNewBrowserRunners_MockAndUnavail(t *testing.T) {
	mock := &mockDriver{}
	cfg := domain.BrowserConfig{Enabled: true}

	// With mock driver
	runners := NewBrowserRunners(cfg, mock)
	if len(runners) != 4 {
		t.Fatalf("expected 4 runners, got %d", len(runners))
	}
	expectedNames := []string{"browser_navigate", "browser_action", "browser_screenshot", "browser_close"}
	for i, r := range runners {
		if r.Name() != expectedNames[i] {
			t.Errorf("runner[%d].Name() = %q, want %q", i, r.Name(), expectedNames[i])
		}
		if r.Backend() != "browser" {
			t.Errorf("runner[%d].Backend() = %q, want 'browser'", i, r.Backend())
		}
	}

	// With unavail driver (nil driver + impossible custom path)
	unavailCfg := domain.BrowserConfig{
		ExecutablePath: "/path/to/never/existing/browser/binary",
	}
	unavailRunners := NewBrowserRunners(unavailCfg, nil)
	if len(unavailRunners) != 4 {
		t.Fatalf("expected 4 unavail runners, got %d", len(unavailRunners))
	}
	for _, r := range unavailRunners {
		if r.Backend() != "browser" {
			t.Errorf("unavail runner Backend() = %q, want 'browser'", r.Backend())
		}
		if r.Description() == "" {
			t.Errorf("unavail runner Description() is empty")
		}
		_, err := r.Run(context.Background(), "test")
		if err == nil || !strings.Contains(err.Error(), "browser tool unavailable") {
			t.Errorf("expected 'browser tool unavailable' error, got %v", err)
		}
	}
}

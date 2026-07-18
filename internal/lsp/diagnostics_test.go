package lsp

import (
	"testing"
)

func TestParseDiagnostics_NilResult(t *testing.T) {
	diags := parseDiagnostics(nil)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for nil result, got %d", len(diags))
	}
}

func TestParseDiagnostics_Empty(t *testing.T) {
	result := map[string]interface{}{
		"items": []interface{}{},
	}
	diags := parseDiagnostics(result)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for empty items, got %d", len(diags))
	}
}

func TestParseDiagnostics_Valid(t *testing.T) {
	result := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"uri": "file:///home/user/main.go",
				"diagnostics": []interface{}{
					map[string]interface{}{
						"severity": float64(1),
						"message":  "undefined variable",
						"code":     "compiler",
						"range": map[string]interface{}{
							"start": map[string]interface{}{
								"line":      float64(10),
								"character": float64(5),
							},
						},
					},
				},
			},
		},
	}

	diags := parseDiagnostics(result)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}

	d := diags[0]
	if d.File != "/home/user/main.go" {
		t.Errorf("expected file '/home/user/main.go', got %q", d.File)
	}
	if d.Line != 11 { // LSP lines are 0-based, we add 1
		t.Errorf("expected line 11, got %d", d.Line)
	}
	if d.Severity != "error" {
		t.Errorf("expected severity 'error', got %q", d.Severity)
	}
	if d.Message != "undefined variable" {
		t.Errorf("expected message 'undefined variable', got %q", d.Message)
	}
	if d.Code != "compiler" {
		t.Errorf("expected code 'compiler', got %q", d.Code)
	}
}

func TestParseDiagnostics_Multiple(t *testing.T) {
	result := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"uri": "file:///main.go",
				"diagnostics": []interface{}{
					map[string]interface{}{
						"severity": float64(1),
						"message":  "error 1",
						"range": map[string]interface{}{
							"start": map[string]interface{}{
								"line":      float64(0),
								"character": float64(0),
							},
						},
					},
					map[string]interface{}{
						"severity": float64(2),
						"message":  "warning 1",
						"range": map[string]interface{}{
							"start": map[string]interface{}{
								"line":      float64(5),
								"character": float64(10),
							},
						},
					},
				},
			},
		},
	}

	diags := parseDiagnostics(result)
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}

	if diags[0].Severity != "error" {
		t.Errorf("expected first diagnostic severity 'error', got %q", diags[0].Severity)
	}
	if diags[1].Severity != "warning" {
		t.Errorf("expected second diagnostic severity 'warning', got %q", diags[1].Severity)
	}
}

func TestSeverityName(t *testing.T) {
	tests := []struct {
		severity int
		want     string
	}{
		{1, "error"},
		{2, "warning"},
		{3, "info"},
		{4, "hint"},
		{0, "unknown"},
		{99, "unknown"},
	}

	for _, tt := range tests {
		got := severityName(tt.severity)
		if got != tt.want {
			t.Errorf("severityName(%d) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}

func TestFormatDiagnostics_Empty(t *testing.T) {
	output := FormatDiagnostics(nil)
	if output != "No diagnostics found." {
		t.Errorf("expected 'No diagnostics found.', got %q", output)
	}
}

func TestFormatDiagnostics_WithItems(t *testing.T) {
	diags := []Diagnostic{
		{File: "main.go", Line: 10, Column: 5, Severity: "error", Message: "undefined: x", Code: "compiler"},
	}
	output := FormatDiagnostics(diags)
	if output == "" {
		t.Error("expected non-empty output")
	}
	if !contains(output, "main.go") {
		t.Error("output should contain file name")
	}
	if !contains(output, "undefined: x") {
		t.Error("output should contain error message")
	}
}

func TestFormatCode(t *testing.T) {
	if got := formatCode(nil); got != "" {
		t.Errorf("formatCode(nil) = %q, want empty", got)
	}
	if got := formatCode("compiler"); got != "compiler" {
		t.Errorf("formatCode('compiler') = %q, want 'compiler'", got)
	}
	if got := formatCode(float64(42)); got != "42" {
		t.Errorf("formatCode(42.0) = %q, want '42'", got)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

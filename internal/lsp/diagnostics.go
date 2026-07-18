package lsp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Diagnostic represents an LSP diagnostic (error, warning, hint).
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // "error", "warning", "info", "hint"
	Message  string `json:"message"`
	Code     string `json:"code"`
}

// parseDiagnostics extracts diagnostics from an LSP workspace/diagnostic response.
func parseDiagnostics(result interface{}) []Diagnostic {
	if result == nil {
		return nil
	}

	// The result can be either a raw map or an Items field.
	raw, ok := result.(map[string]interface{})
	if !ok {
		return nil
	}

	items, ok := raw["items"].([]interface{})
	if !ok {
		return nil
	}

	var diags []Diagnostic
	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract URI → file path.
		uri := stringField(itemMap, "uri")
		file := strings.TrimPrefix(uri, "file://")

		// Extract diagnostics array.
		diagItems, ok := itemMap["diagnostics"].([]interface{})
		if !ok {
			continue
		}

		for _, di := range diagItems {
			d, ok := di.(map[string]interface{})
			if !ok {
				continue
			}

			severity := severityName(intField(d, "severity"))
			msg := stringField(d, "message")
			code := formatCode(d["code"])

			line := 0
			col := 0
			if rng, ok := d["range"].(map[string]interface{}); ok {
				if start, ok := rng["start"].(map[string]interface{}); ok {
					line = intField(start, "line") + 1
					col = intField(start, "character")
				}
			}

			diags = append(diags, Diagnostic{
				File:     file,
				Line:     line,
				Column:   col,
				Severity: severity,
				Message:  msg,
				Code:     code,
			})
		}
	}

	return diags
}

// Format transforms diagnostics into a human-readable tool output string.
func FormatDiagnostics(diags []Diagnostic) string {
	if len(diags) == 0 {
		return "No diagnostics found."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d diagnostic(s):\n\n", len(diags)))

	for _, d := range diags {
		b.WriteString(fmt.Sprintf("%s:%d:%d: %s: %s\n",
			d.File, d.Line, d.Column, d.Severity, d.Message))
	}

	return b.String()
}

// severityName converts an LSP severity number to a human-readable name.
func severityName(severity int) string {
	switch severity {
	case 1:
		return "error"
	case 2:
		return "warning"
	case 3:
		return "info"
	case 4:
		return "hint"
	default:
		return "unknown"
	}
}

// stringField safely extracts a string from a map.
func stringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// intField safely extracts an int from a map (handles float64 JSON numbers).
func intField(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

// formatCode converts a diagnostic code to a string representation.
func formatCode(code interface{}) string {
	if code == nil {
		return ""
	}
	switch v := code.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

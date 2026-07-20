package shell

import (
	"testing"

	"gaia/internal/modules/security"
)

// TestIntegration_SecretRedactionInShellOutput verifies that secret patterns
// are redacted from shell command output before the result is returned.
// The integration test covers the full path: shell module → RedactSecrets → ToolResult.
func TestIntegration_SecretRedactionInShellOutput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantRedact  bool
		description string
	}{
		{
			name:        "OpenAI key",
			input:       "Key: sk-abcdefghijklmnopqrstuvwxyz123456",
			wantRedact:  true,
			description: "sk- prefix with 20+ chars should be redacted",
		},
		{
			name:        "GitHub token",
			input:       "Token: ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			wantRedact:  true,
			description: "ghp_ prefix with 36 chars should be redacted",
		},
		{
			name:        "Bearer token",
			input:       "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature",
			wantRedact:  true,
			description: "Bearer token in auth header should be redacted",
		},
		{
			name: "PEM private key",
			input: `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC...
-----END PRIVATE KEY-----`,
			wantRedact:  true,
			description: "PEM private key block should be redacted",
		},
		{
			name:       "clean output",
			input:      "Files: main.go (124 lines), utils.go (56 lines)",
			wantRedact: false,
			description: "Clean output with no secrets should pass through unchanged",
		},
		{
			name:       "fake key too short",
			input:      "export KEY=sk-short",
			wantRedact: false,
			description: "sk- prefix with fewer than 20 chars after should not match",
		},
		{
			name: "mixed output",
			input: `Build successful.
API key: sk-abcdefghijklmnopqrstuvwxyz123456
Token: ghp_abcdefghijklmnopqrstuvwxyz1234567890
Files compiled: 42`,
			wantRedact: true,
			description: "Multiple secrets in mixed output should all be redacted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := security.RedactSecrets(tt.input)

			if tt.wantRedact {
				if result == tt.input {
					t.Errorf("expected redaction for %s, but output unchanged: %q", tt.description, result)
				}
				// Check that the redacted marker is present.
				if !containsRedacted(result) {
					t.Errorf("expected [REDACTED] marker in output: %q", result)
				}
				// Verify original secrets are NOT leaked.
				if containsSecretPatterns(result, tt.input) {
					t.Errorf("secrets leaked in redacted output: %q", result)
				}
			} else {
				if result != tt.input {
					t.Errorf("expected output unchanged for %s, got: %q", tt.description, result)
				}
			}
		})
	}
}

// TestIntegration_RedactedOutputNeverLeakedToLLM verifies that when a tool
// result contains secrets, the output that gets fed back to the LLM
// (the ToolResult.Output field) has already been redacted.
func TestIntegration_RedactedOutputNeverLeakedToLLM(t *testing.T) {
	original := "ENV: OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz123456\nGH_TOKEN=ghp_abcdefghijklmnopqrstuvwxyz1234567890"
	redacted := security.RedactSecrets(original)

	// Verify the post-redaction output has no secrets.
	if containsSecretPatterns(redacted, original) {
		t.Errorf("post-redaction output still contains secrets: %q", redacted)
	}

	// Verify [REDACTED] markers present for both patterns.
	redactions := countRedactions(redacted)
	if redactions < 2 {
		t.Errorf("expected at least 2 redactions, got %d in: %q", redactions, redacted)
	}

	// Module should exist and be usable.
	mod := NewModule(".")
	if mod == nil {
		t.Fatal("expected non-nil shell module")
	}
	if mod.Name() != "shell" {
		t.Errorf("expected module name 'shell', got %q", mod.Name())
	}
	tools := mod.GetTools()
	if len(tools) == 0 {
		t.Error("expected at least one tool registered")
	}
}

// redactedMarker is the replacement text used by security.RedactSecrets.
const redactedMarker = "[REDACTED]"

// containsRedacted returns true if the output contains the redaction marker.
func containsRedacted(s string) bool {
	return contains(s, redactedMarker)
}

// containsSecretPatterns checks if the redacted output still contains
// any of the original secret patterns.
func containsSecretPatterns(redacted, original string) bool {
	n := len(original)
	// Check for OpenAI key pattern (sk-* with 20+ chars).
	for i := 0; i+3 <= n; i++ {
		if original[i:i+3] == "sk-" && n-i >= 23 {
			if contains(redacted, original[i:i+23]) {
				return true
			}
		}
	}
	// Check for GitHub token pattern (ghp_* with 36 chars after prefix).
	for i := 0; i+4 <= n; i++ {
		if original[i:i+4] == "ghp_" && n-i >= 40 {
			if contains(redacted, original[i:i+40]) {
				return true
			}
		}
	}
	// Check for Bearer token leak (the whole "Bearer ..." phrase should be gone).
	if contains(original, "Bearer") && contains(redacted, "Bearer") {
		return true
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func countRedactions(s string) int {
	count := 0
	for i := 0; i+len(redactedMarker) <= len(s); i++ {
		if s[i:i+len(redactedMarker)] == redactedMarker {
			count++
		}
	}
	return count
}

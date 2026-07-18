package agent

import (
	"strings"
	"testing"
)

// TestRedactContent_OpenAIKey verifies OpenAI-style API keys are redacted.
func TestRedactContent_OpenAIKey(t *testing.T) {
	input := "export OPENAI_KEY=sk-abcdefghijklmnopqrstuvwxyz123456"
	result, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for OpenAI key")
	}
	if strings.Contains(result, "sk-") {
		t.Error("redacted content should not contain the original key prefix")
	}
	if !strings.Contains(result, "REDACTED") {
		t.Error("redacted content should contain REDACTED marker")
	}
}

// TestRedactContent_GitHubClassicToken verifies classic GitHub tokens are redacted.
func TestRedactContent_GitHubClassicToken(t *testing.T) {
	input := "GITHUB_TOKEN=ghp_1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p7q8r9s0t"
	result, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for GitHub classic PAT")
	}
	if strings.Contains(result, "ghp_") {
		t.Error("redacted content should not contain ghp_ prefix")
	}
	if !strings.Contains(result, "GITHUB_TOKEN") {
		t.Error("redacted content should mention GITHUB_TOKEN in replacement")
	}
}

// TestRedactContent_GitHubFineGrainedToken verifies fine-grained GitHub tokens.
func TestRedactContent_GitHubFineGrainedToken(t *testing.T) {
	input := "token: github_pat_11ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	result, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for GitHub fine-grained PAT")
	}
	if strings.Contains(result, "github_pat_") {
		t.Error("redacted content should not contain github_pat_ prefix")
	}
}

// TestRedactContent_PEMPrivateKey verifies PEM private keys are redacted.
func TestRedactContent_PEMPrivateKey(t *testing.T) {
	input := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3F...
-----END RSA PRIVATE KEY-----`
	result, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for PEM private key")
	}
	if strings.Contains(result, "BEGIN RSA PRIVATE KEY") {
		t.Error("redacted content should not contain the PEM block")
	}
	if !strings.Contains(result, "PRIVATE_KEY") {
		t.Error("redacted content should mention PRIVATE_KEY in replacement")
	}
}

// TestRedactContent_BearerToken verifies Bearer token headers are redacted.
func TestRedactContent_BearerToken(t *testing.T) {
	input := "Authorization: Bearer abcdefghijklmnopqrstuvwxyz1234567890ABCDEF"
	result, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for Bearer token")
	}
	if !strings.Contains(result, "Bearer [REDACTED") {
		t.Error("redacted content should contain Bearer [REDACTED:TOKEN]")
	}
}

// TestRedactContent_LowercaseBearer verifies case-insensitive bearer matching.
func TestRedactContent_LowercaseBearer(t *testing.T) {
	input := "authorization: bearer someSecretTokenValueHere12345"
	_, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for lowercase bearer token")
	}
}

// TestRedactContent_JWT verifies JWT tokens are redacted.
func TestRedactContent_JWT(t *testing.T) {
	input := `{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"}`
	result, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for JWT")
	}
	if !strings.Contains(result, "REDACTED:JWT") {
		t.Error("redacted content should contain REDACTED:JWT")
	}
}

// TestRedactContent_AWSKey verifies AWS access key IDs are redacted.
func TestRedactContent_AWSKey(t *testing.T) {
	input := "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	result, count := RedactContent(input)

	if count == 0 {
		t.Error("expected at least 1 redaction for AWS key")
	}
	if !strings.Contains(result, "REDACTED:AWS_KEY") {
		t.Error("redacted content should contain REDACTED:AWS_KEY")
	}
}

// TestRedactContent_KeyValueSecret verifies key=value secrets are redacted.
func TestRedactContent_KeyValueSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"password", "DB_PASSWORD=supersecret123"},
		{"secret", "API_SECRET=hidden-value-here"},
		{"token", "AUTH_TOKEN=abc.def.ghi"},
		{"api_key", "OPENAI_API_KEY=sk-abc123"},
		{"pwd", "DB_PWD=mypassword"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, count := RedactContent(tt.input)
			if count == 0 {
				t.Errorf("expected at least 1 redaction for %q pattern", tt.name)
			}
			if !strings.Contains(result, "REDACTED:SECRET") {
				t.Errorf("%q: redacted content should contain REDACTED:SECRET", tt.name)
			}
		})
	}
}

// TestRedactContent_NoSensitive verifies benign content is unchanged.
func TestRedactContent_NoSensitive(t *testing.T) {
	input := "This is a normal message about code structure and patterns."
	result, count := RedactContent(input)

	if count != 0 {
		t.Errorf("expected 0 redactions, got %d", count)
	}
	if result != input {
		t.Errorf("benign content should be unchanged. got %q", result)
	}
}

// TestRedactContent_MultiplePatterns verifies multiple patterns in one string.
func TestRedactContent_MultiplePatterns(t *testing.T) {
	input := `Config:
export OPENAI_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz
GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
DB_PASSWORD=secret123`

	result, count := RedactContent(input)

	if count < 3 {
		t.Errorf("expected at least 3 redactions in multi-pattern input, got %d", count)
	}

	if strings.Contains(result, "sk-") {
		t.Error("should not contain OpenAI key prefix")
	}
	if strings.Contains(result, "ghp_") {
		t.Error("should not contain GitHub token prefix")
	}
	if strings.Contains(result, "secret123") {
		t.Error("should not contain database password")
	}
}

// TestMustRedact verifies the MustRedact helper function.
func TestMustRedact(t *testing.T) {
	if MustRedact("normal text without secrets") {
		t.Error("MustRedact should return false for clean text")
	}
	if !MustRedact("API_KEY=sk-abc123def456") {
		t.Error("MustRedact should return true for text with secrets")
	}
}

package core

import (
	"strings"
	"testing"
)

func TestRedactToolOutput(t *testing.T) {
	b := &Brain{}

	tests := []struct {
		name        string
		input       string
		expectedSub string
		minCount    int
	}{
		{
			name:        "Empty string",
			input:       "",
			expectedSub: "",
			minCount:    0,
		},
		{
			name:        "OpenAI API key",
			input:       "API key is sk-abcdefghijklmnopqrstuvwxyz123456",
			expectedSub: "[REDACTED:API_KEY]",
			minCount:    1,
		},
		{
			name:        "GitHub classic PAT",
			input:       "GitHub PAT: ghp_1234567890abcdefghijklmnopqrstuvwxyz12",
			expectedSub: "[REDACTED:GITHUB_TOKEN]",
			minCount:    1,
		},
		{
			name:        "GitHub fine-grained PAT",
			input:       "PAT: github_pat_11AAAAAAA_bbbbbbbbbbbbbbbbbbbb",
			expectedSub: "[REDACTED:GITHUB_TOKEN]",
			minCount:    1,
		},
		{
			name:        "PEM Private Key",
			input:       "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
			expectedSub: "[REDACTED:PRIVATE_KEY]",
			minCount:    1,
		},
		{
			name:        "Bearer token",
			input:       "Authorization: Bearer my-secret-token-1234567890",
			expectedSub: "Bearer [REDACTED:TOKEN]",
			minCount:    1,
		},
		{
			name:        "JWT Token",
			input:       "Session eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozG4B_wH_m",
			expectedSub: "[REDACTED:JWT]",
			minCount:    1,
		},
		{
			name:        "AWS Key ID",
			input:       "AWS ID: AKIAIOSFODNN7EXAMPLE",
			expectedSub: "[REDACTED:AWS_KEY]",
			minCount:    1,
		},
		{
			name:        "Key-value password",
			input:       "DB_PASSWORD=superSecret123!",
			expectedSub: "DB_PASSWORD=[REDACTED:SECRET]",
			minCount:    1,
		},
		{
			name:        "High-entropy base64 secret",
			input:       "Blob: aGVsbG8gd29ybGQgdGhpcyBpcyBhIHZlcnkgc2VjcmV0IHRva2VuIHdpdGggbWl4ZWQgY2hhcnMgKy8=",
			expectedSub: "[REDACTED:BASE64]",
			minCount:    1,
		},
		{
			name:        "Clean benign text",
			input:       "This is regular log output with no credentials.",
			expectedSub: "This is regular log output with no credentials.",
			minCount:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, count := b.RedactToolOutput(tc.input)
			if tc.minCount == 0 && count != 0 {
				t.Errorf("expected count 0, got %d for benign input %q", count, tc.input)
			}
			if tc.minCount > 0 && count < tc.minCount {
				t.Errorf("expected count >= %d, got %d for input %q", tc.minCount, count, tc.input)
			}
			if !strings.Contains(got, tc.expectedSub) {
				t.Errorf("expected result to contain %q, got %q", tc.expectedSub, got)
			}
		})
	}
}

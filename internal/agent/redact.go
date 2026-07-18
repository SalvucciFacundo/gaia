package agent

import (
	"regexp"
	"strings"
)

// Redaction patterns for sensitive content in tool outputs.
// Applied before tool results are fed back to the LLM or displayed.
var redactionPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
	description string
}{
	// OpenAI-style API keys: sk-... (20+ chars, may include hyphens in new formats)
	{
		pattern:     regexp.MustCompile(`sk-[a-zA-Z0-9\-]{20,}`),
		replacement: "[REDACTED:API_KEY]",
		description: "OpenAI API key",
	},
	// GitHub classic personal access tokens: ghp_... (36+ chars of base64)
	{
		pattern:     regexp.MustCompile(`ghp_[a-zA-Z0-9]{36,}`),
		replacement: "[REDACTED:GITHUB_TOKEN]",
		description: "GitHub classic PAT",
	},
	// GitHub fine-grained personal access tokens: github_pat_...
	{
		pattern:     regexp.MustCompile(`github_pat_[a-zA-Z0-9_]{20,}`),
		replacement: "[REDACTED:GITHUB_TOKEN]",
		description: "GitHub fine-grained PAT",
	},
	// PEM private keys: -----BEGIN ... PRIVATE KEY----- ... -----END ... PRIVATE KEY-----
	{
		pattern:     regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----[^-]*-----END [A-Z ]+ PRIVATE KEY-----`),
		replacement: "[REDACTED:PRIVATE_KEY]",
		description: "PEM private key",
	},
	// Bearer tokens in headers: "Bearer eyJ..." or "bearer xyz..."
	{
		pattern:     regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`),
		replacement: "Bearer [REDACTED:TOKEN]",
		description: "HTTP Bearer token",
	},
	// JWT tokens: three base64url segments separated by dots
	{
		pattern:     regexp.MustCompile(`eyJ[a-zA-Z0-9\-_]+\.eyJ[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+`),
		replacement: "[REDACTED:JWT]",
		description: "JSON Web Token",
	},
	// AWS access key IDs: AKIA...
	{
		pattern:     regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		replacement: "[REDACTED:AWS_KEY]",
		description: "AWS access key",
	},
	// Generic secrets in key=value context: password=..., secret=..., etc.
	// Matches regardless of prefix (DB_PASSWORD, MY_SECRET, api_key, etc.)
	// The captured group preserves the key name; value is replaced with placeholder.
	{
		pattern:     regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api[_-]?key)\s*[:=]\s*\S+`),
		replacement: "${1}=[REDACTED:SECRET]",
		description: "Key-value secret",
	},
}

// RedactContent scans content for sensitive patterns and replaces them
// with descriptive placeholders. Returns the redacted content and the
// count of redactions performed.
func RedactContent(content string) (string, int) {
	result := content
	count := 0

	for _, rp := range redactionPatterns {
		matches := rp.pattern.FindAllString(result, -1)
		if len(matches) > 0 {
			result = rp.pattern.ReplaceAllString(result, rp.replacement)
			count += len(matches)
		}
	}

	// Heuristic: very long base64-looking strings (>50 chars without spaces,
	// containing both letters and numbers, with typical base64 characters).
	// This catches encoded credentials that don't match specific patterns.
	base64Pattern := regexp.MustCompile(`[A-Za-z0-9+/=]{50,}`)
	matches := base64Pattern.FindAllString(result, -1)
	for _, match := range matches {
		hasUpper := strings.ContainsAny(match, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		hasLower := strings.ContainsAny(match, "abcdefghijklmnopqrstuvwxyz")
		hasDigit := strings.ContainsAny(match, "0123456789")
		hasSpecial := strings.ContainsAny(match, "+/=")
		// Only redact if it looks like mixed base64 (has diverse char classes)
		if hasUpper && hasLower && hasDigit && hasSpecial {
			result = strings.Replace(result, match, "[REDACTED:BASE64]", 1)
			count++
		}
	}

	return result, count
}

// MustRedact returns true if the content contains any sensitive patterns.
func MustRedact(content string) bool {
	_, count := RedactContent(content)
	return count > 0
}

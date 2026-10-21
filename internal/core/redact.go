package core

import (
	"regexp"
	"strings"
)

// redactPatterns matches sensitive credentials and tokens in tool outputs.
var redactPatterns = []struct {
	pat     *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`sk-[a-zA-Z0-9\-]{20,}`), "[REDACTED:API_KEY]"},
	{regexp.MustCompile(`ghp_[a-zA-Z0-9]{36,}`), "[REDACTED:GITHUB_TOKEN]"},
	{regexp.MustCompile(`github_pat_[a-zA-Z0-9_]{20,}`), "[REDACTED:GITHUB_TOKEN]"},
	{regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----[^-]*-----END [A-Z ]+ PRIVATE KEY-----`), "[REDACTED:PRIVATE_KEY]"},
	{regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`), "Bearer [REDACTED:TOKEN]"},
	{regexp.MustCompile(`eyJ[a-zA-Z0-9\-_]+\.eyJ[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+`), "[REDACTED:JWT]"},
	{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "[REDACTED:AWS_KEY]"},
	{regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api[_-]?key)\s*[:=]\s*\S+`), "${1}=[REDACTED:SECRET]"},
}

// base64Pattern identifies potential encoded high-entropy secrets.
var base64Pattern = regexp.MustCompile(`[A-Za-z0-9+/=]{50,}`)

func redactToolOutput(output string) (string, int) {
	if output == "" {
		return "", 0
	}

	result := output
	count := 0
	for _, rp := range redactPatterns {
		matches := rp.pat.FindAllStringIndex(result, -1)
		if len(matches) > 0 {
			result = rp.pat.ReplaceAllString(result, rp.replace)
			count += len(matches)
		}
	}

	// Heuristic: catches encoded credentials with mixed character classes
	matches := base64Pattern.FindAllString(result, -1)
	for _, match := range matches {
		hasUpper := strings.ContainsAny(match, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		hasLower := strings.ContainsAny(match, "abcdefghijklmnopqrstuvwxyz")
		hasDigit := strings.ContainsAny(match, "0123456789")
		hasSpecial := strings.ContainsAny(match, "+/=")
		if hasUpper && hasLower && hasDigit && hasSpecial {
			result = strings.Replace(result, match, "[REDACTED:BASE64]", 1)
			count++
		}
	}

	return result, count
}

// RedactToolOutput applies message redaction to tool outputs before
// they are fed back to the LLM.
func (b *Brain) RedactToolOutput(output string) (string, int) {
	return redactToolOutput(output)
}

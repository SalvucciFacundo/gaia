// Package security provides shared validation and redaction primitives
// used by all tool modules to enforce safety boundaries.
package security

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Pre-compiled secret patterns.
var secretPatterns = []*regexp.Regexp{
	// OpenAI / Anthropic API keys
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
	// GitHub personal access tokens
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	// Bearer tokens
	regexp.MustCompile(`Bearer\s+[a-zA-Z0-9\-_\.\+]+`),
	// PEM private keys (multiline)
	regexp.MustCompile(`(?s)-----BEGIN[^-]*PRIVATE[^-]*KEY-----[^-]*-----END[^-]*PRIVATE[^-]*KEY-----`),
}

// ValidatePath checks that targetPath, resolved relative to projectRoot,
// does not escape outside the project root. Returns the resolved absolute path
// or an error. Symlinks are resolved on the final path.
func ValidatePath(projectRoot, targetPath string) (string, error) {
	// Normalize the project root
	rootAbs, err := filepath.Abs(filepath.Clean(projectRoot))
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		// Root may not exist entirely — use cleaned path
		rootReal = rootAbs
	}

	// Build candidate absolute path
	candidate := targetPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootAbs, candidate)
	}
	candidate = filepath.Clean(candidate)

	// Resolve symlinks if possible; fall back to cleaned path
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// Path may not exist yet (e.g., file_write) — walk up
		// to the first existing ancestor and resolve that portion.
		resolved = resolveExistingPrefix(candidate)
	}

	// Case-insensitive prefix check (Windows-compatible)
	lowerRoot := strings.ToLower(rootReal)
	lowerResolved := strings.ToLower(resolved)
	sep := string(filepath.Separator)

	if lowerResolved != lowerRoot && !strings.HasPrefix(lowerResolved, lowerRoot+sep) {
		return "", fmt.Errorf("path traversal blocked: %s is outside project root", targetPath)
	}

	return resolved, nil
}

// resolveExistingPrefix walks candidate toward root, resolving symlinks
// on the first existing ancestor and appending the remaining suffix.
func resolveExistingPrefix(candidate string) string {
	parts := []string{}
	current := candidate
	for {
		_, err := os.Lstat(current)
		if err == nil {
			real, err := filepath.EvalSymlinks(current)
			if err == nil {
				return filepath.Join(append([]string{real}, parts...)...)
			}
			break
		}
		parts = append([]string{filepath.Base(current)}, parts...)
		parent := filepath.Dir(current)
		if parent == current {
			// Root reached — use cleaned path as-is
			return filepath.Clean(candidate)
		}
		current = parent
	}
	return filepath.Clean(candidate)
}

// ValidateURL checks that rawURL uses http or https scheme and does not
// resolve to a private or loopback address.
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("empty URL")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host in URL")
	}

	if isPrivateHost(host) {
		return fmt.Errorf("private/internal host blocked: %s", host)
	}

	return nil
}

// isPrivateHost returns true if the hostname is localhost or resolves
// to a private/loopback/link-local address.
func isPrivateHost(host string) bool {
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" || lower == "0.0.0.0" {
		return true
	}

	// Fast path: literal IP address
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}

	// DNS lookup
	addrs, err := net.LookupIP(host)
	if err != nil {
		return false // allow if DNS fails
	}
	for _, addr := range addrs {
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// RedactSecrets scans output for known secret patterns and replaces them
// with [REDACTED].
func RedactSecrets(output string) string {
	for _, p := range secretPatterns {
		output = p.ReplaceAllString(output, "[REDACTED]")
	}
	return output
}

package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath_InsideProjectRoot(t *testing.T) {
	root := t.TempDir()
	// Create a file inside root
	subPath := filepath.Join(root, "src", "main.go")
	os.MkdirAll(filepath.Dir(subPath), 0755)
	os.WriteFile(subPath, []byte("package main"), 0644)

	resolved, err := ValidatePath(root, "src/main.go")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.HasSuffix(resolved, "main.go") {
		t.Errorf("unexpected resolved path: %s", resolved)
	}
}

func TestValidatePath_TraversalBlocked(t *testing.T) {
	root := t.TempDir()

	_, err := ValidatePath(root, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for traversal path")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Errorf("expected traversal error, got: %v", err)
	}
}

func TestValidatePath_AbsoluteOutsideBlocked(t *testing.T) {
	root := t.TempDir()

	// On Windows, /etc is not absolute; use a path that escapes via traversal.
	_, err := ValidatePath(root, "..\\..\\..\\Windows\\System32")
	if err == nil {
		t.Fatal("expected absolute outside path to be blocked")
	}
}

func TestValidatePath_DotDotPrefixBlocked(t *testing.T) {
	root := t.TempDir()

	_, err := ValidatePath(root, "../secret.txt")
	if err == nil {
		t.Fatal("expected error for dot-dot path")
	}
}

func TestValidatePath_SymlinkTraversalBlocked(t *testing.T) {
	root := t.TempDir()

	// Create a symlink pointing outside (skip if not supported)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	os.WriteFile(outside, []byte("secret"), 0644)

	symlinkPath := filepath.Join(root, "link")
	err := os.Symlink(outside, symlinkPath)
	if err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	_, err = ValidatePath(root, "link")
	if err == nil {
		t.Fatal("expected symlink traversal to be blocked")
	}
}

func TestValidatePath_EmptyRoot(t *testing.T) {
	// Even for non-existent directories, traversal should be caught
	_, err := ValidatePath("/nonexistent/root/12345", "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for traversal from non-existent root")
	}
}

func TestValidateURL_HTTPS_OK(t *testing.T) {
	err := ValidateURL("https://example.com/api")
	if err != nil {
		t.Errorf("expected no error for https URL, got: %v", err)
	}
}

func TestValidateURL_HTTP_OK(t *testing.T) {
	err := ValidateURL("http://example.com/api")
	if err != nil {
		t.Errorf("expected no error for http URL, got: %v", err)
	}
}

func TestValidateURL_LocalhostBlocked(t *testing.T) {
	err := ValidateURL("http://localhost:8080/api")
	if err == nil {
		t.Fatal("expected localhost to be blocked")
	}
}

func TestValidateURL_LoopbackBlocked(t *testing.T) {
	err := ValidateURL("http://127.0.0.1:3000/api")
	if err == nil {
		t.Fatal("expected loopback IP to be blocked")
	}
}

func TestValidateURL_PrivateBlocked(t *testing.T) {
	err := ValidateURL("http://10.0.0.1/api")
	if err == nil {
		t.Fatal("expected private IP to be blocked")
	}
}

func TestValidateURL_InvalidScheme(t *testing.T) {
	err := ValidateURL("file:///etc/passwd")
	if err == nil {
		t.Fatal("expected file:// scheme to be rejected")
	}
}

func TestValidateURL_Empty(t *testing.T) {
	err := ValidateURL("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestRedactSecrets_OpenAIKey(t *testing.T) {
	input := "export OPENAI_KEY=sk-abcdefghijklmnopqrstuvwxyz123456"
	result := RedactSecrets(input)
	if strings.Contains(result, "sk-abcdefg") {
		t.Errorf("OpenAI key not redacted: %s", result)
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker in: %s", result)
	}
}

func TestRedactSecrets_GitHubToken(t *testing.T) {
	input := "ghp_1234567890abcdefghijklmnopqrstuvwxyzAB"
	result := RedactSecrets(input)
	if strings.Contains(result, "ghp_") {
		t.Errorf("GitHub token not redacted: %s", result)
	}
}

func TestRedactSecrets_BearerToken(t *testing.T) {
	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"
	result := RedactSecrets(input)
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("Bearer token not redacted: %s", result)
	}
}

func TestRedactSecrets_PEMKey(t *testing.T) {
	input := `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC
-----END PRIVATE KEY-----`
	result := RedactSecrets(input)
	if strings.Contains(result, "BEGIN PRIVATE KEY") {
		t.Errorf("PEM private key not redacted: %s", result)
	}
}

func TestRedactSecrets_Multiple(t *testing.T) {
	// sk- pattern requires >=20 chars after prefix; ghp_ requires exactly 36 chars.
	input := "key1: sk-abcdefghijklmnopqrstuvwxyz\nkey2: ghp_abcdefghijklmnopqrstuvwxyz1234567890"
	result := RedactSecrets(input)
	count := strings.Count(result, "[REDACTED]")
	if count != 2 {
		t.Errorf("expected 2 redactions, got %d: %s", count, result)
	}
}

func TestRedactSecrets_NoMatch(t *testing.T) {
	input := "regular output without secrets"
	result := RedactSecrets(input)
	if result != input {
		t.Errorf("expected no change, got: %s", result)
	}
}

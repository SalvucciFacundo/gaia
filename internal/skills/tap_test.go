package skills

import (
	"testing"
)

func TestTapNameFromURL(t *testing.T) {
	tests := []struct {
		url, want string
	}{
		{"github.com/user/repo", "user-repo"},
		{"github.com/org-name/project", "org-name-project"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := tapNameFromURL(tt.url)
			if got != tt.want {
				t.Errorf("tapNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestAddTap_InvalidURL(t *testing.T) {
	h := NewHub("", t.TempDir())

	// Non-GitHub URL.
	if err := h.AddTap("gitlab.com/user/repo", "main"); err == nil {
		t.Error("expected error for non-github URL")
	}
}

func TestAddTap_PathTraversal(t *testing.T) {
	h := NewHub("", t.TempDir())

	// Path traversal attempt.
	err := h.AddTap("github.com/user/../../etc/passwd", "main")
	if err == nil {
		t.Error("expected error for path traversal URL")
	}
}

func TestListTaps_Empty(t *testing.T) {
	h := NewHub("", t.TempDir())
	taps, err := h.ListTaps()
	if err != nil {
		t.Fatalf("ListTaps error: %v", err)
	}
	if len(taps) != 0 {
		t.Errorf("expected 0 taps, got %d", len(taps))
	}
}

func TestRemoveTap_NotInstalled(t *testing.T) {
	h := NewHub("", t.TempDir())
	err := h.RemoveTap("github.com/user/nonexistent")
	if err == nil {
		t.Error("expected error for uninstalled tap")
	}
}

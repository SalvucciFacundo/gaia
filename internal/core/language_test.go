package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string)
		expected string
	}{
		{
			name: "Go with go.mod",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0600)
			},
			expected: "go",
		},
		{
			name: "Go with Makefile and .go files",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:"), 0600)
				_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0600)
			},
			expected: "go",
		},
		{
			name: "TypeScript with tsconfig.json",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0600)
			},
			expected: "typescript",
		},
		{
			name: "JavaScript with package.json only",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0600)
			},
			expected: "javascript",
		},
		{
			name: "Rust with Cargo.toml",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(""), 0600)
			},
			expected: "rust",
		},
		{
			name: "CSharp with .csproj",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "App.csproj"), []byte("<Project/>"), 0600)
			},
			expected: "csharp",
		},
		{
			name: "Python with pyproject.toml",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(""), 0600)
			},
			expected: "python",
		},
		{
			name: "Java with pom.xml",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0600)
			},
			expected: "java",
		},
		{
			name: "PHP with composer.json",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte("{}"), 0600)
			},
			expected: "php",
		},
		{
			name:     "Empty directory",
			setup:    func(dir string) {},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tc.setup(tmpDir)

			got := DetectLanguage(tmpDir)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestDetectProjectName(t *testing.T) {
	root := "/path/to/my-agent-project"
	name := DetectProjectName(root)
	if name != "my-agent-project" {
		t.Errorf("expected 'my-agent-project', got %q", name)
	}
}

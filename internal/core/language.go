package core

import (
	"os"
	"path/filepath"
	"strings"
)

var languageDetectors = []struct {
	Name     string
	Detect   func(root string) bool
	Projects []string
}{
	{
		Name: "go",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "go.mod")) ||
				fileExists(filepath.Join(root, "Makefile")) && dirContains(root, ".go")
		},
	},
	{
		Name: "java",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "pom.xml")) ||
				fileExists(filepath.Join(root, "build.gradle")) ||
				fileExists(filepath.Join(root, "build.gradle.kts"))
		},
	},
	{
		Name: "php",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "composer.json"))
		},
	},
	{
		Name: "python",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "requirements.txt")) ||
				fileExists(filepath.Join(root, "pyproject.toml")) ||
				fileExists(filepath.Join(root, "setup.py"))
		},
	},
	{
		Name: "javascript",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "package.json")) &&
				!fileExists(filepath.Join(root, "tsconfig.json"))
		},
	},
	{
		Name: "typescript",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "tsconfig.json"))
		},
	},
	{
		Name: "rust",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "Cargo.toml"))
		},
	},
	{
		Name: "csharp",
		Detect: func(root string) bool {
			return fileExists(filepath.Join(root, "*.csproj"))
		},
	},
}

// DetectLanguage detects the programming language of a project from its root.
func DetectLanguage(projectRoot string) string {
	for _, d := range languageDetectors {
		if d.Detect(projectRoot) {
			return d.Name
		}
	}
	return ""
}

// DetectProjectName returns the project directory name.
func DetectProjectName(projectRoot string) string {
	return filepath.Base(projectRoot)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirContains(root, ext string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			return true
		}
	}
	return false
}

// Ensure filepath is used
var _ = filepath.Join

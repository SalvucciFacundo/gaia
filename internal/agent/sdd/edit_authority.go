package sdd

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EditConsentChoice defines a structured option in a consent envelope.
type EditConsentChoice struct {
	Token  string `json:"token"`  // "granted" or "declined"
	Label  string `json:"label"`  // Human-readable choice label
	Effect string `json:"effect"` // Consequence of choosing this option
}

// EditAuthorityConsent models the typed consent envelope emitted when edits
// target surfaces outside authorized roots (blocked(edit_authority_missing)).
type EditAuthorityConsent struct {
	Schema       string              `json:"schema"` // "gaia.sdd.consent/v1"
	Headline     string              `json:"headline"`
	Reason       string              `json:"reason"`
	MissingRoots []string            `json:"missing_roots"`
	AllowedRoots []string            `json:"allowed_roots"`
	Choices      []EditConsentChoice `json:"choices"`
}

// ValidateEditRoots ensures that allowedEditRoots are narrow, relative, non-empty,
// and do not use wildcards like "." or bare root "/".
func ValidateEditRoots(roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("allowed edit roots cannot be empty")
	}
	for _, root := range roots {
		clean := filepath.Clean(strings.TrimSpace(root))
		if clean == "." || clean == "/" || clean == "\\" || clean == "" {
			return fmt.Errorf("invalid edit root %q: broad root or dot wildcard is not allowed", root)
		}
		if filepath.IsAbs(root) {
			return fmt.Errorf("invalid edit root %q: absolute paths are not permitted", root)
		}
		if strings.HasPrefix(clean, "..") {
			return fmt.Errorf("invalid edit root %q: escaping repository root is not permitted", root)
		}
	}
	return nil
}

// IsPathWithinRoots checks if a given file path is covered by any of the allowed edit roots.
func IsPathWithinRoots(filePath string, allowedRoots []string) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(filePath))
	cleanPath = filepath.ToSlash(cleanPath)

	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(strings.TrimSpace(root))
		cleanRoot = filepath.ToSlash(cleanRoot)

		// Exact match or prefix match with trailing slash boundary
		if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, strings.TrimSuffix(cleanRoot, "/")+"/") {
			return true
		}
	}
	return false
}

// DeriveMissingRoots determines which paths in targetFiles are not covered by allowedRoots,
// and returns their normalized directory prefixes as missing roots.
func DeriveMissingRoots(targetFiles []string, allowedRoots []string) []string {
	seen := make(map[string]bool)
	var missing []string

	for _, file := range targetFiles {
		if !IsPathWithinRoots(file, allowedRoots) {
			dir := filepath.Dir(file)
			cleanDir := filepath.Clean(dir)
			if cleanDir == "." {
				cleanDir = file // root-level single file
			} else {
				cleanDir = filepath.ToSlash(cleanDir) + "/"
			}
			if !seen[cleanDir] {
				seen[cleanDir] = true
				missing = append(missing, cleanDir)
			}
		}
	}
	return missing
}

// EvaluateEditAuthority checks target files against allowed roots and returns a typed
// EditAuthorityConsent envelope if any file falls outside the authorized edit roots.
// Returns nil if all paths are authorized.
func EvaluateEditAuthority(targetFiles []string, allowedRoots []string) *EditAuthorityConsent {
	missing := DeriveMissingRoots(targetFiles, allowedRoots)
	if len(missing) == 0 {
		return nil
	}

	return &EditAuthorityConsent{
		Schema:       "gaia.sdd.consent/v1",
		Headline:     "Edit authority required for out-of-bounds paths",
		Reason:       "Task targets files outside currently authorized edit roots",
		MissingRoots: missing,
		AllowedRoots: allowedRoots,
		Choices: []EditConsentChoice{
			{
				Token:  "granted",
				Label:  "Grant edit authority for missing roots",
				Effect: "Adds missing roots to allowedEditRoots for this change",
			},
			{
				Token:  "declined",
				Label:  "Decline and edit task plan",
				Effect: "Change stays blocked(edit_authority_missing) until tasks.md is adjusted",
			},
		},
	}
}

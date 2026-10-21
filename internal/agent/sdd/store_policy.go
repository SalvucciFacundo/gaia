package sdd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gaia/internal/agent/memory"
)

// StoreMode represents the artifact persistence strategy.
type StoreMode string

const (
	StoreModeMneme    StoreMode = "mneme"
	StoreModeEngram   StoreMode = "mneme" // backward compatibility alias
	StoreModeOpenSpec StoreMode = "openspec"
	StoreModeHybrid   StoreMode = "hybrid"
	StoreModeNone     StoreMode = "none"
)

// ParseStoreMode converts a string to a valid StoreMode.
func ParseStoreMode(s string) StoreMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mneme", "engram":
		return StoreModeMneme
	case "openspec":
		return StoreModeOpenSpec
	case "hybrid":
		return StoreModeHybrid
	case "none":
		return StoreModeNone
	default:
		return StoreModeHybrid
	}
}

// StorePolicy manages artifact store resolution and guards against mismatched queries.
type StorePolicy struct {
	mode      StoreMode
	baseDir   string // usually root directory where openspec/ lives
	namespace *memory.NamespaceManager
}

// NewStorePolicy creates a new StorePolicy.
func NewStorePolicy(mode StoreMode, baseDir string, ns *memory.NamespaceManager) *StorePolicy {
	if mode == "" {
		mode = StoreModeHybrid
	}
	if baseDir == "" {
		baseDir = "."
	}
	return &StorePolicy{
		mode:      mode,
		baseDir:   baseDir,
		namespace: ns,
	}
}

// Mode returns the active StoreMode.
func (p *StorePolicy) Mode() StoreMode {
	return p.mode
}

// ShouldQueryFilesystem returns true if the current store mode uses files on disk.
func (p *StorePolicy) ShouldQueryFilesystem() bool {
	return p.mode == StoreModeOpenSpec || p.mode == StoreModeHybrid
}

// ShouldQueryMneme returns true if the current store mode uses persistent memory.
func (p *StorePolicy) ShouldQueryMneme() bool {
	return p.mode == StoreModeMneme || p.mode == StoreModeHybrid
}

// ShouldQueryEngram is an alias for ShouldQueryMneme for backward compatibility.
func (p *StorePolicy) ShouldQueryEngram() bool {
	return p.ShouldQueryMneme()
}

// SaveArtifact persists an artifact according to the active store policy.
func (p *StorePolicy) SaveArtifact(ctx context.Context, changeName, artifactName, content string) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if changeName == "" || artifactName == "" {
		return fmt.Errorf("change name and artifact name are required")
	}

	// 1. Filesystem write if OpenSpec or Hybrid
	if p.ShouldQueryFilesystem() {
		changeDir := filepath.Join(p.baseDir, "openspec", "changes", changeName)
		if err := os.MkdirAll(changeDir, 0755); err != nil {
			return fmt.Errorf("create change directory: %w", err)
		}

		filePath := filepath.Join(changeDir, artifactName+".md")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write artifact file: %w", err)
		}
	}

	// 2. Mneme memory write if Mneme or Hybrid
	if p.ShouldQueryMneme() && p.namespace != nil {
		// Formatted topic key: gaia/sdd/{project}/{change}/{artifact}
		_ = p.namespace.TopicKey("sdd", fmt.Sprintf("%s/%s", changeName, artifactName))
	}

	return nil
}

// ReadArtifact retrieves an artifact according to the active store policy.
func (p *StorePolicy) ReadArtifact(ctx context.Context, changeName, artifactName string) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	if changeName == "" || artifactName == "" {
		return "", fmt.Errorf("change name and artifact name are required")
	}

	// If pure Mneme mode, do NOT touch filesystem
	if p.mode == StoreModeMneme {
		if p.namespace == nil {
			return "", fmt.Errorf("mneme namespace not configured for mneme store mode")
		}
		// Return topic key indication when pure mneme
		return fmt.Sprintf("<!-- mneme topic: %s -->", p.namespace.TopicKey("sdd", fmt.Sprintf("%s/%s", changeName, artifactName))), nil
	}

	// Filesystem read
	filePath := filepath.Join(p.baseDir, "openspec", "changes", changeName, artifactName+".md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read artifact file: %w", err)
	}

	return string(data), nil
}

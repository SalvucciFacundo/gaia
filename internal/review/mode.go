package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReviewMode represents the operational state of Receipt-Driven Development review.
type ReviewMode string

const (
	ModeEnabled  ReviewMode = "enabled"
	ModeDisabled ReviewMode = "disabled"
)

// ReviewScope represents whether the mode applies to the current repo clone or machine-wide.
type ReviewScope string

const (
	ScopeClone  ReviewScope = "clone"
	ScopeGlobal ReviewScope = "global"
)

// ModeStatus represents the effective review mode and its deciding source.
type ModeStatus struct {
	Mode           ReviewMode  `json:"mode"`
	DecidingSource string      `json:"deciding_source"` // "clone", "global", or "default"
	Scope          ReviewScope `json:"scope"`
}

// IsEnabled checks if review mode is enabled for a given repository.
// By default, Receipt-Driven Development is OPT-IN and off by default.
func IsEnabled(repoRoot string) ModeStatus {
	// 1. Check clone-scoped setting
	if repoRoot != "" {
		clonePath := filepath.Join(repoRoot, ".gaia", "review-mode")
		if data, err := os.ReadFile(clonePath); err == nil {
			mode := strings.TrimSpace(string(data))
			if mode == string(ModeEnabled) {
				return ModeStatus{Mode: ModeEnabled, DecidingSource: "clone", Scope: ScopeClone}
			}
			return ModeStatus{Mode: ModeDisabled, DecidingSource: "clone", Scope: ScopeClone}
		}
	}

	// 2. Check global setting in ~/.gaia/review-mode
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(homeDir, ".gaia", "review-mode")
		if data, err := os.ReadFile(globalPath); err == nil {
			mode := strings.TrimSpace(string(data))
			if mode == string(ModeEnabled) {
				return ModeStatus{Mode: ModeEnabled, DecidingSource: "global", Scope: ScopeGlobal}
			}
			return ModeStatus{Mode: ModeDisabled, DecidingSource: "global", Scope: ScopeGlobal}
		}
	}

	// 3. Default: off
	return ModeStatus{
		Mode:           ModeDisabled,
		DecidingSource: "default",
		Scope:          ScopeGlobal,
	}
}

// SetMode configures review mode for clone or global scope.
func SetMode(mode ReviewMode, scope ReviewScope, repoRoot string) error {
	if mode != ModeEnabled && mode != ModeDisabled {
		return fmt.Errorf("invalid mode %q: must be 'enabled' or 'disabled'", mode)
	}

	var targetPath string
	if scope == ScopeClone {
		if repoRoot == "" {
			return fmt.Errorf("repository root required for clone scope")
		}
		targetPath = filepath.Join(repoRoot, ".gaia", "review-mode")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determine user home directory: %w", err)
		}
		targetPath = filepath.Join(homeDir, ".gaia", "review-mode")
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(targetPath, []byte(mode), 0644); err != nil {
		return fmt.Errorf("write review-mode file: %w", err)
	}

	return nil
}

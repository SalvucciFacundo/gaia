package gates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HooksDir is the standard git hooks directory within a repository.
const HooksDir = ".git/hooks"

// HookScript is a shell script that wraps a gaia review validate call.
type HookScript struct {
	// Gate is the gate name to validate ("pre-commit", "pre-push", "pre-pr").
	Gate GateName
	// Command is the CLI command that runs the gate (default: "gaia review validate").
	Command string
}

// Pre-commit hook: validates staged changes before committing.
var PreCommitHook = HookScript{
	Gate:    PreCommitGate,
	Command: "gaia",
}

// Pre-push hook: validates before pushing to a remote.
var PrePushHook = HookScript{
	Gate:    PrePushGate,
	Command: "gaia",
}

// hookScriptTemplate generates a POSIX-compatible shell script for a gate.
// It is designed to work on Unix (sh/bash) and Windows (git bash from Git for Windows).
func (h *HookScript) scriptContent() string {
	return fmt.Sprintf(`#!/bin/sh
# GAIA Review Gate: %s
# Installed by: gaia review install-hooks
# This hook validates that a review receipt exists and matches current content.

set -e

GATE="%s"
GAIA_CMD="%s"
HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$HOOK_DIR/../.." && pwd)"

echo "[GAIA] Running %s gate validation..."

# Get staged files (or all tracked files for pre-push).
if [ "$GATE" = "pre-commit" ]; then
    FILES=$(cd "$REPO_ROOT" && git diff --cached --name-only --diff-filter=ACM)
elif [ "$GATE" = "pre-push" ]; then
    # For pre-push, validate all tracked files in the HEAD commit.
    FILES=$(cd "$REPO_ROOT" && git diff --name-only HEAD --diff-filter=ACM)
else
    FILES=$(cd "$REPO_ROOT" && git ls-files --exclude-standard)
fi

if [ -z "$FILES" ]; then
    echo "[GAIA] No files to validate. Gate passed."
    exit 0
fi

# Convert files to a comma-separated list for the CLI.
FILE_LIST=$(echo "$FILES" | tr '\n' ',' | sed 's/,$//')

# Run gate validation via the GAIA CLI.
cd "$REPO_ROOT"
$GAIA_CMD review validate --gate "$GATE" --files "$FILE_LIST"
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "[GAIA] %s gate passed."
    exit 0
else
    echo "[GAIA] %s gate FAILED. Run 'gaia review start' to review changes."
    exit 1
fi
`, h.Gate, h.Gate, h.Command, h.Gate, h.Gate, h.Gate)
}

// WriteHooks installs the pre-commit and pre-push hook scripts into the
// git repository at projectRoot. It creates the hooks directory if needed,
// appends GAIA content to existing hooks (does not overwrite), and
// returns an error if projectRoot is not a git repository.
func WriteHooks(projectRoot string) error {
	hooksDir := filepath.Join(projectRoot, HooksDir)
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s not found (run 'git init' first)", hooksDir)
	}

	hooks := []HookScript{PreCommitHook, PrePushHook}
	for _, hook := range hooks {
		hookPath := filepath.Join(hooksDir, string(hook.Gate))
		if err := installHook(hookPath, hook.scriptContent()); err != nil {
			return fmt.Errorf("install %s hook: %w", hook.Gate, err)
		}
	}

	return nil
}

// installHook writes a hook script to the given path. If the hook already
// exists and does NOT already contain GAIA content, the GAIA block is
// appended. If GAIA content already exists, it is NOT duplicated.
func installHook(path, content string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing hook: %w", err)
	}

	// If the hook already has GAIA content, do nothing.
	if strings.Contains(string(existing), "GAIA Review Gate") {
		return nil
	}

	// Append GAIA content to existing hook (or write fresh).
	var final []byte
	if len(existing) > 0 {
		final = append(existing, '\n')
	}
	final = append(final, []byte(content)...)

	if err := os.WriteFile(path, final, 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	return nil
}

// UninstallHooks removes the GAIA block from existing hook scripts.
// It preserves any non-GAIA hook content that was present before installation.
func UninstallHooks(projectRoot string) error {
	hooksDir := filepath.Join(projectRoot, HooksDir)
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil // nothing to uninstall
	}

	hooks := []GateName{PreCommitGate, PrePushGate}
	for _, gate := range hooks {
		hookPath := filepath.Join(hooksDir, string(gate))
		data, err := os.ReadFile(hookPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read hook %s: %w", gate, err)
		}

		content := string(data)
		gaiaStart := strings.Index(content, "# GAIA Review Gate")
		if gaiaStart < 0 {
			continue
		}

		// Remove everything from "# GAIA Review Gate" to end of file.
		clean := strings.TrimRight(content[:gaiaStart], "\n") + "\n"
		if strings.TrimSpace(clean) == "" {
			os.Remove(hookPath)
		} else {
			os.WriteFile(hookPath, []byte(clean), 0755)
		}
	}

	return nil
}

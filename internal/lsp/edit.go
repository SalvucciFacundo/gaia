package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// SortTextEditsReverse sorts edits in reverse document order:
// descending by End line and character, then Start line and character.
func SortTextEditsReverse(edits []TextEdit) {
	sort.SliceStable(edits, func(i, j int) bool {
		ei := edits[i].Range
		ej := edits[j].Range

		if ei.End.Line != ej.End.Line {
			return ei.End.Line > ej.End.Line
		}
		if ei.End.Character != ej.End.Character {
			return ei.End.Character > ej.End.Character
		}
		if ei.Start.Line != ej.Start.Line {
			return ei.Start.Line > ej.Start.Line
		}
		return ei.Start.Character > ej.Start.Character
	})
}

// positionToOffset converts a 0-indexed LSP Position to a character offset in content.
func positionToOffset(content string, lineStarts []int, pos Position) (int, error) {
	if pos.Line < 0 {
		return 0, fmt.Errorf("negative line number %d", pos.Line)
	}
	if pos.Character < 0 {
		return 0, fmt.Errorf("negative character offset %d", pos.Character)
	}

	if pos.Line == len(lineStarts) {
		if pos.Character == 0 {
			return len(content), nil
		}
		return 0, fmt.Errorf("position line %d out of bounds (total lines %d)", pos.Line, len(lineStarts))
	}
	if pos.Line > len(lineStarts) {
		return 0, fmt.Errorf("position line %d out of bounds (total lines %d)", pos.Line, len(lineStarts))
	}

	lineStart := lineStarts[pos.Line]
	var lineEnd int
	if pos.Line+1 < len(lineStarts) {
		lineEnd = lineStarts[pos.Line+1]
	} else {
		lineEnd = len(content)
	}

	offset := lineStart + pos.Character
	if offset > lineEnd {
		return 0, fmt.Errorf("character offset %d exceeds line %d length (%d)", pos.Character, pos.Line, lineEnd-lineStart)
	}
	return offset, nil
}

// computeLineStarts returns the byte offset of each line start in content.
func computeLineStarts(content string) []int {
	var starts []int
	starts = append(starts, 0)
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// ApplyTextEdits applies a slice of TextEdits to the given text content.
// It sorts edits in reverse order, checks for overlapping ranges, and applies modifications.
func ApplyTextEdits(content string, edits []TextEdit) (string, error) {
	if len(edits) == 0 {
		return content, nil
	}

	// Copy edits so we don't mutate the input slice order unexpectedly
	sortedEdits := make([]TextEdit, len(edits))
	copy(sortedEdits, edits)
	SortTextEditsReverse(sortedEdits)

	lineStarts := computeLineStarts(content)

	type resolvedEdit struct {
		startOffset int
		endOffset   int
		newText     string
		orig        TextEdit
	}

	resolved := make([]resolvedEdit, len(sortedEdits))
	for i, e := range sortedEdits {
		startOff, err := positionToOffset(content, lineStarts, e.Range.Start)
		if err != nil {
			return "", fmt.Errorf("invalid start position %+v: %w", e.Range.Start, err)
		}
		endOff, err := positionToOffset(content, lineStarts, e.Range.End)
		if err != nil {
			return "", fmt.Errorf("invalid end position %+v: %w", e.Range.End, err)
		}
		if startOff > endOff {
			return "", fmt.Errorf("invalid range: start offset %d > end offset %d", startOff, endOff)
		}

		resolved[i] = resolvedEdit{
			startOffset: startOff,
			endOffset:   endOff,
			newText:     e.NewText,
			orig:        e,
		}
	}

	// Check for overlapping edits (since sorted descending by endOffset)
	for i := 1; i < len(resolved); i++ {
		prev := resolved[i-1] // later in file
		curr := resolved[i]   // earlier in file
		if curr.endOffset > prev.startOffset {
			return "", fmt.Errorf("overlapping text edits detected between [%+v] and [%+v]", curr.orig.Range, prev.orig.Range)
		}
	}

	// Apply edits from bottom to top
	result := content
	for _, edit := range resolved {
		result = result[:edit.startOffset] + edit.newText + result[edit.endOffset:]
	}

	return result, nil
}

// ApplyResult contains the outcome of applying a WorkspaceEdit.
type ApplyResult struct {
	ModifiedFiles map[string]int `json:"modifiedFiles"` // file path -> number of edits
	TotalEdits    int            `json:"totalEdits"`
}

// WorkspaceEditApplier handles safe, atomic application of multi-file WorkspaceEdits.
type WorkspaceEditApplier struct{}

// NewWorkspaceEditApplier creates a new WorkspaceEditApplier instance.
func NewWorkspaceEditApplier() *WorkspaceEditApplier {
	return &WorkspaceEditApplier{}
}

// Apply executes a WorkspaceEdit across all target files with atomic backup and rollback.
func (a *WorkspaceEditApplier) Apply(we *WorkspaceEdit) (*ApplyResult, error) {
	if we == nil {
		return &ApplyResult{ModifiedFiles: make(map[string]int)}, nil
	}

	changes := we.NormalizedChanges()
	if len(changes) == 0 {
		return &ApplyResult{ModifiedFiles: make(map[string]int)}, nil
	}

	type filePlan struct {
		path       string
		newContent string
		editCount  int
	}

	backups := make(map[string][]byte)
	existed := make(map[string]bool)
	plans := make([]filePlan, 0, len(changes))
	totalEdits := 0
	modifiedFiles := make(map[string]int)

	// Phase 1: Read all files, calculate edits, fail-fast in memory before touching disk
	for uri, edits := range changes {
		filePath := URIToPath(uri)
		var originalContent string

		if data, err := os.ReadFile(filePath); err == nil {
			backups[filePath] = data
			existed[filePath] = true
			originalContent = string(data)
		} else if os.IsNotExist(err) {
			// If file does not exist, edits apply to empty content if creation edit
			backups[filePath] = nil
			existed[filePath] = false
			originalContent = ""
		} else {
			return nil, fmt.Errorf("failed to read target file %s: %w", filePath, err)
		}

		newContent, err := ApplyTextEdits(originalContent, edits)
		if err != nil {
			return nil, fmt.Errorf("failed to apply text edits to %s: %w", filePath, err)
		}

		plans = append(plans, filePlan{
			path:       filePath,
			newContent: newContent,
			editCount:  len(edits),
		})
		totalEdits += len(edits)
		modifiedFiles[filePath] = len(edits)
	}

	// Phase 2: Atomic write to disk with rollback tracking
	var writtenFiles []string
	rollback := func() {
		for _, writtenPath := range writtenFiles {
			if existed[writtenPath] {
				_ = os.WriteFile(writtenPath, backups[writtenPath], 0644)
			} else {
				_ = os.Remove(writtenPath)
			}
		}
	}

	for _, plan := range plans {
		dir := filepath.Dir(plan.path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			rollback()
			return nil, fmt.Errorf("failed to create directory for %s: %w", plan.path, err)
		}

		if err := os.WriteFile(plan.path, []byte(plan.newContent), 0644); err != nil {
			rollback()
			return nil, fmt.Errorf("failed to write file %s: %w", plan.path, err)
		}
		writtenFiles = append(writtenFiles, plan.path)
	}

	return &ApplyResult{
		ModifiedFiles: modifiedFiles,
		TotalEdits:    totalEdits,
	}, nil
}

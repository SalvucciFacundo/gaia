package diff

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// LineType classifies a line within a diff hunk.
type LineType int

const (
	LineContext LineType = iota
	LineAddition
	LineDeletion
)

// DiffLine represents a single addition, deletion, or context line.
type DiffLine struct {
	Type    LineType
	Content string
	OldLine int
	NewLine int
}

// DiffHunk represents a modified block with metadata for patch generation and display.
type DiffHunk struct {
	Header   string
	Lines    []DiffLine
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	IsStaged bool
}

// DiffFile contains path metadata and its associated hunks.
type DiffFile struct {
	OldPath  string
	NewPath  string
	IsBinary bool
	Hunks    []DiffHunk
}

// ParseUnifiedDiff parses a raw unified diff string into a slice of DiffFile.
func ParseUnifiedDiff(raw string) ([]DiffFile, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []DiffFile{}, nil
	}

	var files []DiffFile
	scanner := bufio.NewScanner(strings.NewReader(raw))

	var currentFile *DiffFile
	var currentHunk *DiffHunk

	flushHunk := func() {
		if currentFile != nil && currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			currentHunk = nil
		}
	}

	flushFile := func() {
		flushHunk()
		if currentFile != nil {
			files = append(files, *currentFile)
			currentFile = nil
		}
	}

	oldLineCounter := 0
	newLineCounter := 0

	for scanner.Scan() {
		line := scanner.Text()

		// New file diff header
		if strings.HasPrefix(line, "diff --git ") {
			flushFile()
			parts := strings.Split(line, " ")
			oldPath := ""
			newPath := ""
			if len(parts) >= 4 {
				oldPath = strings.TrimPrefix(parts[2], "a/")
				newPath = strings.TrimPrefix(parts[3], "b/")
			}
			currentFile = &DiffFile{
				OldPath: oldPath,
				NewPath: newPath,
				Hunks:   []DiffHunk{},
			}
			continue
		}

		if currentFile == nil {
			// In case diff doesn't start with diff --git (e.g. plain patch)
			if strings.HasPrefix(line, "--- ") {
				currentFile = &DiffFile{
					OldPath: cleanFilePath(line),
					Hunks:   []DiffHunk{},
				}
			} else {
				continue
			}
		}

		// Check for binary files
		if strings.HasPrefix(line, "Binary files ") && strings.HasSuffix(line, " differ") {
			currentFile.IsBinary = true
			continue
		}

		// Parse --- and +++ lines if needed to update paths
		if strings.HasPrefix(line, "--- ") {
			path := cleanFilePath(line)
			if path != "/dev/null" && currentFile.OldPath == "" {
				currentFile.OldPath = path
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			path := cleanFilePath(line)
			if path != "/dev/null" && currentFile.NewPath == "" {
				currentFile.NewPath = path
			}
			continue
		}

		// Hunk header: @@ -oldStart,oldLines +newStart,newLines @@
		if strings.HasPrefix(line, "@@ ") {
			flushHunk()
			oldStart, oldLines, newStart, newLines, err := parseHunkHeader(line)
			if err == nil {
				currentHunk = &DiffHunk{
					Header:   line,
					OldStart: oldStart,
					OldLines: oldLines,
					NewStart: newStart,
					NewLines: newLines,
					Lines:    []DiffLine{},
				}
				oldLineCounter = oldStart
				newLineCounter = newStart
			}
			continue
		}

		// Inside a hunk
		if currentHunk != nil {
			if strings.HasPrefix(line, "+") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    LineAddition,
					Content: line[1:],
					OldLine: 0,
					NewLine: newLineCounter,
				})
				newLineCounter++
			} else if strings.HasPrefix(line, "-") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    LineDeletion,
					Content: line[1:],
					OldLine: oldLineCounter,
					NewLine: 0,
				})
				oldLineCounter++
			} else if strings.HasPrefix(line, " ") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    LineContext,
					Content: line[1:],
					OldLine: oldLineCounter,
					NewLine: newLineCounter,
				})
				oldLineCounter++
				newLineCounter++
			} else if line == `\ No newline at end of file` {
				// Special context/marker line
				continue
			}
		}
	}

	flushFile()

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

// BuildHunkPatch constructs a valid unified diff patch string for a single hunk in the file.
func (f *DiffFile) BuildHunkPatch(hunkIndex int) string {
	if hunkIndex < 0 || hunkIndex >= len(f.Hunks) {
		return ""
	}

	hunk := f.Hunks[hunkIndex]
	var sb strings.Builder

	oldPath := f.OldPath
	if oldPath == "" {
		oldPath = f.NewPath
	}
	newPath := f.NewPath
	if newPath == "" {
		newPath = f.OldPath
	}

	sb.WriteString(fmt.Sprintf("--- a/%s\n", oldPath))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", newPath))
	sb.WriteString(hunk.Header)
	sb.WriteString("\n")

	for _, line := range hunk.Lines {
		switch line.Type {
		case LineAddition:
			sb.WriteString("+")
			sb.WriteString(line.Content)
			sb.WriteString("\n")
		case LineDeletion:
			sb.WriteString("-")
			sb.WriteString(line.Content)
			sb.WriteString("\n")
		case LineContext:
			sb.WriteString(" ")
			sb.WriteString(line.Content)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func cleanFilePath(line string) string {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	path := strings.TrimSpace(parts[1])
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}

// parseHunkHeader parses "@@ -oldStart[,oldLines] +newStart[,newLines] @@"
func parseHunkHeader(header string) (int, int, int, int, error) {
	parts := strings.Split(header, "@@")
	if len(parts) < 3 {
		return 0, 0, 0, 0, fmt.Errorf("invalid hunk header: %s", header)
	}

	rangeSection := strings.TrimSpace(parts[1])
	rangeParts := strings.Split(rangeSection, " ")
	if len(rangeParts) < 2 {
		return 0, 0, 0, 0, fmt.Errorf("invalid hunk range: %s", rangeSection)
	}

	oldStart, oldLines, err1 := parseRange(rangeParts[0], "-")
	newStart, newLines, err2 := parseRange(rangeParts[1], "+")

	if err1 != nil || err2 != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to parse hunk ranges: %v / %v", err1, err2)
	}

	return oldStart, oldLines, newStart, newLines, nil
}

func parseRange(r, prefix string) (int, int, error) {
	r = strings.TrimPrefix(r, prefix)
	parts := strings.Split(r, ",")
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}

	lines := 1
	if len(parts) > 1 {
		l, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
		lines = l
	}

	return start, lines, nil
}

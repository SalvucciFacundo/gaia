package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var atMentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_\-\./\\]+)`)

// ImageExtensions contains supported image file extensions.
var ImageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

// ImageAttacher represents an agent capable of attaching image files for vision models.
type ImageAttacher interface {
	AttachImage(ctx context.Context, path string) error
}

// IngestMentions scans content for @path references.
// If a path is an image, it attaches it to the Brain via AttachImage.
// If a path is a text file, it reads the file (up to 50KB) and inlines its content into the prompt.
// If a path is an existing subagent name, it is left untouched for direct agent routing.
func IngestMentions(ctx context.Context, attacher ImageAttacher, content string, subagents []string) (string, error) {
	if !strings.Contains(content, "@") {
		return content, nil
	}

	subagentMap := make(map[string]bool)
	for _, sa := range subagents {
		subagentMap[strings.ToLower(sa)] = true
	}

	matches := atMentionRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	var inlinedContext strings.Builder
	handledPaths := make(map[string]bool)

	for _, m := range matches {
		target := m[1]
		if subagentMap[strings.ToLower(target)] {
			continue // Skip subagent names (e.g. @implementer)
		}

		cleanPath := filepath.Clean(target)
		info, err := os.Stat(cleanPath)
		if err != nil {
			continue // Not a local file on disk
		}

		if info.IsDir() {
			continue
		}

		if handledPaths[cleanPath] {
			continue
		}
		handledPaths[cleanPath] = true

		ext := strings.ToLower(filepath.Ext(cleanPath))

		// Check if it's an image
		if ImageExtensions[ext] {
			if attacher != nil {
				_ = attacher.AttachImage(ctx, cleanPath)
			}
			continue
		}

		// Text / general file injection
		const maxFileSize = 50 * 1024 // 50 KB safety limit per file
		fileSize := info.Size()
		if fileSize > maxFileSize {
			inlinedContext.WriteString(fmt.Sprintf(
				"\n\n[Attached File: %s (truncated: size %.1f KB exceeds 50 KB limit)]\n",
				cleanPath, float64(fileSize)/1024,
			))
			data, readErr := os.ReadFile(cleanPath)
			if readErr == nil {
				inlinedContext.WriteString(string(data[:maxFileSize]))
				inlinedContext.WriteString("\n...[TRUNCATED]...\n")
			}
		} else {
			data, readErr := os.ReadFile(cleanPath)
			if readErr == nil {
				// Safety check: verify it's valid UTF-8 text, not arbitrary binary
				if utf8.Valid(data) {
					inlinedContext.WriteString(fmt.Sprintf("\n\n[Attached File: %s]\n```\n%s\n```\n", cleanPath, string(data)))
				} else {
					inlinedContext.WriteString(fmt.Sprintf("\n\n[Attached File: %s (binary file, omitted)]\n", cleanPath))
				}
			}
		}
	}

	if inlinedContext.Len() > 0 {
		return content + inlinedContext.String(), nil
	}

	return content, nil
}

package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultSlashCommands returns all known slash commands for autocompletion.
var DefaultSlashCommands = []string{
	"/help",
	"/clear",
	"/diff",
	"/paste",
	"/image",
	"/model",
	"/models",
	"/undo",
	"/retry",
	"/new",
	"/save",
	"/resume",
	"/sessions",
	"/title",
	"/compress",
	"/branch",
	"/branches",
	"/snapshot",
	"/goal",
	"/subgoal",
	"/goals",
	"/queue",
	"/steer",
	"/fast",
	"/yolo",
	"/trust",
	"/permisos",
	"/reasoning",
	"/personality",
	"/skin",
	"/tasks",
	"/cancel",
	"/usage",
	"/version",
	"/timestamps",
	"/statusbar",
	"/footer",
	"/verbose",
	"/indicator",
	"/skills",
	"/cron",
	"/copy",
	"/reload-mcp",
	"/reload-skills",
}

// CompleteWord calculates an autocompletion for the current input word.
// It supports:
// - Slash commands (e.g., "/di" -> "/diff")
// - Subagent / file mentions (e.g., "@rev" -> "@reviewer", "@pkg/calc" -> "@pkg/calc.go")
// - Filesystem paths (e.g., "internal/core/k" -> "internal/core/kernel.go")
func CompleteWord(input string, subagents []string) (string, bool) {
	if input == "" {
		return input, false
	}

	// Find the last token delimited by whitespace
	idx := strings.LastIndexAny(input, " \t\n")
	prefix := ""
	lastToken := input
	if idx != -1 {
		prefix = input[:idx+1]
		lastToken = input[idx+1:]
	}

	if lastToken == "" {
		return input, false
	}

	// 1. Slash commands completion
	if strings.HasPrefix(lastToken, "/") && idx == -1 {
		for _, cmd := range DefaultSlashCommands {
			if strings.HasPrefix(cmd, lastToken) && cmd != lastToken {
				return cmd + " ", true
			}
		}
		return input, false
	}

	// 2. Subagent or file @ mention completion
	if strings.HasPrefix(lastToken, "@") {
		mention := strings.TrimPrefix(lastToken, "@")

		// First, check subagents
		for _, sa := range subagents {
			if strings.HasPrefix(sa, mention) && sa != mention {
				return prefix + "@" + sa + " ", true
			}
		}

		// Second, check files with @
		if mention != "" {
			if completedPath, ok := completeFilePath(mention); ok {
				return prefix + "@" + completedPath, true
			}
		}
		return input, false
	}

	// 3. Regular path completion
	if completedPath, ok := completeFilePath(lastToken); ok {
		return prefix + completedPath, true
	}

	return input, false
}

func completeFilePath(prefix string) (string, bool) {
	dir := filepath.Dir(prefix)
	base := filepath.Base(prefix)

	if strings.HasSuffix(prefix, "/") || strings.HasSuffix(prefix, string(filepath.Separator)) {
		dir = prefix
		base = ""
	}

	searchDir := dir
	if searchDir == "." && !strings.HasPrefix(prefix, ".") {
		searchDir = "."
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return prefix, false
	}

	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue // skip hidden files unless explicitly typed
		}
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(base)) {
			matchedPath := name
			if dir != "." && dir != "" {
				matchedPath = filepath.Join(dir, name)
			}
			if entry.IsDir() {
				matchedPath += "/"
			}
			matches = append(matches, matchedPath)
		}
	}

	if len(matches) == 1 {
		return matches[0], true
	}

	// If multiple matches, find longest common prefix
	if len(matches) > 1 {
		sort.Strings(matches)
		common := longestCommonPrefix(matches)
		if len(common) > len(prefix) {
			return common, true
		}
	}

	return prefix, false
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

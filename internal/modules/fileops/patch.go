package fileops

import (
	"fmt"
	"strings"
)

// normalizeLF converts CRLF into standard LF line endings.
func normalizeLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// stripLineWhitespace strips trailing whitespace and carriage returns from a line.
func cleanLine(s string) string {
	return strings.TrimRight(s, " \t\r")
}

// ApplyPatch replaces searchTarget in source with replaceWith using exact or fuzzy line matching.
func ApplyPatch(source, searchTarget, replaceWith string) (string, error) {
	normSource := normalizeLF(source)
	normSearch := normalizeLF(searchTarget)
	normReplace := normalizeLF(replaceWith)

	if normSearch == "" {
		return "", fmt.Errorf("search block cannot be empty")
	}

	// 1. Exact match attempt
	if count := strings.Count(normSource, normSearch); count == 1 {
		res := strings.Replace(normSource, normSearch, normReplace, 1)
		// Preserve CRLF if original had CRLF and wasn't purely LF
		if strings.Contains(source, "\r\n") && !strings.Contains(normReplace, "\r\n") {
			res = strings.ReplaceAll(res, "\n", "\r\n")
		}
		return res, nil
	} else if count > 1 {
		return "", fmt.Errorf("search block is not unique: found %d matches (provide more surrounding context lines)", count)
	}

	// 2. Fuzzy match attempt: match lines ignoring leading/trailing whitespace & indentations
	srcLines := strings.Split(normSource, "\n")
	searchLines := strings.Split(normSearch, "\n")

	// Filter empty trailing lines from search if any
	for len(searchLines) > 0 && searchLines[len(searchLines)-1] == "" {
		searchLines = searchLines[:len(searchLines)-1]
	}
	if len(searchLines) == 0 {
		return "", fmt.Errorf("search block contains no valid lines")
	}

	matchIdx := -1
	matchesFound := 0

	for i := 0; i <= len(srcLines)-len(searchLines); i++ {
		matched := true
		for j := 0; j < len(searchLines); j++ {
			srcClean := cleanLine(strings.TrimSpace(srcLines[i+j]))
			searchClean := cleanLine(strings.TrimSpace(searchLines[j]))
			if srcClean != searchClean {
				matched = false
				break
			}
		}
		if matched {
			matchesFound++
			matchIdx = i
		}
	}

	if matchesFound == 0 {
		return "", fmt.Errorf("search block not found in file (verify surrounding context)")
	}
	if matchesFound > 1 {
		return "", fmt.Errorf("fuzzy search block is ambiguous: found %d matches (provide more surrounding context lines)", matchesFound)
	}

	// Calculate base indentation if replace block needs alignment
	baseIndent := ""
	firstOrigLine := srcLines[matchIdx]
	for _, r := range firstOrigLine {
		if r == ' ' || r == '\t' {
			baseIndent += string(r)
		} else {
			break
		}
	}

	replaceLines := strings.Split(normReplace, "\n")
	// If first line of search had no indent in prompt but orig line did, align replace lines
	firstSearchIndent := ""
	for _, r := range searchLines[0] {
		if r == ' ' || r == '\t' {
			firstSearchIndent += string(r)
		} else {
			break
		}
	}

	var finalReplaceLines []string
	if firstSearchIndent == "" && baseIndent != "" {
		for _, rl := range replaceLines {
			if rl == "" {
				finalReplaceLines = append(finalReplaceLines, "")
			} else {
				finalReplaceLines = append(finalReplaceLines, baseIndent+rl)
			}
		}
	} else {
		finalReplaceLines = replaceLines
	}

	// Construct final content
	var newLines []string
	newLines = append(newLines, srcLines[:matchIdx]...)
	newLines = append(newLines, finalReplaceLines...)
	newLines = append(newLines, srcLines[matchIdx+len(searchLines):]...)

	result := strings.Join(newLines, "\n")
	if strings.Contains(source, "\r\n") {
		result = strings.ReplaceAll(result, "\n", "\r\n")
	}
	return result, nil
}

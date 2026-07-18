package tracker

import (
	"regexp"
	"strings"
)

// ChangeType classifies a changelog entry.
type ChangeType string

const (
	ChangeFeature        ChangeType = "feature"
	ChangeFix            ChangeType = "fix"
	ChangeProtocolChange ChangeType = "protocol-change"
	ChangeRefactor       ChangeType = "refactor"
	ChangeDocs           ChangeType = "docs"
	ChangeOther          ChangeType = "other"
)

// Change represents a single classified changelog entry.
type Change struct {
	Raw         string
	Type        ChangeType
	Description string
	Release     string
}

// ChangelogAnalyzer parses release bodies into structured, classified changes
// using Conventional Commits pattern matching.
type ChangelogAnalyzer struct{}

// conventional commit prefix patterns.
var (
	reFeature  = regexp.MustCompile(`(?im)^[\s*-]*feat(?:\([^)]*\))?[:\s!]+(.+)$`)
	reFix      = regexp.MustCompile(`(?im)^[\s*-]*fix(?:\([^)]*\))?[:\s!]+(.+)$`)
	reBreaking = regexp.MustCompile(`(?im)BREAKING\s*CHANGE[:\s]+(.+)$`)
	reRefactor = regexp.MustCompile(`(?im)^[\s*-]*refactor(?:\([^)]*\))?[:\s!]+(.+)$`)
	reDocs     = regexp.MustCompile(`(?im)^[\s*-]*docs(?:\([^)]*\))?[:\s!]+(.+)$`)
	rePerf     = regexp.MustCompile(`(?im)^[\s*-]*perf(?:\([^)]*\))?[:\s!]+(.+)$`)
	reChore    = regexp.MustCompile(`(?im)^[\s*-]*chore(?:\([^)]*\))?[:\s!]+(.+)$`)
	reStyle    = regexp.MustCompile(`(?im)^[\s*-]*style(?:\([^)]*\))?[:\s!]+(.+)$`)
)

// AnalyzeRelease parses a release body into a slice of classified changes.
func (a *ChangelogAnalyzer) AnalyzeRelease(release Release) []Change {
	return a.classifyLines(release.Body, release.Tag)
}

// DiffReleases returns changes present in the "to" release but not in the "from" release.
func (a *ChangelogAnalyzer) DiffReleases(from, to Release) []Change {
	fromChanges := a.AnalyzeRelease(from)
	toChanges := a.AnalyzeRelease(to)

	// Build a set of from descriptions for comparison.
	fromSet := make(map[string]bool)
	for _, c := range fromChanges {
		fromSet[normalizeDesc(c.Description)] = true
	}

	var diff []Change
	for _, c := range toChanges {
		if !fromSet[normalizeDesc(c.Description)] {
			diff = append(diff, c)
		}
	}
	return diff
}

func (a *ChangelogAnalyzer) classifyLines(body, releaseTag string) []Change {
	lines := strings.Split(body, "\n")
	var changes []Change

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		change := Change{
			Raw:     line,
			Release: releaseTag,
		}

		switch {
		case matches(line, reBreaking):
			change.Type = ChangeProtocolChange
			change.Description = extractDesc(line, reBreaking)
		case matches(line, reFeature):
			change.Type = ChangeFeature
			change.Description = extractDesc(line, reFeature)
		case matches(line, reFix):
			change.Type = ChangeFix
			change.Description = extractDesc(line, reFix)
		case matches(line, reRefactor):
			change.Type = ChangeRefactor
			change.Description = extractDesc(line, reRefactor)
		case matches(line, reDocs):
			change.Type = ChangeDocs
			change.Description = extractDesc(line, reDocs)
		case matches(line, rePerf):
			change.Type = ChangeRefactor
			change.Description = extractDesc(line, rePerf)
		case matches(line, reChore):
			change.Type = ChangeOther
			change.Description = extractDesc(line, reChore)
		case matches(line, reStyle):
			change.Type = ChangeOther
			change.Description = extractDesc(line, reStyle)
		default:
			// Non-standard entries.
			change.Type = ChangeOther
			change.Description = line
		}

		if change.Description != "" {
			changes = append(changes, change)
		}
	}

	return changes
}

func matches(line string, re *regexp.Regexp) bool {
	return re.MatchString(line)
}

func extractDesc(line string, re *regexp.Regexp) string {
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return strings.TrimSpace(line)
}

func normalizeDesc(desc string) string {
	return strings.TrimSpace(strings.ToLower(desc))
}

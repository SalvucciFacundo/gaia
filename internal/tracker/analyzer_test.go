package tracker

import (
	"testing"
	"time"
)

func TestAnalyzeRelease_ClassifyConventionalCommits(t *testing.T) {
	release := Release{
		Tag: "v2.0.0",
		Body: `feat: add streaming support
fix: resolve nil pointer dereference
BREAKING CHANGE: protocol v2 uses new message format
refactor: extract auth middleware
docs: update API reference
chore: bump dependencies
Some freeform text that does not match any pattern`,
		PublishedAt: time.Now(),
		HTMLURL:     "https://github.com/org/repo/releases/tag/v2.0.0",
	}

	a := &ChangelogAnalyzer{}
	changes := a.AnalyzeRelease(release)

	if len(changes) < 6 {
		t.Fatalf("got %d changes, want at least 6", len(changes))
	}

	// Verify classification.
	for _, c := range changes {
		switch {
		case c.Description == "add streaming support":
			if c.Type != ChangeFeature {
				t.Errorf("streaming support type = %s, want feature", c.Type)
			}
		case c.Description == "resolve nil pointer dereference":
			if c.Type != ChangeFix {
				t.Errorf("nil pointer type = %s, want fix", c.Type)
			}
		case c.Description == "protocol v2 uses new message format":
			if c.Type != ChangeProtocolChange {
				t.Errorf("protocol change type = %s, want protocol-change", c.Type)
			}
		case c.Description == "extract auth middleware":
			if c.Type != ChangeRefactor {
				t.Errorf("auth middleware type = %s, want refactor", c.Type)
			}
		case c.Description == "update API reference":
			if c.Type != ChangeDocs {
				t.Errorf("API reference type = %s, want docs", c.Type)
			}
		}
	}
}

func TestAnalyzeRelease_WithScopedCommits(t *testing.T) {
	release := Release{
		Tag: "v2.0.0",
		Body: `feat(auth): add OAuth2 support
fix(ui): correct button alignment
perf(db): optimize query plan`,
		PublishedAt: time.Now(),
	}

	a := &ChangelogAnalyzer{}
	changes := a.AnalyzeRelease(release)

	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3", len(changes))
	}

	for _, c := range changes {
		switch {
		case c.Description == "add OAuth2 support":
			if c.Type != ChangeFeature {
				t.Errorf("OAuth2 type = %s, want feature", c.Type)
			}
		case c.Description == "correct button alignment":
			if c.Type != ChangeFix {
				t.Errorf("button type = %s, want fix", c.Type)
			}
		case c.Description == "optimize query plan":
			if c.Type != ChangeRefactor {
				t.Errorf("query plan type = %s, want refactor", c.Type)
			}
		}
	}
}

func TestAnalyzeRelease_FreeformText(t *testing.T) {
	release := Release{
		Tag: "v1.0.0",
		Body: `This release contains miscellaneous improvements.
Nothing specific to classify here.`,
		PublishedAt: time.Now(),
	}

	a := &ChangelogAnalyzer{}
	changes := a.AnalyzeRelease(release)

	for _, c := range changes {
		if c.Type != ChangeOther {
			t.Errorf("freeform line type = %s, want other", c.Type)
		}
	}
}

func TestAnalyzeRelease_EmptyBody(t *testing.T) {
	release := Release{
		Tag:  "v1.0.0",
		Body: "",
	}

	a := &ChangelogAnalyzer{}
	changes := a.AnalyzeRelease(release)

	if len(changes) != 0 {
		t.Errorf("got %d changes from empty body, want 0", len(changes))
	}
}

func TestAnalyzeRelease_BulletedList(t *testing.T) {
	release := Release{
		Tag: "v2.0.0",
		Body: `- feat: add new dashboard
- fix: resolve memory leak
- docs: update changelog format`,
		PublishedAt: time.Now(),
	}

	a := &ChangelogAnalyzer{}
	changes := a.AnalyzeRelease(release)

	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3", len(changes))
	}

	expected := map[string]ChangeType{
		"add new dashboard":       ChangeFeature,
		"resolve memory leak":     ChangeFix,
		"update changelog format": ChangeDocs,
	}

	for _, c := range changes {
		wantType, ok := expected[c.Description]
		if !ok {
			t.Errorf("unexpected description: %q", c.Description)
			continue
		}
		if c.Type != wantType {
			t.Errorf("%q type = %s, want %s", c.Description, c.Type, wantType)
		}
	}
}

func TestDiffReleases(t *testing.T) {
	from := Release{
		Tag:  "v1.0.0",
		Body: "feat: old feature\nfix: old fix",
	}
	to := Release{
		Tag:  "v2.0.0",
		Body: "feat: old feature\nfeat: new feature\nfix: new fix",
	}

	a := &ChangelogAnalyzer{}
	diff := a.DiffReleases(from, to)

	if len(diff) != 2 {
		t.Fatalf("got %d diff changes, want 2", len(diff))
	}

	for _, c := range diff {
		switch c.Description {
		case "new feature":
			if c.Type != ChangeFeature {
				t.Errorf("new feature type = %s, want feature", c.Type)
			}
		case "new fix":
			if c.Type != ChangeFix {
				t.Errorf("new fix type = %s, want fix", c.Type)
			}
		default:
			t.Errorf("unexpected diff change: %q", c.Description)
		}
	}
}

func TestDiffReleases_NoNewChanges(t *testing.T) {
	from := Release{
		Tag:  "v1.0.0",
		Body: "feat: shared feature",
	}
	to := Release{
		Tag:  "v2.0.0",
		Body: "feat: shared feature",
	}

	a := &ChangelogAnalyzer{}
	diff := a.DiffReleases(from, to)

	if len(diff) != 0 {
		t.Errorf("got %d diff changes, want 0", len(diff))
	}
}

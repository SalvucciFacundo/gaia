package core

import (
	"testing"
)

// TestDetectSDDTrigger_KeywordFeature verifies "feature" triggers SDD.
func TestDetectSDDTrigger_KeywordFeature(t *testing.T) {
	tr := DetectSDDTrigger("implement a new feature for user authentication")
	if !tr.ShouldSDD {
		t.Error("'feature' keyword should trigger SDD")
	}
	if tr.ForceDirect {
		t.Error("should not be force-direct")
	}
	if tr.Reason == "" {
		t.Error("reason should not be empty")
	}
}

// TestDetectSDDTrigger_KeywordImplement verifies "implement" triggers SDD.
func TestDetectSDDTrigger_KeywordImplement(t *testing.T) {
	tr := DetectSDDTrigger("implement the login endpoint")
	if !tr.ShouldSDD {
		t.Error("'implement' keyword should trigger SDD")
	}
}

// TestDetectSDDTrigger_KeywordRefactor verifies "refactor" triggers SDD.
func TestDetectSDDTrigger_KeywordRefactor(t *testing.T) {
	tr := DetectSDDTrigger("refactor the database layer to use connection pooling")
	if !tr.ShouldSDD {
		t.Error("'refactor' keyword should trigger SDD")
	}
}

// TestDetectSDDTrigger_KeywordAdd verifies "add" triggers SDD.
func TestDetectSDDTrigger_KeywordAdd(t *testing.T) {
	tr := DetectSDDTrigger("add rate limiting to the API gateway")
	if !tr.ShouldSDD {
		t.Error("'add' keyword should trigger SDD")
	}
}

// TestDetectSDDTrigger_KeywordCreate verifies "create" triggers SDD.
func TestDetectSDDTrigger_KeywordCreate(t *testing.T) {
	tr := DetectSDDTrigger("create a new microservice for payments")
	if !tr.ShouldSDD {
		t.Error("'create' keyword should trigger SDD")
	}
}

// TestDetectSDDTrigger_KeywordArchitecture verifies "architecture" triggers.
func TestDetectSDDTrigger_KeywordArchitecture(t *testing.T) {
	tr := DetectSDDTrigger("redesign the architecture to use event sourcing")
	if !tr.ShouldSDD {
		t.Error("'architecture' keyword should trigger SDD")
	}
}

// TestDetectSDDTrigger_KeywordNewModule verifies multi-word keywords work.
func TestDetectSDDTrigger_KeywordNewModule(t *testing.T) {
	tr := DetectSDDTrigger("create a new module for report generation")
	if !tr.ShouldSDD {
		t.Error("'new module' keyword should trigger SDD")
	}
}

// TestDetectSDDTrigger_NoKeyword verifies normal messages don't trigger.
func TestDetectSDDTrigger_NoKeyword(t *testing.T) {
	tests := []string{
		"how do I check git status?",
		"what does this error mean?",
		"show me the current logs",
		"help me understand this code",
	}

	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			tr := DetectSDDTrigger(msg)
			if tr.ShouldSDD {
				t.Errorf("message %q should not trigger SDD", msg)
			}
		})
	}
}

// TestDetectSDDTrigger_DirectOverride verifies /direct bypasses SDD.
func TestDetectSDDTrigger_DirectOverride(t *testing.T) {
	tr := DetectSDDTrigger("/direct implement the new feature")

	if tr.ShouldSDD {
		t.Error("/direct should bypass SDD regardless of keywords")
	}
	if !tr.ForceDirect {
		t.Error("ForceDirect should be true for /direct")
	}
}

// TestDetectSDDTrigger_SDDOverride verifies /sdd forces SDD.
func TestDetectSDDTrigger_SDDOverride(t *testing.T) {
	// Even without keywords, /sdd should trigger
	tr := DetectSDDTrigger("/sdd show me the logs")

	if !tr.ShouldSDD {
		t.Error("/sdd should force SDD even without keywords")
	}
	if !tr.ForceSDD {
		t.Error("ForceSDD should be true for /sdd")
	}
}

// TestDetectSDDTrigger_CaseInsensitive verifies keyword matching is case-insensitive.
func TestDetectSDDTrigger_CaseInsensitive(t *testing.T) {
	tr := DetectSDDTrigger("IMPLEMENT the new FEATURE")
	if !tr.ShouldSDD {
		t.Error("case-insensitive keyword matching should trigger SDD")
	}
}

// TestDetectSDDTrigger_EmptyInput verifies empty input doesn't trigger.
func TestDetectSDDTrigger_EmptyInput(t *testing.T) {
	tr := DetectSDDTrigger("")
	if tr.ShouldSDD {
		t.Error("empty input should not trigger SDD")
	}
	if tr.ForceDirect {
		t.Error("empty input should not be force-direct")
	}
}

// TestDetectSDDTrigger_ReasonHasContent verifies reason is informative.
func TestDetectSDDTrigger_ReasonHasContent(t *testing.T) {
	tr := DetectSDDTrigger("implement user auth")
	if tr.Reason == "" {
		t.Error("reason should not be empty for triggered SDD")
	}
	if !tr.ShouldSDD {
		t.Error("should detect keyword")
	}

	tr2 := DetectSDDTrigger("/direct whatever")
	if tr2.Reason == "" {
		t.Error("reason should not be empty for /direct")
	}

	tr3 := DetectSDDTrigger("hello world")
	if tr3.Reason == "" {
		t.Error("reason should not be empty even for no trigger")
	}
}

// TestDetectSDDTrigger_Substring_Match verifies partial keyword matching works.
func TestDetectSDDTrigger_SubstringMatch(t *testing.T) {
	tr := DetectSDDTrigger("we should refactor that")
	if !tr.ShouldSDD {
		t.Error("'refactor' as substring should trigger SDD")
	}
}

// TestDetectSDDTrigger_KeywordsList verifies all keywords in the list trigger.
func TestDetectSDDTrigger_KeywordsList(t *testing.T) {
	for _, kw := range SDDKeywords {
		t.Run(kw, func(t *testing.T) {
			tr := DetectSDDTrigger("please " + kw + " something new")
			if !tr.ShouldSDD {
				t.Errorf("keyword %q should trigger SDD", kw)
			}
		})
	}
}

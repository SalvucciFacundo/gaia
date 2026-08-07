package core

import (
	"testing"
)

// TestDetectSDDTrigger_GenericKeywordsNotSDD verifies simple action words do NOT trigger SDD.
func TestDetectSDDTrigger_GenericKeywordsNotSDD(t *testing.T) {
	genericMessages := []string{
		"add a comment to line 50",
		"create a test file for login",
		"build the binary",
		"implement a quick fix",
		"refactor this helper function",
	}

	for _, msg := range genericMessages {
		t.Run(msg, func(t *testing.T) {
			tr := DetectSDDTrigger(msg)
			if tr.ShouldSDD {
				t.Errorf("generic message %q should NOT trigger SDD pipeline", msg)
			}
		})
	}
}

// TestDetectSDDTrigger_ArchitecturalKeywordsTriggerSDD verifies breaking/architectural change phrases trigger SDD.
func TestDetectSDDTrigger_ArchitecturalKeywordsTriggerSDD(t *testing.T) {
	archMessages := []string{
		"this change causes a breaking change in auth",
		"we need an architectural redesign of the event bus",
		"perform a database migration for the user table",
		"requerimos un rediseño de arquitectura completo",
	}

	for _, msg := range archMessages {
		t.Run(msg, func(t *testing.T) {
			tr := DetectSDDTrigger(msg)
			if !tr.ShouldSDD {
				t.Errorf("architectural message %q SHOULD trigger SDD pipeline", msg)
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
	tr := DetectSDDTrigger("this involves a BREAKING CHANGE in auth")
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
	tr := DetectSDDTrigger("this causes a breaking change")
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
	tr := DetectSDDTrigger("we need a system migration for this service")
	if !tr.ShouldSDD {
		t.Error("'system migration' as substring should trigger SDD")
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

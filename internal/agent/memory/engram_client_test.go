package memory

import (
	"strings"
	"testing"
)

func TestFormatCurrentProjectArgs(t *testing.T) {
	args := FormatCurrentProjectArgs("/some/path")
	if args["directory"] != "/some/path" {
		t.Fatalf("expected directory '/some/path', got %v", args["directory"])
	}

	emptyArgs := FormatCurrentProjectArgs("")
	if len(emptyArgs) != 0 {
		t.Fatalf("expected empty args for empty directory, got %v", emptyArgs)
	}
}

func TestFormatSaveArgs(t *testing.T) {
	helper := NewEngramProtocolHelper("gaia", "sess-abc")

	f := false
	args := helper.FormatSaveArgs(SaveObservationInput{
		Title:         "Fixed connection bug",
		Type:          TypeBugfix,
		Content:       "Resolved nil pointer on reconnect",
		TopicKey:      "gaia/bugfix/reconnect",
		CapturePrompt: &f,
	})

	if args["title"] != "Fixed connection bug" {
		t.Errorf("expected title 'Fixed connection bug', got %v", args["title"])
	}
	if args["type"] != "bugfix" {
		t.Errorf("expected type 'bugfix', got %v", args["type"])
	}
	if args["scope"] != "project" {
		t.Errorf("expected default scope 'project', got %v", args["scope"])
	}
	if args["session_id"] != "sess-abc" {
		t.Errorf("expected session_id 'sess-abc', got %v", args["session_id"])
	}
	if args["capture_prompt"] != false {
		t.Errorf("expected capture_prompt false, got %v", args["capture_prompt"])
	}
}

func TestFormatJudgeArgs(t *testing.T) {
	args := FormatJudgeArgs("judge-456", "supersedes", "Newer pattern replaces old pattern")
	if args["judgment_id"] != "judge-456" {
		t.Errorf("unexpected judgment_id: %v", args["judgment_id"])
	}
	if args["relation"] != "supersedes" {
		t.Errorf("unexpected relation: %v", args["relation"])
	}
	if args["evidence"] != "Newer pattern replaces old pattern" {
		t.Errorf("unexpected evidence: %v", args["evidence"])
	}
}

func TestBuildSessionSummaryText(t *testing.T) {
	input := SessionSummaryInput{
		Goal:         "Modernize GAIA to ODD and Engram protocol",
		Instructions: "Strict tests, no regressions",
		Discoveries:  []string{"Engram requires session identity"},
		Accomplished: []string{"Implemented Phase 1 memory helpers"},
		NextSteps:    []string{"Implement Phase 2 ODD router"},
		RelevantFiles: map[string]string{
			"internal/agent/memory/engram_client.go": "Engram protocol client helpers",
		},
	}

	text := BuildSessionSummaryText(input)

	if !strings.Contains(text, "## Goal\nModernize GAIA to ODD and Engram protocol") {
		t.Error("missing or malformed Goal section")
	}
	if !strings.Contains(text, "## Instructions\nStrict tests, no regressions") {
		t.Error("missing or malformed Instructions section")
	}
	if !strings.Contains(text, "## Discoveries\n- Engram requires session identity") {
		t.Error("missing or malformed Discoveries section")
	}
	if !strings.Contains(text, "## Accomplished\n- Implemented Phase 1 memory helpers") {
		t.Error("missing or malformed Accomplished section")
	}
	if !strings.Contains(text, "## Next Steps\n- Implement Phase 2 ODD router") {
		t.Error("missing or malformed Next Steps section")
	}
	if !strings.Contains(text, "## Relevant Files\n- internal/agent/memory/engram_client.go — Engram protocol client helpers") {
		t.Error("missing or malformed Relevant Files section")
	}
}

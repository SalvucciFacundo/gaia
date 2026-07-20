package output

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecResult_FormatJSON_Success(t *testing.T) {
	r := ExecResult{
		Status:    "success",
		Result:    "Task completed successfully.",
		Artifacts: []string{"file1.go", "file2.go"},
		Risks:     []string{"risk-1"},
	}

	raw, err := r.FormatJSON()
	if err != nil {
		t.Fatalf("FormatJSON returned error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["status"] != "success" {
		t.Errorf("expected status 'success', got %v", parsed["status"])
	}
	if parsed["result"] != "Task completed successfully." {
		t.Errorf("unexpected result: %v", parsed["result"])
	}

	artifacts, ok := parsed["artifacts"].([]interface{})
	if !ok {
		t.Fatal("artifacts is not an array")
	}
	if len(artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(artifacts))
	}
}

func TestExecResult_FormatJSON_Error(t *testing.T) {
	r := ExecResult{
		Status: "error",
		Result: "LLM timeout after 30s",
	}

	raw, err := r.FormatJSON()
	if err != nil {
		t.Fatalf("FormatJSON returned error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["status"] != "error" {
		t.Errorf("expected status 'error', got %v", parsed["status"])
	}
}

func TestExecResult_FormatJSON_EmptyArtifacts(t *testing.T) {
	r := ExecResult{
		Status: "success",
		Result: "ok",
	}

	raw, err := r.FormatJSON()
	if err != nil {
		t.Fatalf("FormatJSON returned error: %v", err)
	}

	// Empty artifacts marshal as null (nil slice) or [] (initialized).
	// Both are valid JSON; omitting the field via omitempty for risks is what matters.
	if !strings.Contains(raw, `"artifacts"`) {
		t.Errorf("expected artifacts field, got: %s", raw)
	}

	// Risks should be omitted when empty (omitempty tag).
	if strings.Contains(raw, `"risks"`) {
		t.Errorf("empty risks should be omitted, got: %s", raw)
	}
}

func TestExecResult_FormatText_Quiet(t *testing.T) {
	r := ExecResult{
		Status:    "success",
		Result:    "just the result",
		Artifacts: []string{"a", "b"},
		Risks:     []string{"r1"},
	}

	out := r.FormatText(true, false)
	if out != "just the result" {
		t.Errorf("quiet mode should return only result, got: %s", out)
	}
}

func TestExecResult_FormatText_Normal(t *testing.T) {
	r := ExecResult{
		Status: "success",
		Result: "All good.",
	}

	out := r.FormatText(false, false)
	if out != "All good." {
		t.Errorf("normal success should return result only, got: %s", out)
	}

	r2 := ExecResult{
		Status: "error",
		Result: "something broke",
	}

	out2 := r2.FormatText(false, false)
	if !strings.Contains(out2, "error") {
		t.Errorf("normal error should include status, got: %s", out2)
	}
	if !strings.Contains(out2, "something broke") {
		t.Errorf("normal error should include result, got: %s", out2)
	}
}

func TestExecResult_FormatText_Verbose(t *testing.T) {
	r := ExecResult{
		Status:    "success",
		Result:    "Task completed.",
		Artifacts: []string{"src/main.go", "src/test.go"},
		Risks:     []string{"Potential API break", "Untested edge case"},
	}

	out := r.FormatText(false, true)

	if !strings.Contains(out, "Status: success") {
		t.Error("verbose should include status")
	}
	if !strings.Contains(out, "Task completed.") {
		t.Error("verbose should include result")
	}
	if !strings.Contains(out, "Artifacts:") {
		t.Error("verbose should include artifacts section")
	}
	if !strings.Contains(out, "src/main.go") {
		t.Error("verbose should list artifacts")
	}
	if !strings.Contains(out, "Risks:") {
		t.Error("verbose should include risks section")
	}
	if !strings.Contains(out, "Potential API break") {
		t.Error("verbose should list risks")
	}
}

func TestExecResult_FormatText_VerboseNoRisks(t *testing.T) {
	r := ExecResult{
		Status:    "success",
		Result:    "done",
		Artifacts: []string{"file.go"},
	}

	out := r.FormatText(false, true)

	if strings.Contains(out, "Risks:") {
		t.Error("verbose should not show Risks section when empty")
	}
}

func TestExecResult_FormatText_VerboseNoArtifacts(t *testing.T) {
	r := ExecResult{
		Status: "success",
		Result: "done",
	}

	out := r.FormatText(false, true)

	if strings.Contains(out, "Artifacts:") {
		t.Error("verbose should not show Artifacts section when empty")
	}
}

func TestNewSuccessResult_InitializedSlices(t *testing.T) {
	r := NewSuccessResult("ok", nil, nil)
	if r.Artifacts == nil {
		t.Error("NewSuccessResult should initialize Artifacts to non-nil")
	}
	if len(r.Artifacts) != 0 {
		t.Errorf("expected empty artifacts, got %v", r.Artifacts)
	}

	raw, err := r.FormatJSON()
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}
	if !strings.Contains(raw, `"artifacts":[]`) {
		t.Errorf("expected artifacts:[], got: %s", raw)
	}
}

func TestNewErrorResult_InitializedSlices(t *testing.T) {
	r := NewErrorResult("something failed")
	if r.Status != "error" {
		t.Errorf("expected status error, got %s", r.Status)
	}
	if r.Result != "something failed" {
		t.Errorf("expected result, got %s", r.Result)
	}
	if r.Artifacts == nil {
		t.Error("NewErrorResult should initialize Artifacts to non-nil")
	}
	if len(r.Artifacts) != 0 {
		t.Errorf("expected empty artifacts, got %v", r.Artifacts)
	}
}

func TestNewSuccessResult_WithArtifacts(t *testing.T) {
	r := NewSuccessResult("done", []string{"f1.go", "f2.go"}, []string{"r1"})
	if len(r.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(r.Artifacts))
	}
	if len(r.Risks) != 1 {
		t.Errorf("expected 1 risk, got %d", len(r.Risks))
	}
}

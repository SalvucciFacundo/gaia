package output

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExecResult is the structured output from a headless execution.
type ExecResult struct {
	Status    string   `json:"status"`
	Result    string   `json:"result"`
	Artifacts []string `json:"artifacts"`
	Risks     []string `json:"risks,omitempty"`
}

// NewSuccessResult creates a result with initialized slices.
func NewSuccessResult(result string, artifacts, risks []string) *ExecResult {
	r := &ExecResult{
		Status:    "success",
		Result:    result,
		Artifacts: artifacts,
		Risks:     risks,
	}
	if r.Artifacts == nil {
		r.Artifacts = []string{}
	}
	return r
}

// NewErrorResult creates an error result with initialized slices.
func NewErrorResult(result string) *ExecResult {
	return &ExecResult{
		Status:    "error",
		Result:    result,
		Artifacts: []string{},
	}
}

// FormatJSON returns the JSON representation of the result.
func (r *ExecResult) FormatJSON() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	return string(data), nil
}

// FormatText returns a human-readable representation of the result.
// When quiet is true, only the result content is returned (no framing).
// When verbose is true, all details including artifacts and risks are shown.
// Normal mode shows the result with a simple status prefix.
func (r *ExecResult) FormatText(quiet, verbose bool) string {
	if quiet {
		return r.Result
	}

	var sb strings.Builder

	if verbose {
		sb.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
		sb.WriteString(strings.Repeat("-", 40) + "\n")
		sb.WriteString(r.Result)
		sb.WriteString("\n" + strings.Repeat("-", 40))

		if len(r.Artifacts) > 0 {
			sb.WriteString("\n\nArtifacts:")
			for _, a := range r.Artifacts {
				sb.WriteString(fmt.Sprintf("\n  - %s", a))
			}
		}

		if len(r.Risks) > 0 {
			sb.WriteString("\n\nRisks:")
			for _, risk := range r.Risks {
				sb.WriteString(fmt.Sprintf("\n  - %s", risk))
			}
		}
	} else {
		// Normal mode: just status + result
		if r.Status == "error" {
			sb.WriteString(fmt.Sprintf("[%s] %s", r.Status, r.Result))
		} else {
			sb.WriteString(r.Result)
		}
	}

	return sb.String()
}

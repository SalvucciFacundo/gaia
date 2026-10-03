package blocking

import (
	"strings"
	"testing"

	"gaia/internal/core/domain"
)

func sampleEnvelope() domain.ChoiceEnvelope {
	return domain.ChoiceEnvelope{
		WhyRequired: "Subagent requested edit authority over core packages.",
		Questions: []domain.ChoiceQuestion{
			{
				Header:   "Permission Elevation",
				Question: "How do you want to handle this request?",
				Options: []domain.ChoiceOption{
					{Token: "grant_once", Label: "Grant Once", Description: "Allow this specific edit operation only."},
					{Token: "grant_session", Label: "Grant for Session", Description: "Allow edits to these packages until session ends."},
					{Token: "deny", Label: "Deny?", Description: "Block the edit and continue without modifications."},
				},
			},
		},
	}
}

func sampleMultiSelectEnvelope() domain.ChoiceEnvelope {
	return domain.ChoiceEnvelope{
		WhyRequired: "Select packages to audit",
		Questions: []domain.ChoiceQuestion{
			{
				Header:        "Package Selection",
				Question:      "Which packages should be reviewed?",
				IsMultiSelect: true,
				Options: []domain.ChoiceOption{
					{Token: "core", Label: "Core Engine"},
					{Token: "adapters", Label: "Adapters Layer"},
					{Token: "agent", Label: "Subagent System"},
				},
			},
		},
	}
}

func TestFormatLosslessEnvelope(t *testing.T) {
	env := sampleEnvelope()
	formatted := FormatLosslessEnvelope(env)

	if !strings.Contains(formatted, "ACTION REQUIRED") {
		t.Errorf("expected header in formatted envelope")
	}
	if !strings.Contains(formatted, "Permission Elevation") {
		t.Errorf("expected question header")
	}
	if !strings.Contains(formatted, "[1] **Grant Once** — Allow this specific edit operation only.") {
		t.Errorf("expected option 1 with description")
	}
	if !strings.Contains(formatted, "[3] **Deny?**") {
		t.Errorf("expected option 3")
	}
}

func TestValidateChoiceResponse(t *testing.T) {
	q := sampleEnvelope().Questions[0]

	tests := []struct {
		input       string
		wantToken   string
		wantMeta    bool
		wantErr     bool
	}{
		// Exact numerals
		{input: "1", wantToken: "grant_once"},
		{input: "2", wantToken: "grant_session"},
		{input: "3", wantToken: "deny"},

		// Ordinal aliases
		{input: "first", wantToken: "grant_once"},
		{input: "primero", wantToken: "grant_once"},
		{input: "la 1", wantToken: "grant_once"},
		{input: "opción 2", wantToken: "grant_session"},
		{input: "opcion 3", wantToken: "deny"},

		// Rationale with "porque" (must NOT be treated as meta question!)
		{input: "1 porque es necesario", wantToken: "grant_once"},
		{input: "opción 2 porque confío en el agente", wantToken: "grant_session"},

		// Exact label and token
		{input: "Grant Once", wantToken: "grant_once"},
		{input: "grant_once", wantToken: "grant_once"},
		{input: "Deny?", wantToken: "deny"}, // Matches option label ending with '?'

		// Case-insensitive
		{input: "grant for session", wantToken: "grant_session"},

		// Meta-questions
		{input: "por qué me pide permiso?", wantMeta: true},
		{input: "qué significa grant for session?", wantMeta: true},
		{input: "what happens if I deny?", wantMeta: true},

		// Strict rejection of loose substring match (e.g. "e")
		{input: "e", wantErr: true},
		{input: "grant", wantErr: true}, // ambiguous / incomplete

		// Invalid options
		{input: "4", wantErr: true},
		{input: "algo inventado", wantErr: true},
		{input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res, err := ValidateChoiceResponse(tt.input, q)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if tt.wantMeta {
				if !res.IsMeta {
					t.Errorf("expected IsMeta=true for %q", tt.input)
				}
				return
			}
			if len(res.MatchedTokens) != 1 || res.MatchedTokens[0] != tt.wantToken {
				t.Errorf("input %q: got token %v, want %q", tt.input, res.MatchedTokens, tt.wantToken)
			}
		})
	}
}

func TestValidateChoiceResponse_MultiSelect(t *testing.T) {
	q := sampleMultiSelectEnvelope().Questions[0]

	tests := []struct {
		input      string
		wantTokens []string
		wantErr    bool
	}{
		{
			input:      "1, 2",
			wantTokens: []string{"core", "adapters"},
		},
		{
			input:      "core, agent",
			wantTokens: []string{"core", "agent"},
		},
		{
			input:      "opción 1, 3",
			wantTokens: []string{"core", "agent"},
		},
		{
			input:   "1, invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res, err := ValidateChoiceResponse(tt.input, q)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if len(res.MatchedTokens) != len(tt.wantTokens) {
				t.Fatalf("got %v, want %v", res.MatchedTokens, tt.wantTokens)
			}
			for i, tok := range res.MatchedTokens {
				if tok != tt.wantTokens[i] {
					t.Errorf("index %d: got %s, want %s", i, tok, tt.wantTokens[i])
				}
			}
		})
	}
}

package blocking

import (
	"fmt"
	"strconv"
	"strings"

	"gaia/internal/core/domain"
)

// FormatLosslessEnvelope formats a ChoiceEnvelope for terminal or markdown display,
// preserving the complete choice envelope without summarization, omission, or truncation.
func FormatLosslessEnvelope(env domain.ChoiceEnvelope) string {
	var sb strings.Builder

	sb.WriteString("⚠️  **ACTION REQUIRED: Blocking Choice Envelope**\n\n")
	if env.WhyRequired != "" {
		sb.WriteString(fmt.Sprintf("**Why input is required:** %s\n\n", env.WhyRequired))
	}

	for i, q := range env.Questions {
		if q.Header != "" {
			sb.WriteString(fmt.Sprintf("### %s\n", q.Header))
		}
		prefix := ""
		if len(env.Questions) > 1 {
			prefix = fmt.Sprintf("%d. ", i+1)
		}
		sb.WriteString(fmt.Sprintf("%s**%s**\n", prefix, q.Question))
		if q.IsMultiSelect {
			sb.WriteString("*(Select multiple options separated by comma)*\n")
		} else {
			sb.WriteString("*(Select exactly one option)*\n")
		}

		for idx, opt := range q.Options {
			num := idx + 1
			if opt.Description != "" {
				sb.WriteString(fmt.Sprintf("  [%d] **%s** — %s\n", num, opt.Label, opt.Description))
			} else {
				sb.WriteString(fmt.Sprintf("  [%d] **%s**\n", num, opt.Label))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Respond with your choice by number (e.g. `1`, `opción 1`, `first`) or exact label.\n")
	sb.WriteString("Or ask a question about what a choice means without making a decision.\n")

	return strings.TrimSpace(sb.String())
}

// MatchResult represents the outcome of validating user input against a ChoiceEnvelope.
type MatchResult struct {
	MatchedTokens []string // Resolved canonical token(s)
	IsMeta        bool     // True if input is a meta-question about the block itself
	Explanation   string   // Explanation if IsMeta is true
}

// MetaQuestionKeywords help identify if user is asking about the block rather than choosing.
// NOTE: "porque" is omitted because in Spanish it indicates rationale (e.g. "opcion 1 porque es seguro").
// Only interrogatives like "por qué" / "por que" are preserved.
var MetaQuestionKeywords = []string{
	"why", "qué significa", "que significa", "what does", "what happens",
	"qué pasa si", "que pasa si", "explain", "explicá", "explica",
	"por qué", "por que", "para qué", "para que", "diferencia entre",
}

// ValidateChoiceResponse validates raw user input against a ChoiceQuestion.
// It enforces:
// 1. Meta-question identification (returns IsMeta=true, no token).
// 2. Exact domain matching case-insensitively on labels or canonical tokens.
// 3. Ordinal aliases: 'N', 'la N', 'opción N', 'opcion N', and 'first' for index 1.
// 4. Exact 1 match requirement for single-select (or multiple valid matches for IsMultiSelect).
// 5. Strict matching contract: rejects loose character/substring fallbacks.
func ValidateChoiceResponse(input string, question domain.ChoiceQuestion) (*MatchResult, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("input cannot be empty")
	}

	lower := strings.ToLower(trimmed)

	// Check if this input matches an option label directly (even if it contains a '?')
	for _, opt := range question.Options {
		if strings.EqualFold(trimmed, opt.Label) || strings.EqualFold(trimmed, opt.Token) {
			return &MatchResult{MatchedTokens: []string{opt.Token}}, nil
		}
	}

	// Check if this is a meta-question about the prompt/choices
	isMeta := false
	for _, kw := range MetaQuestionKeywords {
		if strings.HasPrefix(lower, kw) || strings.Contains(lower, " "+kw) {
			isMeta = true
			break
		}
	}
	if !isMeta && strings.HasSuffix(lower, "?") {
		// Only treat trailing '?' as meta if it does not match an option index/alias
		isMeta = true
	}

	if isMeta {
		return &MatchResult{
			IsMeta:      true,
			Explanation: fmt.Sprintf("User asked an informational question about the choice: %q", trimmed),
		}, nil
	}

	// If multi-select is enabled, split by commas
	if question.IsMultiSelect && strings.Contains(trimmed, ",") {
		parts := strings.Split(trimmed, ",")
		var tokens []string
		seen := make(map[string]bool)

		for _, p := range parts {
			subRes, err := matchSingleOption(strings.TrimSpace(p), question)
			if err != nil {
				return nil, fmt.Errorf("in multi-select item %q: %w", strings.TrimSpace(p), err)
			}
			for _, tok := range subRes.MatchedTokens {
				if !seen[tok] {
					seen[tok] = true
					tokens = append(tokens, tok)
				}
			}
		}

		if len(tokens) == 0 {
			return nil, fmt.Errorf("no valid options selected in multi-select")
		}
		return &MatchResult{MatchedTokens: tokens}, nil
	}

	// Single item match
	return matchSingleOption(trimmed, question)
}

func matchSingleOption(trimmed string, question domain.ChoiceQuestion) (*MatchResult, error) {
	lower := strings.ToLower(trimmed)

	// Helper to resolve an index 1-based to option token
	resolveByIndex := func(idx int) (string, error) {
		if idx >= 1 && idx <= len(question.Options) {
			return question.Options[idx-1].Token, nil
		}
		return "", fmt.Errorf("index %d out of range (1-%d)", idx, len(question.Options))
	}

	// If user included rationale (e.g. "1 porque es necesario" or "opcion 1 ya que..."),
	// extract the leading token or ordinal phrase if separated by space.
	firstWord := strings.Fields(lower)[0]
	if num, err := strconv.Atoi(firstWord); err == nil {
		tok, err := resolveByIndex(num)
		if err == nil {
			return &MatchResult{MatchedTokens: []string{tok}}, nil
		}
	}

	// 1. Ordinal alias check: bare numeral N
	if num, err := strconv.Atoi(trimmed); err == nil {
		tok, err := resolveByIndex(num)
		if err != nil {
			return nil, err
		}
		return &MatchResult{MatchedTokens: []string{tok}}, nil
	}

	// 2. Ordinal alias check: 'first' -> 1
	if lower == "first" || lower == "primero" || lower == "primera" {
		tok, err := resolveByIndex(1)
		if err != nil {
			return nil, err
		}
		return &MatchResult{MatchedTokens: []string{tok}}, nil
	}

	// 3. Ordinal alias check: 'la N', 'el N', 'opción N', 'opcion N'
	prefixes := []string{"la ", "el ", "opción ", "opcion ", "option "}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			rest := strings.TrimSpace(lower[len(p):])
			// Extract first word of rest in case rationale followed
			restFields := strings.Fields(rest)
			if len(restFields) > 0 {
				if num, err := strconv.Atoi(restFields[0]); err == nil {
					tok, err := resolveByIndex(num)
					if err != nil {
						return nil, err
					}
					return &MatchResult{MatchedTokens: []string{tok}}, nil
				}
			}
		}
	}

	// 4. Exact match against label or token
	var matched []string
	for _, opt := range question.Options {
		if strings.EqualFold(lower, opt.Token) || strings.EqualFold(lower, opt.Label) {
			matched = append(matched, opt.Token)
		}
	}

	if len(matched) == 1 {
		return &MatchResult{MatchedTokens: matched}, nil
	}

	if len(matched) > 1 {
		return nil, fmt.Errorf("ambiguous choice: input %q matches multiple options (%v)", trimmed, matched)
	}

	return nil, fmt.Errorf("invalid response %q: does not match any allowed option in the domain", trimmed)
}

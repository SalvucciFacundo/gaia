package review

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

// Lens is the interface that all review lenses must implement.
type Lens interface {
	// Name returns the lens identifier (e.g., "review-risk").
	Name() string
	// Analyze examines the given file snapshots and returns classified findings.
	Analyze(ctx context.Context, files []FileSnapshot) ([]domain.ReviewFinding, error)
}

// LensLLM is the interface for LLM access during lens analysis.
// It abstracts away the specific LLM implementation so the review
// package does not depend on the agent infrastructure.
type LensLLM interface {
	Chat(ctx context.Context, systemPrompt, userMessage string) (string, error)
}

// LensRisk analyzes security, permissions, data exposure, and architecture.
type LensRisk struct {
	llm LensLLM
}

func NewLensRisk(llm LensLLM) *LensRisk { return &LensRisk{llm: llm} }
func (l *LensRisk) Name() string        { return "review-risk" }

func (l *LensRisk) Analyze(ctx context.Context, files []FileSnapshot) ([]domain.ReviewFinding, error) {
	prompt := l.buildSystemPrompt()
	userMsg := l.buildUserMessage(files)

	resp, err := l.llm.Chat(ctx, prompt, userMsg)
	if err != nil {
		return nil, fmt.Errorf("lens risk: %w", err)
	}
	return parseFindings(resp, "review-risk"), nil
}

func (l *LensRisk) buildSystemPrompt() string {
	return `You are the RISK lens reviewer. Analyze code for security and risk.
Evaluate:
1. Security vulnerabilities (injection, XSS, secrets, weak crypto)
2. Data integrity risks (nil pointers, race conditions, unchecked errors)
3. Deployment/rollback risks (breaking changes, missing migrations)
4. Architecture violations (circular deps, layer violations)

For each issue, output a finding line:
  FINDING: <severity> <file>:<line> <message> | SUGGESTION: <fix>

Severity: BLOCKER | WARNING | SUGGESTION`
}

func (l *LensRisk) buildUserMessage(files []FileSnapshot) string {
	return buildFileListMessage(files, "Analyze these files for RISK (security, data integrity, deployment):")
}

// LensResilience analyzes error handling, fallbacks, resource cleanup, and observability.
type LensResilience struct {
	llm LensLLM
}

func NewLensResilience(llm LensLLM) *LensResilience { return &LensResilience{llm: llm} }
func (l *LensResilience) Name() string              { return "review-resilience" }

func (l *LensResilience) Analyze(ctx context.Context, files []FileSnapshot) ([]domain.ReviewFinding, error) {
	resp, err := l.llm.Chat(ctx, l.buildSystemPrompt(), l.buildUserMessage(files))
	if err != nil {
		return nil, fmt.Errorf("lens resilience: %w", err)
	}
	return parseFindings(resp, "review-resilience"), nil
}

func (l *LensResilience) buildSystemPrompt() string {
	return `You are the RESILIENCE lens reviewer. Analyze code for failure handling.
Evaluate:
1. Error handling completeness (every error path covered, no swallowed errors)
2. Graceful degradation (no panics on unexpected input, safe defaults)
3. Resource cleanup (deferred closes, context cancellation, connection pools)
4. Observability (logging, metrics, tracing for error paths)

For each issue, output a finding line:
  FINDING: <severity> <file>:<line> <message> | SUGGESTION: <fix>

Severity: BLOCKER | WARNING | SUGGESTION`
}

func (l *LensResilience) buildUserMessage(files []FileSnapshot) string {
	return buildFileListMessage(files, "Analyze these files for RESILIENCE (error handling, cleanup, observability):")
}

// LensReadability analyzes naming, structure, documentation, and maintainability.
type LensReadability struct {
	llm LensLLM
}

func NewLensReadability(llm LensLLM) *LensReadability { return &LensReadability{llm: llm} }
func (l *LensReadability) Name() string               { return "review-readability" }

func (l *LensReadability) Analyze(ctx context.Context, files []FileSnapshot) ([]domain.ReviewFinding, error) {
	resp, err := l.llm.Chat(ctx, l.buildSystemPrompt(), l.buildUserMessage(files))
	if err != nil {
		return nil, fmt.Errorf("lens readability: %w", err)
	}
	return parseFindings(resp, "review-readability"), nil
}

func (l *LensReadability) buildSystemPrompt() string {
	return `You are the READABILITY lens reviewer. Analyze code for clarity and maintainability.
Evaluate:
1. Naming clarity (packages, functions, variables follow conventions)
2. Documentation quality (doc comments, READMEs, inline explanations)
3. Structure organization (single responsibility, reasonable file sizes)
4. Code style consistency (indentation, line length, pattern usage)

For each issue, output a finding line:
  FINDING: <severity> <file>:<line> <message> | SUGGESTION: <fix>

Severity: BLOCKER | WARNING | SUGGESTION`
}

func (l *LensReadability) buildUserMessage(files []FileSnapshot) string {
	return buildFileListMessage(files, "Analyze these files for READABILITY (naming, structure, documentation):")
}

// LensReliability analyzes test coverage, determinism, edge cases, and contract adherence.
type LensReliability struct {
	llm LensLLM
}

func NewLensReliability(llm LensLLM) *LensReliability { return &LensReliability{llm: llm} }
func (l *LensReliability) Name() string               { return "review-reliability" }

func (l *LensReliability) Analyze(ctx context.Context, files []FileSnapshot) ([]domain.ReviewFinding, error) {
	resp, err := l.llm.Chat(ctx, l.buildSystemPrompt(), l.buildUserMessage(files))
	if err != nil {
		return nil, fmt.Errorf("lens reliability: %w", err)
	}
	return parseFindings(resp, "review-reliability"), nil
}

func (l *LensReliability) buildSystemPrompt() string {
	return `You are the RELIABILITY lens reviewer. Analyze code for correctness and testability.
Evaluate:
1. Test coverage (unit tests, integration tests, edge cases, missing tests)
2. Contract adherence (interfaces match expectations, spec compliance)
3. Behavioral correctness (off-by-one, logic errors, stale assumptions, race conditions)
4. Determinism (non-deterministic outputs, time-dependent logic, random without seed)

For each issue, output a finding line:
  FINDING: <severity> <file>:<line> <message> | SUGGESTION: <fix>

Severity: BLOCKER | WARNING | SUGGESTION`
}

func (l *LensReliability) buildUserMessage(files []FileSnapshot) string {
	return buildFileListMessage(files, "Analyze these files for RELIABILITY (test coverage, correctness, determinism):")
}

// buildFileListMessage formats file snapshots into a user message for LLM analysis.
func buildFileListMessage(files []FileSnapshot, header string) string {
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", f.Path, f.Content))
	}
	return sb.String()
}

// parseFindings extracts ReviewFinding entries from LLM response text.
// It looks for lines starting with "FINDING:" and parses them.
func parseFindings(response, lens string) []domain.ReviewFinding {
	var findings []domain.ReviewFinding
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "FINDING:") {
			// Also try without FINDING prefix — LLMs sometimes omit it.
			if !strings.HasPrefix(trimmed, "BLOCKER:") &&
				!strings.HasPrefix(trimmed, "WARNING:") &&
				!strings.HasPrefix(trimmed, "SUGGESTION:") {
				continue
			}
		}

		// Remove the prefix.
		content := trimmed
		content = strings.TrimPrefix(content, "FINDING:")
		content = strings.TrimSpace(content)

		// Also handle severity-prefixed format: "BLOCKER: file:line message"
		severity := "SUGGESTION"
		for _, sev := range []string{"BLOCKER", "WARNING", "SUGGESTION"} {
			if strings.HasPrefix(content, sev+":") {
				severity = sev
				content = strings.TrimPrefix(content, sev+":")
				content = strings.TrimSpace(content)
				break
			}
		}

		// Parse file:line
		file := ""
		lineNum := 0
		message := content
		suggestion := ""

		// Check for "| SUGGESTION:" separator.
		if idx := strings.Index(content, "| SUGGESTION:"); idx >= 0 {
			suggestion = strings.TrimSpace(content[idx+len("| SUGGESTION:"):])
			message = strings.TrimSpace(content[:idx])
		}

		// Parse file:line prefix.
		parts := strings.SplitN(message, " ", 2)
		if len(parts) >= 1 {
			fileLine := parts[0]
			if colonIdx := strings.LastIndex(fileLine, ":"); colonIdx >= 0 {
				file = fileLine[:colonIdx]
				if n, err := fmt.Sscanf(fileLine[colonIdx+1:], "%d", &lineNum); err == nil && n == 1 {
					if len(parts) > 1 {
						message = parts[1]
					}
				} else {
					lineNum = 0
				}
			}
		}

		if message == "" {
			continue
		}

		findings = append(findings, domain.ReviewFinding{
			Lens:       lens,
			Severity:   severity,
			File:       file,
			Line:       lineNum,
			Message:    message,
			Suggestion: suggestion,
		})
	}
	return findings
}

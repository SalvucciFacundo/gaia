package learn

import (
	"context"
	"testing"
)

func TestExtractLearnings_KeyLearningsHeader(t *testing.T) {
	content := `Task completed successfully.

## Key Learnings
- SQLite tables require explicit foreign key constraints enabled on open.
- Context cancellation must be checked before calling LLM APIs.
- Memory topic keys must follow hierarchical patterns.

## Next Steps
Proceed to verify phase.`

	learnings := ExtractLearnings(content)
	if len(learnings) != 3 {
		t.Fatalf("expected 3 learnings, got %d: %v", len(learnings), learnings)
	}

	if learnings[0] != "SQLite tables require explicit foreign key constraints enabled on open." {
		t.Errorf("unexpected first learning: %q", learnings[0])
	}
}

func TestExtractLearnings_ObservationsHeader(t *testing.T) {
	content := `Status: success
ExecutiveSummary: Refactored module.
Observations:
- The parser handles trailing commas gracefully.
- Do not mutate global registry during request processing.
Risks: none`

	learnings := ExtractLearnings(content)
	if len(learnings) != 2 {
		t.Fatalf("expected 2 learnings, got %d: %v", len(learnings), learnings)
	}
}

func TestExtractLearnings_InlineAfterColon(t *testing.T) {
	content := `Learned: always use io.ReadAll with a size limiter.`
	learnings := ExtractLearnings(content)
	if len(learnings) != 1 {
		t.Fatalf("expected 1 learning, got %d: %v", len(learnings), learnings)
	}
	if learnings[0] != "always use io.ReadAll with a size limiter." {
		t.Errorf("unexpected learning: %q", learnings[0])
	}
}

func TestExtractLearnings_None(t *testing.T) {
	content := `Observations: none`
	learnings := ExtractLearnings(content)
	if len(learnings) != 0 {
		t.Errorf("expected 0 learnings for 'none', got %d: %v", len(learnings), learnings)
	}
}

func TestPassiveLearner_CaptureExtract(t *testing.T) {
	loop := NewLearningLoop(5)
	learner := NewPassiveLearner(loop)

	ctx := context.Background()
	output := `## Discoveries
- Token budget is capped at 400 lines.`

	insights := learner.CaptureExtract(ctx, "implementer", output)
	if len(insights) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(insights))
	}

	if loop.Count("implementer") != 1 {
		t.Errorf("expected execution count 1, got %d", loop.Count("implementer"))
	}
}

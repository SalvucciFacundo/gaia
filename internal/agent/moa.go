package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// moaRunner executes the Mixture-of-Agents fan-out for a subagent task.
// It fans out the same messages to multiple models in parallel, collects
// responses, and synthesizes them into a single response via the primary model.
type moaRunner struct {
	primary    ports.LLMProvider
	moaModels  []moaModelInstance // the extra models (primary excluded)
	synthesizer ports.LLMProvider  // defaults to primary
}

type moaModelInstance struct {
	Label    string
	Provider ports.LLMProvider
}

// newMoARunner creates a MoA runner from the task config and available providers.
func newMoARunner(primary ports.LLMProvider, moaProviders map[string]ports.LLMProvider, cfg domain.MoAConfig) (*moaRunner, error) {
	if !cfg.Enabled || len(cfg.Models) == 0 {
		return nil, nil // MoA disabled or no extra models
	}

	var models []moaModelInstance
	for _, m := range cfg.Models {
		prov, ok := moaProviders[m.Provider]
		if !ok {
			return nil, fmt.Errorf("moa: unknown provider %q for model %q", m.Provider, m.Model)
		}
		label := m.Label
		if label == "" {
			label = fmt.Sprintf("%s/%s", m.Provider, m.Model)
		}
		models = append(models, moaModelInstance{
			Label:    label,
			Provider: prov,
		})
	}

	if len(models) == 0 {
		return nil, nil
	}

	return &moaRunner{
		primary:    primary,
		moaModels:  models,
		synthesizer: primary,
	}, nil
}

// run executes the MoA fan-out:
// 1. Fans out the same messages to all extra models in parallel
// 2. Collects all responses (with timeout)
// 3. Synthesizes them into a single response via the primary model
func (m *moaRunner) run(ctx context.Context, messages []domain.Message) (*domain.Message, error) {
	// 1. Fan out to all MoA models in parallel
	type moaResult struct {
		Label string
		Msg   *domain.Message
		Err   error
	}

	results := make(chan moaResult, len(m.moaModels))
	var wg sync.WaitGroup

	for _, model := range m.moaModels {
		wg.Add(1)
		go func(model moaModelInstance) {
			defer wg.Done()
			// Timeout per model call (30s per model)
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			msg, err := model.Provider.Chat(callCtx, messages)
			results <- moaResult{Label: model.Label, Msg: msg, Err: err}
		}(model)
	}

	// Wait for all to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// 2. Collect responses
	var responses []moaResult
	for r := range results {
		if r.Err == nil && r.Msg != nil {
			responses = append(responses, r)
		}
	}

	if len(responses) == 0 {
		// All MoA models failed — fall back to primary only
		return nil, nil
	}

	// 3. Synthesize: build a prompt with all responses and ask the synthesizer to merge
	var sb strings.Builder
	sb.WriteString("The following are responses from different AI models for the same task.\n")
	sb.WriteString("Synthesize them into a single, coherent response that captures the best\n")
	sb.WriteString("insights from each. Resolve any contradictions and avoid repetition.\n\n")

	for i, r := range responses {
		sb.WriteString(fmt.Sprintf("=== Response %d (%s) ===\n%s\n\n", i+1, r.Label, r.Msg.Content))
	}

	sb.WriteString("Synthesized response:")

	synthMsg := &domain.Message{
		Role:    domain.RoleUser,
		Content: sb.String(),
	}

	synthResp, err := m.synthesizer.Chat(ctx, []domain.Message{{
		Role:    domain.RoleSystem,
		Content: "You are a synthesis model. Merge multiple AI responses into one coherent answer.",
	}, *synthMsg})
	if err != nil {
		// Synthesis failed — return the first MoA response as fallback
		return responses[0].Msg, nil
	}

	return synthResp, nil
}

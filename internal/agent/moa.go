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

// moaModelInstance represents an instantiated model provider candidate.
type moaModelInstance struct {
	Label    string
	Provider ports.LLMProvider
}

// MoARunner executes the Mixture-of-Agents fan-out for subagents or one-shot commands.
// It fans out the same messages to multiple models in parallel, collects
// responses, and synthesizes them into a single response via the primary model.
type MoARunner struct {
	primary     ports.LLMProvider
	moaModels   []moaModelInstance
	synthesizer ports.LLMProvider
	timeout     time.Duration
}

var _ ports.MoARunner = (*MoARunner)(nil)

// NewMoARunner creates a MoA runner from the task config and available providers.
func NewMoARunner(primary ports.LLMProvider, moaProviders map[string]ports.LLMProvider, cfg domain.MoAConfig) (*MoARunner, error) {
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

	return &MoARunner{
		primary:     primary,
		moaModels:   models,
		synthesizer: primary,
		timeout:     30 * time.Second,
	}, nil
}

// newMoARunner is an internal alias for backward compatibility.
func newMoARunner(primary ports.LLMProvider, moaProviders map[string]ports.LLMProvider, cfg domain.MoAConfig) (*MoARunner, error) {
	return NewMoARunner(primary, moaProviders, cfg)
}

// NewOneShotMoARunner creates a MoA runner using all available providers for a one-shot command.
func NewOneShotMoARunner(primary ports.LLMProvider, moaProviders map[string]ports.LLMProvider, timeout time.Duration) *MoARunner {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	var models []moaModelInstance
	for name, prov := range moaProviders {
		if prov != nil {
			models = append(models, moaModelInstance{
				Label:    name,
				Provider: prov,
			})
		}
	}

	return &MoARunner{
		primary:     primary,
		moaModels:   models,
		synthesizer: primary,
		timeout:     timeout,
	}
}

// Run executes the MoA fan-out:
// 1. Fans out the same messages to all extra models in parallel
// 2. Collects all responses (with timeout)
// 3. Synthesizes them into a single response via the primary model
func (m *MoARunner) Run(ctx context.Context, messages []domain.Message) (*domain.Message, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	if m == nil || len(m.moaModels) == 0 {
		return nil, nil
	}

	type moaResult struct {
		Label string
		Msg   *domain.Message
		Err   error
	}

	results := make(chan moaResult, len(m.moaModels))
	var wg sync.WaitGroup

	timeout := m.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	for _, model := range m.moaModels {
		wg.Add(1)
		go func(model moaModelInstance) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, timeout)
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
	responses := make([]moaResult, 0, len(m.moaModels))
	for r := range results {
		if r.Err == nil && r.Msg != nil {
			responses = append(responses, r)
		}
	}

	if len(responses) == 0 {
		return nil, nil
	}

	if len(responses) == 1 {
		msgCopy := *responses[0].Msg
		msgCopy.Role = domain.RoleAssistant
		return &msgCopy, nil
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
		msgCopy := *responses[0].Msg
		msgCopy.Role = domain.RoleAssistant
		return &msgCopy, nil
	}

	synthResp.Role = domain.RoleAssistant
	return synthResp, nil
}

// run is an internal alias for Run (backward compatibility).
func (m *MoARunner) run(ctx context.Context, messages []domain.Message) (*domain.Message, error) {
	return m.Run(ctx, messages)
}

// RunPrompt executes a one-shot MoA on a given user prompt.
func (m *MoARunner) RunPrompt(ctx context.Context, prompt string) (*domain.Message, error) {
	messages := []domain.Message{
		{Role: domain.RoleSystem, Content: "You are a helpful AI assistant."},
		{Role: domain.RoleUser, Content: prompt},
	}
	return m.Run(ctx, messages)
}

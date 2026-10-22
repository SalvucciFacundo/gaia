package overnight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// PreflightInterviewer analyzes the goal and conducts pre-flight alignment before autonomous runs.
type PreflightInterviewer struct {
	provider ports.LLMProvider
	storage  string
}

// NewPreflightInterviewer creates a new preflight interviewer.
func NewPreflightInterviewer(provider ports.LLMProvider, baseDir string) *PreflightInterviewer {
	if baseDir == "" {
		baseDir = ".gaia/overnight"
	}
	_ = os.MkdirAll(baseDir, 0755)
	return &PreflightInterviewer{
		provider: provider,
		storage:  baseDir,
	}
}

// AnalyzeGoal generates a targeted pre-flight questionnaire for the user.
func (p *PreflightInterviewer) AnalyzeGoal(ctx context.Context, goal, profile string) ([]PreflightQuestion, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if p.provider != nil {
		questions, err := p.generateQuestionsWithLLM(ctx, goal, profile)
		if err == nil && len(questions) > 0 {
			return questions, nil
		}
	}

	// Rule-based deterministic heuristic fallback
	return p.generateHeuristicQuestions(goal, profile), nil
}

func (p *PreflightInterviewer) generateQuestionsWithLLM(ctx context.Context, goal, profile string) ([]PreflightQuestion, error) {
	systemPrompt := `You are an elite autonomous system planner. Analyze the user's objective and generate 2 to 4 critical pre-flight alignment questions that must be answered BEFORE running overnight unattended.
Focus strictly on:
1. Architectural forks or library choices
2. Scope boundaries (what NOT to touch)
3. Handling migrations or breaking changes
4. Verification expectations (must tests pass before completion?)

Output JSON ONLY as an array of objects with keys:
- id: short identifier (e.g. "q_scope")
- question: clear concise question
- options: list of 2-3 concise option choices
- default_answer: the recommended default option
- criticality: "high" or "medium"
`
	userPrompt := fmt.Sprintf("Objective: %s\nActive Profile: %s", goal, profile)

	msgs := []domain.Message{
		{Role: domain.RoleSystem, Content: systemPrompt},
		{Role: domain.RoleUser, Content: userPrompt},
	}

	resp, err := p.provider.Chat(ctx, msgs)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(resp.Content)
	if idx := strings.Index(content, "["); idx != -1 {
		if endIdx := strings.LastIndex(content, "]"); endIdx != -1 && endIdx > idx {
			content = content[idx : endIdx+1]
		}
	}

	var questions []PreflightQuestion
	if err := json.Unmarshal([]byte(content), &questions); err != nil {
		return nil, err
	}

	return questions, nil
}

func (p *PreflightInterviewer) generateHeuristicQuestions(goal, profile string) []PreflightQuestion {
	lower := strings.ToLower(goal)
	var questions []PreflightQuestion

	// 1. Scope question
	questions = append(questions, PreflightQuestion{
		ID:            "q_scope",
		Question:      "¿El agente debe limitarse a modificar archivos existentes o puede crear nuevos módulos/paquetes?",
		Options:       []string{"Modificar y crear libremente", "Solo modificar archivos existentes", "Crear en directorio aislado"},
		DefaultAnswer: "Modificar y crear libremente",
		Criticality:   "high",
	})

	// 2. Breaking changes
	if strings.Contains(lower, "refactor") || strings.Contains(lower, "update") || strings.Contains(lower, "migra") {
		questions = append(questions, PreflightQuestion{
			ID:            "q_breaking",
			Question:      "¿Se permiten cambios en firmas de funciones públicas/APIs o se debe preservar retrocompatibilidad?",
			Options:       []string{"Preservar retrocompatibilidad 100%", "Permitir cambios si modernizan la API"},
			DefaultAnswer: "Preservar retrocompatibilidad 100%",
			Criticality:   "high",
		})
	}

	// 3. Test verification
	questions = append(questions, PreflightQuestion{
		ID:            "q_tests",
		Question:      "¿Los tests automatizados existentes deben pasar obligatoriamente al 100% para considerar exitosa la ejecución?",
		Options:       []string{"Sí, tests 100% pasando con -race", "Permitir tests pendientes si el código compila"},
		DefaultAnswer: "Sí, tests 100% pasando con -race",
		Criticality:   "medium",
	})

	return questions
}

// SavePlan writes the action plan to disk.
func (p *PreflightInterviewer) SavePlan(plan *ActionPlan) error {
	if plan.ID == "" {
		plan.ID = fmt.Sprintf("plan-%d", time.Now().Unix())
	}
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}

	path := filepath.Join(p.storage, fmt.Sprintf("%s.json", plan.ID))
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// LoadPlan reads an action plan by ID.
func (p *PreflightInterviewer) LoadPlan(planID string) (*ActionPlan, error) {
	path := filepath.Join(p.storage, fmt.Sprintf("%s.json", planID))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}

	var plan ActionPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}

	return &plan, nil
}

package ops

import (
	"context"
	"io"
	"strings"
	"testing"

	"gaia/internal/agent"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// --- Stubs ---

// opsStubProvider returns canned chat responses for ops subagent tests.
type opsStubProvider struct {
	resp    *domain.Message
	chatErr error
}

func (s *opsStubProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.resp, nil
}

func (s *opsStubProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		io.WriteString(pw, `{"content":"ok","done":true}`+"\n")
	}()
	return pr, nil
}

func (s *opsStubProvider) Tools() []domain.ToolDef { return nil }

// newOpsSpawner creates a Spawner with a stub provider and tool registry.
func newOpsSpawner() *agent.Spawner {
	cfg := agent.SpawnerConfig{
		Provider: &opsStubProvider{
			resp: &domain.Message{
				Role:    domain.RoleAssistant,
				Content: successOpsResponse(),
			},
		},
		Tools:  core.NewToolRegistry(),
		Budget: domain.DefaultBudget(),
	}
	return agent.NewSpawner(cfg, agent.NewRegistry())
}

// successOpsResponse returns a valid ops envelope string for parseOpsResult.
func successOpsResponse() string {
	return `Status: success
ExecutiveSummary: Operation completed successfully. All checks passed and the report has been generated.
Artifacts:
- ops-report
- structured-observations
NextRecommended: none
Risks: none
SkillResolution: none`
}

func newTestOpsTask(desc string) domain.SubagentTask {
	return domain.SubagentTask{
		ID:          "task-ops-01",
		Description: desc,
		Mode:        "plan",
	}
}

// --- Interface Contract Tests ---

func TestInterfaceContract_Reviewer(t *testing.T) {
	var _ agent.Subagent = NewReviewer(nil)
	sa := NewReviewer(nil)
	if sa.Name() != "reviewer" {
		t.Errorf("expected name 'reviewer', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(sa.Description(), "read-only") {
		t.Error("description should mention read-only")
	}
}

func TestInterfaceContract_Debugger(t *testing.T) {
	var _ agent.Subagent = NewDebugger(nil)
	sa := NewDebugger(nil)
	if sa.Name() != "debugger" {
		t.Errorf("expected name 'debugger', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(sa.Description(), "debug") && !strings.Contains(sa.Description(), "Debug") {
		t.Error("description should mention debugging")
	}
}

func TestInterfaceContract_Researcher(t *testing.T) {
	var _ agent.Subagent = NewResearcher(nil)
	sa := NewResearcher(nil)
	if sa.Name() != "researcher" {
		t.Errorf("expected name 'researcher', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestInterfaceContract_Learner(t *testing.T) {
	var _ agent.Subagent = NewLearner(nil)
	sa := NewLearner(nil)
	if sa.Name() != "learner" {
		t.Errorf("expected name 'learner', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(sa.Description(), "read-only") {
		t.Error("description should mention read-only")
	}
}

// --- Execute Tests ---

func TestReviewer_Execute(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewReviewer(spawner)
	task := newTestOpsTask("Review the agent package for security issues")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if result.NextRecommended != "none" {
		t.Errorf("expected next 'none', got %q", result.NextRecommended)
	}
}

func TestDebugger_Execute(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewDebugger(spawner)
	task := newTestOpsTask("Debug the nil pointer panic in kernel.go")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
	if len(result.Artifacts) == 0 {
		t.Error("expected at least one artifact")
	}
}

func TestResearcher_Execute(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewResearcher(spawner)
	task := newTestOpsTask("Research best practices for Go error handling patterns")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestLearner_Execute(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewLearner(spawner)
	task := newTestOpsTask("Analyze the codebase for skill creation opportunities")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
	if len(result.Artifacts) == 0 {
		t.Error("expected at least one artifact")
	}
}

// --- Tool Filter Tests ---

func TestReviewer_AllowedTools(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewReviewer(spawner)
	task := newTestOpsTask("Review")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}

	prompt := reviewerPrompt(task)
	if !strings.Contains(prompt, "file_read") {
		t.Error("reviewer prompt should include file_read")
	}
	if !strings.Contains(prompt, "READ-ONLY") {
		t.Error("reviewer prompt should mention READ-ONLY")
	}
	if strings.Contains(prompt, "file_write") {
		t.Error("reviewer prompt should NOT mention file_write")
	}
	if strings.Contains(prompt, "shell_exec") {
		t.Error("reviewer prompt should NOT mention shell_exec")
	}
}

func TestDebugger_AllowedTools(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewDebugger(spawner)
	task := newTestOpsTask("Debug")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}

	prompt := debuggerPrompt(task)
	if !strings.Contains(prompt, "file_write") {
		t.Error("debugger prompt should include file_write")
	}
	if !strings.Contains(prompt, "shell_exec") {
		t.Error("debugger prompt should include shell_exec")
	}
	if !strings.Contains(prompt, "FULL tool access") {
		t.Error("debugger prompt should mention FULL tool access")
	}
}

func TestResearcher_AllowedTools(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewResearcher(spawner)
	task := newTestOpsTask("Research")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}

	prompt := researcherPrompt(task)
	if !strings.Contains(prompt, "shell_exec") {
		t.Error("researcher prompt should include shell_exec (for curl/wget)")
	}
	if !strings.Contains(prompt, "curl") {
		t.Error("researcher prompt should mention curl")
	}
	if strings.Contains(prompt, "file_write") {
		t.Error("researcher prompt should NOT mention file_write")
	}
	if !strings.Contains(prompt, "CANNOT write") {
		t.Error("researcher prompt should mention CANNOT write files")
	}
}

func TestLearner_AllowedTools(t *testing.T) {
	spawner := newOpsSpawner()
	sa := NewLearner(spawner)
	task := newTestOpsTask("Analyze")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}

	prompt := learnerPrompt(task)
	if !strings.Contains(prompt, "file_read") {
		t.Error("learner prompt should include file_read")
	}
	if !strings.Contains(prompt, "READ-ONLY") {
		t.Error("learner prompt should mention READ-ONLY")
	}
	if strings.Contains(prompt, "file_write") {
		t.Error("learner prompt should NOT mention file_write")
	}
	if strings.Contains(prompt, "shell_exec") {
		t.Error("learner prompt should NOT mention shell_exec")
	}
}

// --- Prompt Content Tests ---

func TestReviewerPrompt_Content(t *testing.T) {
	task := newTestOpsTask("Review auth module")
	prompt := reviewerPrompt(task)

	if !strings.Contains(prompt, "Reviewer") {
		t.Error("prompt should mention Reviewer role")
	}
	if !strings.Contains(prompt, "FOUR-LENS") {
		t.Error("prompt should mention FOUR-LENS rubric")
	}
	if !strings.Contains(prompt, "RISK") {
		t.Error("prompt should mention RISK lens")
	}
	if !strings.Contains(prompt, "RESILIENCE") {
		t.Error("prompt should mention RESILIENCE lens")
	}
	if !strings.Contains(prompt, "READABILITY") {
		t.Error("prompt should mention READABILITY lens")
	}
	if !strings.Contains(prompt, "RELIABILITY") {
		t.Error("prompt should mention RELIABILITY lens")
	}
	if !strings.Contains(prompt, "REVIEW RECEIPT") {
		t.Error("prompt should mention REVIEW RECEIPT")
	}
	if !strings.Contains(prompt, "BOUNDED RECEIPT") {
		t.Error("prompt should mention BOUNDED RECEIPT section")
	}
}

func TestDebuggerPrompt_Content(t *testing.T) {
	task := newTestOpsTask("Debug nil pointer")
	prompt := debuggerPrompt(task)

	if !strings.Contains(prompt, "Debugger") {
		t.Error("prompt should mention Debugger role")
	}
	if !strings.Contains(prompt, "PHASE 1: ANALYZE") {
		t.Error("prompt should mention PHASE 1: ANALYZE")
	}
	if !strings.Contains(prompt, "PHASE 2: ROOT CAUSE") {
		t.Error("prompt should mention PHASE 2: ROOT CAUSE")
	}
	if !strings.Contains(prompt, "PHASE 3: FIX") {
		t.Error("prompt should mention PHASE 3: FIX")
	}
	if !strings.Contains(prompt, "PHASE 4: VERIFY") {
		t.Error("prompt should mention PHASE 4: VERIFY")
	}
	if !strings.Contains(prompt, "STRUCTURED DEBUGGING WORKFLOW") {
		t.Error("prompt should mention STRUCTURED DEBUGGING WORKFLOW")
	}
}

func TestResearcherPrompt_Content(t *testing.T) {
	task := newTestOpsTask("Research Go concurrency patterns")
	prompt := researcherPrompt(task)

	if !strings.Contains(prompt, "Researcher") {
		t.Error("prompt should mention Researcher role")
	}
	if !strings.Contains(prompt, "STEP 1: SCOPE") {
		t.Error("prompt should mention STEP 1: SCOPE")
	}
	if !strings.Contains(prompt, "STEP 2: SEARCH") {
		t.Error("prompt should mention STEP 2: SEARCH")
	}
	if !strings.Contains(prompt, "STEP 3: EXTRACT") {
		t.Error("prompt should mention STEP 3: EXTRACT")
	}
	if !strings.Contains(prompt, "STEP 4: CITE") {
		t.Error("prompt should mention STEP 4: CITE")
	}
	if !strings.Contains(prompt, "SOURCE CITATION") {
		t.Error("prompt should mention SOURCE CITATION")
	}
}

func TestLearnerPrompt_Content(t *testing.T) {
	task := newTestOpsTask("Analyze skill gaps")
	prompt := learnerPrompt(task)

	if !strings.Contains(prompt, "Learner") {
		t.Error("prompt should mention Learner role")
	}
	if !strings.Contains(prompt, "PHASE 1: OBSERVE") {
		t.Error("prompt should mention PHASE 1: OBSERVE")
	}
	if !strings.Contains(prompt, "PHASE 2: ANALYZE") {
		t.Error("prompt should mention PHASE 2: ANALYZE")
	}
	if !strings.Contains(prompt, "PHASE 3: PROPOSE") {
		t.Error("prompt should mention PHASE 3: PROPOSE")
	}
	if !strings.Contains(prompt, "SKILL PROPOSAL:") {
		t.Error("prompt should mention SKILL PROPOSAL format")
	}
	if !strings.Contains(prompt, "Do NOT create skill files") {
		t.Error("prompt should state do NOT create skill files")
	}
}

// --- parseOpsResult Tests ---

func TestParseOpsResult_Success(t *testing.T) {
	resp := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: successOpsResponse(),
	}
	result := parseOpsResult(resp)
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
	if len(result.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(result.Artifacts))
	}
	if result.NextRecommended != "none" {
		t.Errorf("expected next 'none', got %q", result.NextRecommended)
	}
	if result.SkillResolution != "none" {
		t.Errorf("expected skill resolution 'none', got %q", result.SkillResolution)
	}
}

func TestParseOpsResult_Partial(t *testing.T) {
	resp := &domain.Message{
		Role: domain.RoleAssistant,
		Content: `Status: partial
ExecutiveSummary: Some sources were unavailable, but partial findings are included.
Artifacts:
- partial-report
NextRecommended: none
Risks:
- Incomplete data from source 2
SkillResolution: none`,
	}
	result := parseOpsResult(resp)
	if result.Status != domain.SubagentPartial {
		t.Errorf("expected partial, got %q", result.Status)
	}
	if len(result.Risks) != 1 {
		t.Errorf("expected 1 risk, got %d", len(result.Risks))
	}
	if len(result.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(result.Artifacts))
	}
}

func TestParseOpsResult_Blocked(t *testing.T) {
	resp := &domain.Message{
		Role: domain.RoleAssistant,
		Content: `Status: blocked
ExecutiveSummary: Could not access required files for review.
NextRecommended: none
Risks: none
SkillResolution: none`,
	}
	result := parseOpsResult(resp)
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked, got %q", result.Status)
	}
}

func TestParseOpsResult_NilResponse(t *testing.T) {
	result := parseOpsResult(nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked for nil response, got %q", result.Status)
	}
}

func TestParseOpsResult_EmptyContent(t *testing.T) {
	resp := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: "",
	}
	result := parseOpsResult(resp)
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success (default), got %q", result.Status)
	}
	if result.NextRecommended != "none" {
		t.Errorf("expected next 'none' for empty content, got %q", result.NextRecommended)
	}
}

// --- Error Handling Tests ---

func TestReviewer_ExecuteProviderError(t *testing.T) {
	cfg := agent.SpawnerConfig{
		Provider: &opsStubProvider{
			chatErr: context.DeadlineExceeded,
		},
		Tools:  core.NewToolRegistry(),
		Budget: domain.DefaultBudget(),
	}
	spawner := agent.NewSpawner(cfg, agent.NewRegistry())
	sa := NewReviewer(spawner)
	task := newTestOpsTask("Review after timeout")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result on error")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked status on error, got %q", result.Status)
	}
}

func TestDebugger_ExecuteProviderError(t *testing.T) {
	cfg := agent.SpawnerConfig{
		Provider: &opsStubProvider{
			chatErr: context.DeadlineExceeded,
		},
		Tools:  core.NewToolRegistry(),
		Budget: domain.DefaultBudget(),
	}
	spawner := agent.NewSpawner(cfg, agent.NewRegistry())
	sa := NewDebugger(spawner)
	task := newTestOpsTask("Debug after timeout")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result on error")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked status on error, got %q", result.Status)
	}
}

func TestResearcher_ExecuteProviderError(t *testing.T) {
	cfg := agent.SpawnerConfig{
		Provider: &opsStubProvider{
			chatErr: context.DeadlineExceeded,
		},
		Tools:  core.NewToolRegistry(),
		Budget: domain.DefaultBudget(),
	}
	spawner := agent.NewSpawner(cfg, agent.NewRegistry())
	sa := NewResearcher(spawner)
	task := newTestOpsTask("Research after timeout")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result on error")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked status on error, got %q", result.Status)
	}
}

func TestLearner_ExecuteProviderError(t *testing.T) {
	cfg := agent.SpawnerConfig{
		Provider: &opsStubProvider{
			chatErr: context.DeadlineExceeded,
		},
		Tools:  core.NewToolRegistry(),
		Budget: domain.DefaultBudget(),
	}
	spawner := agent.NewSpawner(cfg, agent.NewRegistry())
	sa := NewLearner(spawner)
	task := newTestOpsTask("Analyze after timeout")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result on error")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked status on error, got %q", result.Status)
	}
}

// --- Description Content Tests ---

func TestReviewerDescription_ContainsGGA(t *testing.T) {
	sa := NewReviewer(nil)
	desc := sa.Description()
	if !strings.Contains(strings.ToLower(desc), "risk") {
		t.Error("reviewer description should mention risk lens")
	}
}

func TestDebuggerDescription_ContainsWorkflow(t *testing.T) {
	sa := NewDebugger(nil)
	desc := sa.Description()
	if !strings.Contains(strings.ToLower(desc), "debug") {
		t.Error("debugger description should mention debugging")
	}
	if !strings.Contains(desc, "fix") {
		t.Error("debugger description should mention fix capability")
	}
}

func TestResearcherDescription_ContainsWeb(t *testing.T) {
	sa := NewResearcher(nil)
	desc := sa.Description()
	if !strings.Contains(strings.ToLower(desc), "web") {
		t.Error("researcher description should mention web")
	}
	if !strings.Contains(strings.ToLower(desc), "cite") || !strings.Contains(strings.ToLower(desc), "source") {
		t.Error("researcher description should mention citing sources")
	}
}

func TestLearnerDescription_ContainsPropose(t *testing.T) {
	sa := NewLearner(nil)
	desc := sa.Description()
	if !strings.Contains(strings.ToLower(desc), "propos") {
		t.Error("learner description should mention proposing skills")
	}
}

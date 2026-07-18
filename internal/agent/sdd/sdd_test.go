package sdd

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

// sddStubProvider returns canned chat responses for subagent tests.
type sddStubProvider struct {
	resp    *domain.Message
	chatErr error
}

func (s *sddStubProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.resp, nil
}

func (s *sddStubProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		io.WriteString(pw, `{"content":"ok","done":true}`+"\n")
	}()
	return pr, nil
}

func (s *sddStubProvider) Tools() []domain.ToolDef { return nil }

// newSDDSpawner creates a Spawner with a stub provider and tool registry.
func newSDDSpawner() *agent.Spawner {
	cfg := agent.SpawnerConfig{
		Provider: &sddStubProvider{
			resp: &domain.Message{
				Role:    domain.RoleAssistant,
				Content: successResponse(),
			},
		},
		Tools:  core.NewToolRegistry(),
		Budget: domain.DefaultBudget(),
	}
	return agent.NewSpawner(cfg, agent.NewRegistry())
}

// successResponse returns a valid SDD envelope string for parseSDDResult.
func successResponse() string {
	return `Status: success
ExecutiveSummary: All tasks completed successfully. The implementation follows project conventions and passes all tests.
Artifacts:
- artifact-one
- artifact-two
NextRecommended: sdd-verify
Risks: none
SkillResolution: none`
}

func newTestTask(desc string) domain.SubagentTask {
	return domain.SubagentTask{
		ID:          "task-sdd-01",
		Description: desc,
		Mode:        "plan",
	}
}

// --- 2.7a: Interface Contract ---

func TestInterfaceContract_Explorer(t *testing.T) {
	var _ agent.Subagent = NewExplorer(nil)
	sa := NewExplorer(nil)
	if sa.Name() != "explorer" {
		t.Errorf("expected name 'explorer', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestInterfaceContract_Proposer(t *testing.T) {
	var _ agent.Subagent = NewProposer(nil)
	sa := NewProposer(nil)
	if sa.Name() != "proposer" {
		t.Errorf("expected name 'proposer', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestInterfaceContract_Specifier(t *testing.T) {
	var _ agent.Subagent = NewSpecifier(nil)
	sa := NewSpecifier(nil)
	if sa.Name() != "specifier" {
		t.Errorf("expected name 'specifier', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestInterfaceContract_Implementer(t *testing.T) {
	var _ agent.Subagent = NewImplementer(nil)
	sa := NewImplementer(nil)
	if sa.Name() != "implementer" {
		t.Errorf("expected name 'implementer', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestInterfaceContract_Verifier(t *testing.T) {
	var _ agent.Subagent = NewVerifier(nil)
	sa := NewVerifier(nil)
	if sa.Name() != "verifier" {
		t.Errorf("expected name 'verifier', got %q", sa.Name())
	}
	if sa.Description() == "" {
		t.Error("description should not be empty")
	}
}

// --- 2.7b: Execute with stub provider ---

func TestExplorer_Execute(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewExplorer(spawner)
	task := newTestTask("Investigate the agent package")

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

func TestProposer_Execute(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewProposer(spawner)
	task := newTestTask("Create proposal for auth module")

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

func TestSpecifier_Execute(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewSpecifier(spawner)
	task := newTestTask("Write delta specs for new API endpoint")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
}

func TestImplementer_Execute(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewImplementer(spawner)
	task := newTestTask("Implement the auth middleware")
	task.Mode = "build"

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
}

func TestVerifier_Execute(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewVerifier(spawner)
	task := newTestTask("Verify auth module implementation")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
}

// --- 2.7c: Tool filter enforcement ---

func TestExplorer_AllowedTools(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewExplorer(spawner)
	task := newTestTask("Investigate")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}

	// Prompt includes the tool list — verify it mentions read-only tools.
	prompt := explorerPrompt(task)
	if !strings.Contains(prompt, "file_read") {
		t.Error("explorer prompt should include file_read")
	}
	if !strings.Contains(prompt, "READ-ONLY") {
		t.Error("explorer prompt should mention READ-ONLY")
	}
	if strings.Contains(prompt, "file_write") {
		t.Error("explorer prompt should NOT mention file_write")
	}
}

func TestImplementer_AllowedTools(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewImplementer(spawner)
	task := newTestTask("Implement")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}

	prompt := implementerPrompt(task)
	if !strings.Contains(prompt, "file_write") {
		t.Error("implementer prompt should include file_write")
	}
	if !strings.Contains(prompt, "shell_exec") {
		t.Error("implementer prompt should include shell_exec")
	}
}

func TestVerifier_AllowedTools(t *testing.T) {
	spawner := newSDDSpawner()
	sa := NewVerifier(spawner)
	task := newTestTask("Verify")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}

	prompt := verifierPrompt(task)
	if strings.Contains(prompt, "file_write") {
		t.Error("verifier prompt should NOT mention file_write")
	}
	if !strings.Contains(prompt, "shell_exec") {
		t.Error("verifier prompt should include shell_exec")
	}
	if !strings.Contains(prompt, "MUST NOT write") {
		t.Error("verifier prompt should mention MUST NOT write")
	}
}

// --- 2.7d: System prompt content ---

func TestExplorerPrompt_Content(t *testing.T) {
	task := newTestTask("Find auth patterns")
	prompt := explorerPrompt(task)

	if !strings.Contains(prompt, "Explorer") {
		t.Error("prompt should mention Explorer role")
	}
	if !strings.Contains(prompt, "file_read") {
		t.Error("prompt should mention file_read tool")
	}
	if !strings.Contains(prompt, "READ-ONLY") {
		t.Error("prompt should mention read-only access")
	}
	if !strings.Contains(prompt, "sdd-propose") {
		t.Error("prompt should mention next recommended phase")
	}
}

func TestProposerPrompt_Content(t *testing.T) {
	task := newTestTask("Create proposal")
	prompt := proposerPrompt(task)

	if !strings.Contains(prompt, "Proposer") {
		t.Error("prompt should mention Proposer role")
	}
	if !strings.Contains(prompt, "INTENT") {
		t.Error("prompt should mention INTENT section")
	}
	if !strings.Contains(prompt, "SCOPE") {
		t.Error("prompt should mention SCOPE")
	}
	if !strings.Contains(prompt, "ROLLBACK") {
		t.Error("prompt should mention ROLLBACK plan")
	}
}

func TestSpecifierPrompt_Content(t *testing.T) {
	task := newTestTask("Write specs")
	prompt := specifierPrompt(task)

	if !strings.Contains(prompt, "Specifier") {
		t.Error("prompt should mention Specifier role")
	}
	if !strings.Contains(prompt, "RFC 2119") {
		t.Error("prompt should mention RFC 2119")
	}
	if !strings.Contains(prompt, "Given/When/Then") {
		t.Error("prompt should mention Given/When/Then format")
	}
	if !strings.Contains(prompt, "ADDED") {
		t.Error("prompt should mention ADDED requirements")
	}
}

func TestImplementerPrompt_Content(t *testing.T) {
	task := newTestTask("Write code")
	prompt := implementerPrompt(task)

	if !strings.Contains(prompt, "Implementer") {
		t.Error("prompt should mention Implementer role")
	}
	if !strings.Contains(prompt, "file_write") {
		t.Error("prompt should mention file_write tool")
	}
	if !strings.Contains(prompt, "shell_exec") {
		t.Error("prompt should mention shell_exec tool")
	}
	if !strings.Contains(prompt, "idiomatic Go") {
		t.Error("prompt should mention idiomatic Go")
	}
}

func TestVerifierPrompt_Content(t *testing.T) {
	task := newTestTask("Verify tests")
	prompt := verifierPrompt(task)

	if !strings.Contains(prompt, "Verifier") {
		t.Error("prompt should mention Verifier role")
	}
	if !strings.Contains(prompt, "go test") {
		t.Error("prompt should mention go test")
	}
	if !strings.Contains(prompt, "MUST NOT write") {
		t.Error("prompt should mention that Verifier must not write")
	}
}

// --- 2.7e: parseSDDResult ---

func TestParseSDDResult_Success(t *testing.T) {
	resp := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: successResponse(),
	}
	result := parseSDDResult(resp, "sdd-verify")
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
	if len(result.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(result.Artifacts))
	}
	if result.NextRecommended != "sdd-verify" {
		t.Errorf("expected next 'sdd-verify', got %q", result.NextRecommended)
	}
	if result.SkillResolution != "none" {
		t.Errorf("expected skill resolution 'none', got %q", result.SkillResolution)
	}
}

func TestParseSDDResult_Partial(t *testing.T) {
	resp := &domain.Message{
		Role: domain.RoleAssistant,
		Content: `Status: partial
ExecutiveSummary: Some tasks completed, others were blocked.
Artifacts:
- partial-result
NextRecommended: none
Risks:
- Incomplete spec coverage
SkillResolution: fallback-registry`,
	}
	result := parseSDDResult(resp, "sdd-spec")
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

func TestParseSDDResult_Blocked(t *testing.T) {
	resp := &domain.Message{
		Role: domain.RoleAssistant,
		Content: `Status: blocked
ExecutiveSummary: Could not proceed due to missing spec documentation.
NextRecommended: none
Risks: none
SkillResolution: none`,
	}
	result := parseSDDResult(resp, "none")
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked, got %q", result.Status)
	}
}

func TestParseSDDResult_NilResponse(t *testing.T) {
	result := parseSDDResult(nil, "none")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked for nil response, got %q", result.Status)
	}
}

func TestParseSDDResult_EmptyContent(t *testing.T) {
	resp := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: "",
	}
	result := parseSDDResult(resp, "sdd-verify")
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success (default), got %q", result.Status)
	}
	if result.Summary == "" {
		t.Log("summary is empty but that's ok for empty content")
	}
}

// --- 2.7f: Error handling in Execute ---

func TestExplorer_ExecuteProviderError(t *testing.T) {
	cfg := agent.SpawnerConfig{
		Provider: &sddStubProvider{
			chatErr: context.DeadlineExceeded,
		},
		Tools:  core.NewToolRegistry(),
		Budget: domain.DefaultBudget(),
	}
	spawner := agent.NewSpawner(cfg, agent.NewRegistry())
	sa := NewExplorer(spawner)
	task := newTestTask("Investigate after timeout")

	result := sa.Execute(context.Background(), task)
	if result == nil {
		t.Fatal("expected non-nil result on error")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked status on error, got %q", result.Status)
	}
}

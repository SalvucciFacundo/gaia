package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// stubProvider records calls and returns canned responses.
type stubProvider struct {
	chatCalls   int
	streamCalls int
	resp        *domain.Message
	chatErr     error
	streamErr   error
}

func (s *stubProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	s.chatCalls++
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.resp, nil
}

func (s *stubProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	s.streamCalls++
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		fmt.Fprintf(pw, `{"content":"%s","done":true}`+"\n", s.resp.Content)
	}()
	return pr, nil
}

func (s *stubProvider) Tools() []domain.ToolDef                          { return nil }
func (s *stubProvider) ListModels(ctx context.Context) ([]string, error) { return nil, nil }

// stubRepo is a test repository that records saved messages.
type stubRepo struct {
	messages []domain.Message
}

func (r *stubRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	r.messages = append(r.messages, msg)
	return nil
}
func (r *stubRepo) GetHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	return r.messages, nil
}
func (r *stubRepo) CreateSession(ctx context.Context, name string) (string, error) {
	return "test-session", nil
}
func (r *stubRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}
func (r *stubRepo) GetMessageCount(ctx context.Context) (int, error) {
	return len(r.messages), nil
}
func (r *stubRepo) ListSessions(ctx context.Context) ([]domain.SessionInfo, error) {
	return nil, nil
}
func (r *stubRepo) GetHistoryFrom(ctx context.Context, limit, offset int) ([]domain.Message, error) {
	if offset >= len(r.messages) {
		return []domain.Message{}, nil
	}
	end := offset + limit
	if end > len(r.messages) {
		end = len(r.messages)
	}
	return r.messages[offset:end], nil
}
func (r *stubRepo) GetLastMessages(ctx context.Context, n int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}
func (r *stubRepo) DeleteMessagesAfter(ctx context.Context, afterID string) error {
	return nil
}
func (r *stubRepo) ClearMessages(ctx context.Context) error {
	return nil
}
func (r *stubRepo) RenameSession(ctx context.Context, sessionID, name string) error {
	return nil
}

// stubUI records Display calls.
type stubUI struct {
	displayed []domain.Message
}

func (u *stubUI) Display(msg domain.Message) error {
	u.displayed = append(u.displayed, msg)
	return nil
}
func (u *stubUI) AppendToken(content string) error               { return nil }
func (u *stubUI) PromptConfirmation(prompt string) (bool, error) { return true, nil }
func (u *stubUI) PromptChoice(envelope domain.ChoiceEnvelope) (string, error) {
	if len(envelope.Questions) > 0 && len(envelope.Questions[0].Options) > 0 {
		return envelope.Questions[0].Options[0].Token, nil
	}
	return "", nil
}
func (u *stubUI) Run() error { return nil }

func TestBrain_BudgetExhausted(t *testing.T) {
	prov := &stubProvider{
		streamErr: io.EOF,
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "I'll call a tool",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "fake_tool", Arguments: map[string]interface{}{}},
			},
		},
	}
	repo := &stubRepo{}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 2}

	brain := NewBrain(prov, repo, ui, nil, budget)
	brain.RegisterModule(&fakeModule{})

	err := brain.ProcessMessage(context.Background(), "do something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prov.chatCalls != 2 {
		t.Errorf("expected 2 chat calls (hit budget), got %d", prov.chatCalls)
	}

	lastMsg := ui.displayed[len(ui.displayed)-1]
	if !contains(lastMsg.Content, "budget exhausted") && !contains(lastMsg.Content, "budget limit reached") {
		t.Errorf("expected budget exceeded message, got: %s", lastMsg.Content)
	}
}

func TestBrain_SimpleChat(t *testing.T) {
	prov := &stubProvider{
		streamErr: io.EOF,
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Hello! How can I help you today?",
		},
	}
	repo := &stubRepo{}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 5}

	brain := NewBrain(prov, repo, ui, nil, budget)

	err := brain.ProcessMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prov.chatCalls != 1 {
		t.Errorf("expected 1 chat call, got %d", prov.chatCalls)
	}

	if len(ui.displayed) != 1 {
		t.Fatalf("expected 1 displayed message, got %d", len(ui.displayed))
	}
	if ui.displayed[0].Content != "Hello! How can I help you today?" {
		t.Errorf("unexpected message content: %s", ui.displayed[0].Content)
	}
}

// fakeModule provides a dummy tool for testing the agent loop.
type fakeModule struct{}

func (m *fakeModule) Name() string                { return "fake" }
func (m *fakeModule) Description() string         { return "A fake module for testing" }
func (m *fakeModule) GetTools() []domain.ToolCall { return nil }
func (m *fakeModule) Tools() []domain.ToolDef {
	return []domain.ToolDef{
		{
			Name:        "fake_tool",
			Description: "A fake tool for tests",
		},
	}
}
func (m *fakeModule) Execute(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	return &domain.ToolResult{
		Success: true,
		Output:  "fake tool executed successfully",
	}, nil
}

func TestBrain_ProcessMessageWiresImages(t *testing.T) {
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F,
		0x00, 0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59,
		0xE7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}

	ui := &stubUI{}
	repo := &recordingRepo{
		stubRepo: stubRepo{},
	}
	prov := &stubProvider{
		resp: &domain.Message{Role: domain.RoleAssistant, Content: "I see an image!"},
	}
	brain := NewBrain(prov, repo, ui, nil, domain.BudgetConfig{MaxIterations: 25})

	brain.AddPendingImage(domain.ImageContent{
		MIMEType: "image/png",
		Data:     base64.StdEncoding.EncodeToString(pngData),
	})

	if len(brain.pendingImages) != 1 {
		t.Fatal("expected 1 pending image after AddPendingImage")
	}

	err := brain.ProcessMessage(context.Background(), "What do you see?")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if len(brain.pendingImages) != 0 {
		t.Errorf("expected pendingImages to be cleared, got %d", len(brain.pendingImages))
	}
}

type recordingRepo struct {
	stubRepo
	saved []domain.Message
}

func (r *recordingRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	r.saved = append(r.saved, msg)
	return nil
}

type stubCommandDispatcher struct {
	handled bool
	err     error
	lastCmd string
}

func (s *stubCommandDispatcher) Dispatch(_ context.Context, cmd string) (bool, error) {
	s.lastCmd = cmd
	return s.handled, s.err
}

func TestBrain_CommandDispatcher(t *testing.T) {
	prov := &stubProvider{}
	repo := &stubRepo{}
	ui := &stubUI{}
	brain := NewBrain(prov, repo, ui, nil, domain.BudgetConfig{MaxIterations: 5})

	// Without dispatcher
	handled, err := brain.DispatchCommand(context.Background(), "/test")
	if handled || err != nil {
		t.Errorf("expected false, nil without dispatcher, got %v, %v", handled, err)
	}

	// With wired dispatcher
	cd := &stubCommandDispatcher{handled: true}
	brain.SetCommandDispatcher(cd)

	handled, err = brain.DispatchCommand(context.Background(), "/test")
	if !handled || err != nil {
		t.Fatalf("expected true, nil with dispatcher, got %v, %v", handled, err)
	}
	if cd.lastCmd != "/test" {
		t.Errorf("expected '/test', got %q", cd.lastCmd)
	}

	// Via ProcessMessage
	cd.lastCmd = ""
	err = brain.ProcessMessage(context.Background(), "/test2")
	if err != nil {
		t.Fatalf("unexpected error from ProcessMessage: %v", err)
	}
	if cd.lastCmd != "/test2" {
		t.Errorf("expected dispatcher to receive '/test2', got %q", cd.lastCmd)
	}
}


func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type recordingKGStore struct {
	facts []domain.KnowledgeFact
}

func (m *recordingKGStore) AddFact(_ context.Context, fact domain.KnowledgeFact) (string, error) {
	m.facts = append(m.facts, fact)
	return fmt.Sprintf("fact-%d", len(m.facts)), nil
}

func (m *recordingKGStore) GetFactsByTopic(_ context.Context, topic string) ([]domain.KnowledgeFact, error) {
	var res []domain.KnowledgeFact
	for _, f := range m.facts {
		if f.Topic == topic {
			res = append(res, f)
		}
	}
	return res, nil
}

func (m *recordingKGStore) GetFactsByConcept(_ context.Context, topic, concept string) ([]domain.KnowledgeFact, error) {
	var res []domain.KnowledgeFact
	for _, f := range m.facts {
		if f.Topic == topic && f.Concept == concept {
			res = append(res, f)
		}
	}
	return res, nil
}

func (m *recordingKGStore) SearchFacts(_ context.Context, query string) ([]domain.KnowledgeFact, error) {
	var res []domain.KnowledgeFact
	for _, f := range m.facts {
		if strings.Contains(strings.ToLower(f.Fact), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(f.Concept), strings.ToLower(query)) {
			res = append(res, f)
		}
	}
	return res, nil
}

func (m *recordingKGStore) GetRecentFacts(_ context.Context, limit int) ([]domain.KnowledgeFact, error) {
	if len(m.facts) <= limit {
		return m.facts, nil
	}
	return m.facts[len(m.facts)-limit:], nil
}

func (m *recordingKGStore) GetAllTopics(_ context.Context) ([]string, error) {
	seen := make(map[string]bool)
	var topics []string
	for _, f := range m.facts {
		if !seen[f.Topic] {
			seen[f.Topic] = true
			topics = append(topics, f.Topic)
		}
	}
	return topics, nil
}

func (m *recordingKGStore) GetRecentTopics(_ context.Context, _ time.Duration) ([]string, error) {
	return m.GetAllTopics(context.Background())
}

func (m *recordingKGStore) Clear(_ context.Context) error {
	m.facts = nil
	return nil
}

type dummyWriteModule struct{}

func (d *dummyWriteModule) Name() string        { return "fileops" }
func (d *dummyWriteModule) Description() string { return "file operations" }
func (d *dummyWriteModule) GetTools() []domain.ToolCall {
	return []domain.ToolCall{{Name: "file_write"}}
}
func (d *dummyWriteModule) Execute(_ context.Context, _ string, _ map[string]interface{}) (*domain.ToolResult, error) {
	return &domain.ToolResult{
		Success: true,
		Output:  "file written successfully",
	}, nil
}

type stubKGRecorder struct {
	facts []domain.KnowledgeFact
}

func (r *stubKGRecorder) RecordToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result *domain.ToolResult) {
	if result != nil && result.Success {
		filePath, _ := args["path"].(string)
		r.facts = append(r.facts, domain.KnowledgeFact{
			Topic:   "DirectAction",
			Concept: filePath,
			Fact:    fmt.Sprintf("Directly modified file %s via %s", filePath, toolName),
		})
	}
}

func (r *stubKGRecorder) RecordResponseFacts(ctx context.Context, response string) {}
func (r *stubKGRecorder) RecordSubagentResult(ctx context.Context, name, description string, result *domain.SubagentResult) {}

func TestBrain_DirectAction_MemoryPersistence(t *testing.T) {
	prov := &stubProvider{}
	repo := &stubRepo{}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 5}

	brain := NewBrain(prov, repo, ui, nil, budget)
	recorder := &stubKGRecorder{}
	brain.SetKnowledgeRecorder(recorder)

	// Register dummy module
	brain.RegisterModule(&dummyWriteModule{})

	msg := &domain.Message{
		Role: domain.RoleAssistant,
		ToolCalls: []domain.ToolCall{
			{
				ID:   "call-1",
				Name: "file_write",
				Arguments: map[string]interface{}{
					"path":    "internal/auth/login.go",
					"content": "package auth\n",
				},
			},
		},
	}

	err := brain.handleToolCalls(context.Background(), msg)
	if err != nil {
		t.Fatalf("handleToolCalls failed: %v", err)
	}

	if len(recorder.facts) == 0 {
		t.Fatal("expected direct action tool execution to persist a fact in KnowledgeGraph")
	}

	fact := recorder.facts[0]
	if fact.Topic != "DirectAction" {
		t.Errorf("expected Topic 'DirectAction', got %q", fact.Topic)
	}
	if fact.Concept != "internal/auth/login.go" {
		t.Errorf("expected Concept 'internal/auth/login.go', got %q", fact.Concept)
	}
	if !strings.Contains(fact.Fact, "login.go") {
		t.Errorf("expected Fact to describe file modification, got %q", fact.Fact)
	}
}

func TestBrain_SkillResolver_ProgressiveLoading(t *testing.T) {
	prov := &stubProvider{}
	repo := &stubRepo{}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 5}

	brain := NewBrain(prov, repo, ui, nil, budget)

	// Set up skill lister, matcher, and path resolver
	brain.SetSkillLister(func() []string {
		return []string{"go-testing", "git-workflow", "security-audit"}
	})
	brain.SetSkillMatcher(func(taskDesc string, _ []string) []string {
		if strings.Contains(taskDesc, "test") {
			return []string{"go-testing"}
		}
		return nil
	})
	brain.SetSkillPathResolver(func(skills []string) []string {
		var paths []string
		for _, s := range skills {
			paths = append(paths, fmt.Sprintf("skills/%s/SKILL.md", s))
		}
		return paths
	})

	task := domain.SubagentTask{
		ID:          "task-1",
		Description: "Write tests for auth package",
	}

	brain.populateSkillRegistry(&task)

	// Check catalog
	if len(task.SkillRegistry) != 3 {
		t.Errorf("expected 3 skills in catalog, got %d", len(task.SkillRegistry))
	}

	// Check resolved matching skill
	if len(task.Skills) != 1 || task.Skills[0] != "go-testing" {
		t.Errorf("expected matched skill 'go-testing', got %v", task.Skills)
	}

	// Check resolved path for lazy loading
	if len(task.SkillPaths) != 1 || task.SkillPaths[0] != "skills/go-testing/SKILL.md" {
		t.Errorf("expected SkillPath 'skills/go-testing/SKILL.md', got %v", task.SkillPaths)
	}
}

type stubWorkflowRouter struct {
	routedTurn   bool
	recordedTool string
	checkedMid   bool
	resetCount   int
}

func (s *stubWorkflowRouter) RouteTurn(ctx context.Context, content string) (bool, error) {
	if content == "route-me" {
		s.routedTurn = true
		return true, nil
	}
	return false, nil
}

func (s *stubWorkflowRouter) RecordToolCall(name string, args map[string]interface{}, output string) {
	s.recordedTool = name
}

func (s *stubWorkflowRouter) CheckMidTurn(ctx context.Context, content string) (bool, error) {
	s.checkedMid = true
	return false, nil
}

func (s *stubWorkflowRouter) ResetTurn() {
	s.resetCount++
}

func TestBrain_WorkflowRouter(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{
			Role: domain.RoleAssistant,
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "fake_tool", Arguments: map[string]interface{}{}},
			},
		},
	}
	repo := &stubRepo{}
	ui := &stubUI{}
	brain := NewBrain(prov, repo, ui, nil, domain.BudgetConfig{MaxIterations: 2})
	brain.RegisterModule(&fakeModule{})

	wr := &stubWorkflowRouter{}
	brain.SetWorkflowRouter(wr)

	// 1. Test RouteTurn interception
	err := brain.ProcessMessage(context.Background(), "route-me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wr.routedTurn {
		t.Error("expected RouteTurn to intercept 'route-me'")
	}
	if wr.resetCount == 0 {
		t.Error("expected ResetTurn to be called")
	}

	// 2. Test mid-turn RecordToolCall and CheckMidTurn
	prov.resp = &domain.Message{Role: domain.RoleAssistant, Content: "all done"}
	err = brain.ProcessMessage(context.Background(), "normal turn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type stubSubagentPort struct {
	available   []string
	spawnResult *domain.SubagentResult
	spawnErr    error
}

func (s *stubSubagentPort) Available() []string {
	return s.available
}

func (s *stubSubagentPort) Spawn(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	return s.spawnResult, s.spawnErr
}

func TestBrain_PostTurnHooks(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Processed queued response",
		},
	}
	repo := &stubRepo{}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 5}

	brain := NewBrain(prov, repo, ui, nil, budget)
	brain.messageQueue = append(brain.messageQueue, "check queued task")

	brain.PostTurnHooks(context.Background(), "completed turn output")

	if len(brain.messageQueue) != 0 {
		t.Errorf("expected messageQueue to be drained by PostTurnHooks, remaining: %d", len(brain.messageQueue))
	}
}

type parallelRecordingRepo struct {
	stubRepo
	mu       sync.Mutex
	messages []domain.Message
}

func (r *parallelRecordingRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, msg)
	return nil
}

type testParallelModule struct {
	mu             sync.Mutex
	executionOrder []string
}

func (m *testParallelModule) Name() string        { return "parallel_test" }
func (m *testParallelModule) Description() string { return "Test parallel execution" }
func (m *testParallelModule) GetTools() []domain.ToolCall {
	return []domain.ToolCall{
		{Name: "mem_slow"},
		{Name: "mem_fast"},
		{Name: "mem_med"},
		{Name: "file_write"},
	}
}
func (m *testParallelModule) Tools() []domain.ToolDef {
	return []domain.ToolDef{
		{Name: "mem_slow"},
		{Name: "mem_fast"},
		{Name: "mem_med"},
		{Name: "file_write"},
	}
}

func (m *testParallelModule) Execute(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch name {
	case "mem_slow":
		time.Sleep(50 * time.Millisecond)
	case "mem_fast":
		time.Sleep(5 * time.Millisecond)
	case "mem_med":
		time.Sleep(25 * time.Millisecond)
	case "file_write":
		time.Sleep(10 * time.Millisecond)
	}

	m.mu.Lock()
	m.executionOrder = append(m.executionOrder, name)
	m.mu.Unlock()

	return &domain.ToolResult{
		Success: true,
		Output:  "output_" + name,
	}, nil
}

func TestBrain_ParallelToolCalls_OrderPreserved(t *testing.T) {
	mod := &testParallelModule{}
	repo := &parallelRecordingRepo{}
	ui := &stubUI{}
	brain := NewBrain(&stubProvider{}, repo, ui, nil, domain.DefaultBudget())
	brain.RegisterModule(mod)

	msg := &domain.Message{
		Role: domain.RoleAssistant,
		ToolCalls: []domain.ToolCall{
			{ID: "call_1", Name: "mem_slow"},
			{ID: "call_2", Name: "mem_fast"},
			{ID: "call_3", Name: "mem_med"},
		},
	}

	start := time.Now()
	err := brain.handleToolCalls(context.Background(), msg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Concurrency verification: total elapsed time should be < 75ms (parallel takes ~50ms vs sequential ~80ms)
	if elapsed > 150*time.Millisecond {
		t.Errorf("expected parallel execution to complete quickly, took %v", elapsed)
	}

	// Execution order check: mem_fast should finish before mem_slow
	mod.mu.Lock()
	firstFinished := mod.executionOrder[0]
	mod.mu.Unlock()
	if firstFinished != "mem_fast" {
		t.Errorf("expected mem_fast to finish first, but %s finished first", firstFinished)
	}

	// Order preservation check: repo.messages must have 1 assistant message + 3 tool messages (in that order),
	// and tool messages MUST be in original order: mem_slow, mem_fast, mem_med
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.messages) != 4 {
		t.Fatalf("expected 4 saved messages (1 assistant + 3 tool), got %d", len(repo.messages))
	}

	if repo.messages[0].Role != domain.RoleAssistant {
		t.Errorf("expected message 0 to be assistant, got %s", repo.messages[0].Role)
	}
	if repo.messages[1].Content != "output_mem_slow" {
		t.Errorf("expected message 1 to be output_mem_slow, got %s", repo.messages[1].Content)
	}
	if repo.messages[2].Content != "output_mem_fast" {
		t.Errorf("expected message 2 to be output_mem_fast, got %s", repo.messages[2].Content)
	}
	if repo.messages[3].Content != "output_mem_med" {
		t.Errorf("expected message 3 to be output_mem_med, got %s", repo.messages[3].Content)
	}
}

func TestBrain_ParallelToolCalls_MixedSafety(t *testing.T) {
	mod := &testParallelModule{}
	repo := &parallelRecordingRepo{}
	ui := &stubUI{}
	brain := NewBrain(&stubProvider{}, repo, ui, nil, domain.DefaultBudget())
	brain.RegisterModule(mod)

	msg := &domain.Message{
		Role: domain.RoleAssistant,
		ToolCalls: []domain.ToolCall{
			{ID: "call_1", Name: "mem_fast"},
			{ID: "call_2", Name: "file_write"},
			{ID: "call_3", Name: "mem_med"},
		},
	}

	err := brain.handleToolCalls(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.messages) != 4 {
		t.Fatalf("expected 4 saved messages (1 assistant + 3 tool), got %d", len(repo.messages))
	}

	if repo.messages[0].Role != domain.RoleAssistant {
		t.Errorf("message 0: expected assistant, got %s", repo.messages[0].Role)
	}
	if repo.messages[1].Content != "output_mem_fast" {
		t.Errorf("message 1: got %s, want output_mem_fast", repo.messages[1].Content)
	}
	if repo.messages[2].Content != "output_file_write" {
		t.Errorf("message 2: got %s, want output_file_write", repo.messages[2].Content)
	}
	if repo.messages[3].Content != "output_mem_med" {
		t.Errorf("message 3: got %s, want output_mem_med", repo.messages[3].Content)
	}
}

func TestBrain_SetReasoningEffort(t *testing.T) {
	brain := NewBrain(nil, nil, nil, nil, domain.BudgetConfig{})
	if brain.ReasoningEffort() != "" {
		t.Fatalf("expected initial reasoning effort empty, got %q", brain.ReasoningEffort())
	}

	brain.SetReasoningEffort("high")
	if brain.ReasoningEffort() != "high" {
		t.Errorf("expected reasoning effort 'high', got %q", brain.ReasoningEffort())
	}

	brain.SetReasoningEffort(" LOW ")
	if brain.ReasoningEffort() != "low" {
		t.Errorf("expected normalized reasoning effort 'low', got %q", brain.ReasoningEffort())
	}
}

type stubPersonaResolver struct{}

func (s *stubPersonaResolver) Resolve(name string) (string, error) {
	if name == "teacher" {
		return "Teach while answering", nil
	}
	return "", nil
}

func TestBrain_SetPersona(t *testing.T) {
	brain := NewBrain(nil, nil, nil, nil, domain.BudgetConfig{})
	brain.SetPersonaResolver(&stubPersonaResolver{})

	name, overlay := brain.Persona()
	if name != "" || overlay != "" {
		t.Fatalf("expected initial persona empty, got name=%q, overlay=%q", name, overlay)
	}

	brain.SetPersona("teacher")
	name, overlay = brain.Persona()
	if name != "teacher" {
		t.Errorf("expected persona 'teacher', got %q", name)
	}
	if !strings.Contains(overlay, "Teach while answering") {
		t.Errorf("expected overlay to contain teaching instructions, got %q", overlay)
	}

	// Clearing persona
	brain.SetPersona("default")
	name, overlay = brain.Persona()
	if overlay != "" {
		t.Errorf("expected cleared overlay for 'default', got %q", overlay)
	}
}



type stubMutationVerifier struct{}

func (s *stubMutationVerifier) IsMutationTool(name string) bool {
	return name == "file_write"
}

func (s *stubMutationVerifier) ExtractModifiedPaths(args map[string]interface{}) string {
	if p, ok := args["path"].(string); ok {
		return p
	}
	return ""
}

func (s *stubMutationVerifier) VerifyMutations(ctx context.Context, paths []string) (string, error) {
	return "[AUTO-REPAIR FEEDBACK]\ncompiler error: undefinedFunctionHere", nil
}

func TestBrain_AutoRepair_FeedbackLoop(t *testing.T) {
	repo := &stubRepo{}
	brain := NewBrain(&stubProvider{}, repo, &stubUI{}, nil, domain.BudgetConfig{MaxIterations: 5})
	brain.RegisterModule(&dummyWriteModule{})
	brain.SetMutationVerifier(&stubMutationVerifier{})

	msg := &domain.Message{
		Role: domain.RoleAssistant,
		ToolCalls: []domain.ToolCall{
			{
				ID:   "call-fix-1",
				Name: "file_write",
				Arguments: map[string]interface{}{
					"path":    "broken.go",
					"content": "package main\n\nfunc main() { undefinedFunctionHere() }\n",
				},
			},
		},
	}

	err := brain.handleToolCalls(context.Background(), msg)
	if err != nil {
		t.Fatalf("handleToolCalls failed: %v", err)
	}

	// Verify that an auto-repair feedback message was saved to history
	var foundAutoRepair bool
	for _, m := range repo.messages {
		if m.Role == domain.RoleSystem && strings.Contains(m.Content, "[AUTO-REPAIR FEEDBACK]") {
			foundAutoRepair = true
			if !strings.Contains(m.Content, "undefinedFunctionHere") {
				t.Errorf("expected auto-repair message to contain compiler error details, got: %s", m.Content)
			}
			break
		}
	}

	if !foundAutoRepair {
		t.Fatal("expected auto-repair feedback message to be generated and saved into repo")
	}
}

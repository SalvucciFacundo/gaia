package agent

import (
	"context"
	"errors"
	"testing"

	"gaia/internal/agent/memory"
	"gaia/internal/core/domain"
)

// --- Mock DefRepository ---

type mockDefRepo struct {
	defs map[string]SubagentDef
}

func newMockDefRepo() *mockDefRepo {
	return &mockDefRepo{defs: make(map[string]SubagentDef)}
}

func (m *mockDefRepo) CreateDef(_ context.Context, def SubagentDef) error {
	if _, ok := m.defs[def.Name]; ok {
		return errors.New("already exists")
	}
	m.defs[def.Name] = def
	return nil
}

func (m *mockDefRepo) GetDef(_ context.Context, name string) (*SubagentDef, error) {
	def, ok := m.defs[name]
	if !ok {
		return nil, nil
	}
	return &def, nil
}

func (m *mockDefRepo) ListDefs(_ context.Context) ([]SubagentDef, error) {
	var defs []SubagentDef
	for _, d := range m.defs {
		defs = append(defs, d)
	}
	return defs, nil
}

func (m *mockDefRepo) UpdateDef(_ context.Context, def SubagentDef) error {
	if _, ok := m.defs[def.Name]; !ok {
		return errors.New("not found")
	}
	m.defs[def.Name] = def
	return nil
}

func (m *mockDefRepo) DeleteDef(_ context.Context, name string) error {
	if _, ok := m.defs[name]; !ok {
		return errors.New("not found")
	}
	delete(m.defs, name)
	return nil
}

// --- Tests ---

func TestDynamicSubagent_Interface(t *testing.T) {
	def := SubagentDef{
		Name:         "test-agent",
		Description:  "A test subagent",
		AllowedTools: []string{"read", "write"},
		Skills:       []string{"testing"},
		SystemPrompt: "You are a test agent.",
		Personality:  "helpful",
	}

	ds := NewDynamicSubagent(def, nil)

	if name := ds.Name(); name != "test-agent" {
		t.Errorf("Name() = %q, want %q", name, "test-agent")
	}
	if desc := ds.Description(); desc != "A test subagent" {
		t.Errorf("Description() = %q, want %q", desc, "A test subagent")
	}

	// Verify interface satisfaction
	var _ Subagent = ds
}

func TestDynamicLoader_CreateFromDef_ToolValidation(t *testing.T) {
	repo := newMockDefRepo()
	registry := NewRegistry()
	ns := memory.NewNamespaceManager("test")

	// Pre-register an agent so we can check it doesn't conflict
	def := SubagentDef{
		Name:         "real",
		Description:  "existing",
		AllowedTools: []string{"tool_a"},
		SystemPrompt: "prompt",
	}
	if err := repo.CreateDef(context.Background(), def); err != nil {
		t.Fatal(err)
	}
	// Register the factory so LoadAll can register it
	if err := registry.Register("real", func(s *Spawner) Subagent {
		return NewDynamicSubagent(def, s)
	}); err != nil {
		t.Fatal(err)
	}

	loader := NewDynamicLoader(repo, registry, nil, ns)
	loader.SetValidator(newStrictValidator([]string{"tool_a", "tool_b", "tool_c"}))

	// Valid create
	newDef := SubagentDef{
		Name:         "newbie",
		Description:  "a new one",
		AllowedTools: []string{"tool_a", "tool_b"},
		SystemPrompt: "prompt",
	}
	if err := loader.CreateFromDef(context.Background(), newDef); err != nil {
		t.Errorf("CreateFromDef should succeed: %v", err)
	}

	// Verify it was persisted
	persisted, err := repo.GetDef(context.Background(), "newbie")
	if err != nil {
		t.Fatal(err)
	}
	if persisted == nil {
		t.Fatal("def not persisted")
	}
	if persisted.Name != "newbie" {
		t.Errorf("persisted name = %q, want newbie", persisted.Name)
	}

	// Verify it was registered
	factory, ok := registry.Get("newbie")
	if !ok {
		t.Fatal("subagent not registered in registry")
	}
	sa := factory(nil)
	if sa.Name() != "newbie" {
		t.Errorf("factory name = %q, want newbie", sa.Name())
	}
}

func TestDynamicLoader_CreateFromDef_InvalidTool(t *testing.T) {
	repo := newMockDefRepo()
	registry := NewRegistry()
	ns := memory.NewNamespaceManager("test")

	loader := NewDynamicLoader(repo, registry, nil, ns)
	loader.SetValidator(newStrictValidator([]string{"tool_a", "tool_b"}))

	def := SubagentDef{
		Name:         "bad-agent",
		Description:  "uses unknown tool",
		AllowedTools: []string{"tool_a", "nonexistent"},
		SystemPrompt: "prompt",
	}

	err := loader.CreateFromDef(context.Background(), def)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestDynamicLoader_LoadAll(t *testing.T) {
	repo := newMockDefRepo()
	registry := NewRegistry()
	ns := memory.NewNamespaceManager("test")

	// Seed some defs
	defs := []SubagentDef{
		{Name: "alpha", Description: "alpha desc", AllowedTools: []string{"a"}, SystemPrompt: "p"},
		{Name: "beta", Description: "beta desc", AllowedTools: []string{"b"}, SystemPrompt: "p"},
		{Name: "gamma", Description: "gamma desc", AllowedTools: []string{"c"}, SystemPrompt: "p"},
	}

	for _, d := range defs {
		if err := repo.CreateDef(context.Background(), d); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewDynamicLoader(repo, registry, nil, ns)
	if err := loader.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	// Verify all registered
	available := registry.Available()
	if len(available) < 3 {
		t.Errorf("expected at least 3 registered, got %d: %v", len(available), available)
	}

	for _, d := range defs {
		factory, ok := registry.Get(d.Name)
		if !ok {
			t.Errorf("subagent %q not registered", d.Name)
			continue
		}
		sa := factory(nil)
		if sa.Name() != d.Name {
			t.Errorf("factory for %q returns Name()=%q", d.Name, sa.Name())
		}
		if sa.Description() != d.Description {
			t.Errorf("factory for %q returns Description()=%q, want %q", d.Name, sa.Description(), d.Description)
		}
	}
}

func TestDynamicLoader_CreateFromDef_DuplicateName(t *testing.T) {
	repo := newMockDefRepo()
	registry := NewRegistry()
	ns := memory.NewNamespaceManager("test")

	loader := NewDynamicLoader(repo, registry, nil, ns)
	loader.SetValidator(newStrictValidator([]string{"tool_a"}))

	def := SubagentDef{
		Name:         "dup",
		Description:  "first",
		AllowedTools: []string{"tool_a"},
		SystemPrompt: "p",
	}

	// First create should succeed
	if err := loader.CreateFromDef(context.Background(), def); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Second create should fail (duplicate registry name)
	err := loader.CreateFromDef(context.Background(), def)
	if err == nil {
		t.Fatal("expected error for duplicate subagent name")
	}
}

func TestDynamicLoader_RemoveDynamic(t *testing.T) {
	repo := newMockDefRepo()
	registry := NewRegistry()
	ns := memory.NewNamespaceManager("test")

	loader := NewDynamicLoader(repo, registry, nil, ns)
	loader.SetValidator(newStrictValidator([]string{"tool_a"}))

	def := SubagentDef{
		Name:         "removable",
		Description:  "temp",
		AllowedTools: []string{"tool_a"},
		SystemPrompt: "p",
	}

	if err := loader.CreateFromDef(context.Background(), def); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Remove should succeed
	if err := loader.RemoveDynamic(context.Background(), "removable"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Verify gone from registry
	if _, ok := registry.Get("removable"); ok {
		t.Error("subagent still in registry after removal")
	}

	// Verify gone from repo
	persisted, err := repo.GetDef(context.Background(), "removable")
	if err != nil {
		t.Fatal(err)
	}
	if persisted != nil {
		t.Error("def still in repo after removal")
	}
}

func TestBuildDynamicPrompt(t *testing.T) {
	def := SubagentDef{
		Name:         "helper",
		Description:  "Helps with stuff",
		SystemPrompt: "You are a helpful assistant.",
		Personality:  "Warm and concise.",
		Skills:       []string{"code-review"},
	}

	task := domain.SubagentTask{
		ID:          "task-1",
		Description: "Explain concurrency",
		KGContext:   []string{"Go routines are lightweight threads"},
	}

	prompt := buildDynamicPrompt(def, task)

	checks := []string{
		def.SystemPrompt,
		def.Personality,
		def.Name,
		task.Description,
		task.KGContext[0],
	}

	for _, check := range checks {
		if !containsString(prompt, check) {
			t.Errorf("prompt missing %q\ngot: %s", check, prompt)
		}
	}
}

func TestDynamicSubagent_FactoryClosure(t *testing.T) {
	def := SubagentDef{
		Name:         "closure-test",
		Description:  "Testing closures",
		AllowedTools: []string{"a", "b"},
	}

	// Simulate what DynamicLoader.register does
	factory := func(spawner *Spawner) Subagent {
		return NewDynamicSubagent(def, spawner)
	}

	sa := factory(nil)
	if sa.Name() != "closure-test" {
		t.Errorf("factory Name() = %q, want closure-test", sa.Name())
	}
	if sa.Description() != "Testing closures" {
		t.Errorf("factory Description() = %q", sa.Description())
	}
}

// --- Helpers ---

func containsString(s, sub string) bool {
	if s == "" {
		return false
	}
	return len(sub) == 0 || stringContains(s, sub)
}

func stringContains(s, sub string) bool {
	// Simple linear search
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// newStrictValidator creates a ToolValidator that checks each tool against
// the allowed list and returns an error for unknowns.
func newStrictValidator(allowed []string) ToolValidator {
	return func(tools []string) error {
		for _, tool := range tools {
			found := false
			for _, a := range allowed {
				if a == tool {
					found = true
					break
				}
			}
			if !found {
				return errors.New("unknown tool: " + tool)
			}
		}
		return nil
	}
}

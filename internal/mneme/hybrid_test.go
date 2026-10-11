package mneme

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.uber.org/goleak"
)

// mockEmbedder implements Embedder for deterministic testing.
type mockEmbedder struct {
	mu          sync.Mutex
	dim         int
	embedFunc   func(ctx context.Context, text string) ([]float32, error)
	embedCalls  int
	batchCalls  int
	errToReturn error
}

func newMockEmbedder(dim int) *mockEmbedder {
	return &mockEmbedder{
		dim: dim,
	}
}

func (m *mockEmbedder) Dimension() int {
	return m.dim
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	m.embedCalls++
	err := m.errToReturn
	fn := m.embedFunc
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if fn != nil {
		return fn(ctx, text)
	}
	return deterministicVector(text, m.dim), nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	m.batchCalls++
	err := m.errToReturn
	fn := m.embedFunc
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if fn != nil {
			v, err := fn(ctx, t)
			if err != nil {
				return nil, err
			}
			out[i] = v
		} else {
			out[i] = deterministicVector(t, m.dim)
		}
	}
	return out, nil
}

// deterministicVector generates a unit-like float32 vector based on keyword presence.
func deterministicVector(text string, dim int) []float32 {
	v := make([]float32, dim)
	lower := strings.ToLower(text)

	// Concept 1: automotive
	if strings.Contains(lower, "car") || strings.Contains(lower, "automobile") ||
		strings.Contains(lower, "vehicle") || strings.Contains(lower, "repair") ||
		strings.Contains(lower, "maintenance") {
		v[0] = 0.9
		v[1] = 0.1
	}

	// Concept 2: cooking
	if strings.Contains(lower, "cake") || strings.Contains(lower, "baking") ||
		strings.Contains(lower, "recipe") || strings.Contains(lower, "chocolate") {
		v[2] = 0.9
		v[3] = 0.1
	}

	// Concept 3: programming
	if strings.Contains(lower, "golang") || strings.Contains(lower, "code") ||
		strings.Contains(lower, "software") || strings.Contains(lower, "programming") {
		v[4] = 0.9
		v[5] = 0.1
	}

	// Default non-zero fallback for any text
	if v[0] == 0 && v[2] == 0 && v[4] == 0 {
		v[dim-1] = 0.5
	}
	return v
}

func TestRepository_HybridSearch_NilEmbedder_FallsBackToFTS(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	repo, err := NewRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}

	obs := []Observation{
		{TopicKey: "arch.go", Type: "architecture", Content: "Clean architecture in Go systems", Importance: 4},
		{TopicKey: "food.recipe", Type: "preference", Content: "Chocolate cake baking steps", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	if err != nil {
		t.Fatalf("SaveObservations error = %v", err)
	}

	results, err := repo.Search(ctx, "architecture", 10)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Content, "Clean architecture") {
		t.Errorf("content does not contain 'Clean architecture': %s", results[0].Content)
	}

	results, err = repo.Search(ctx, "microservices", 10)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRepository_HybridSearch_EmbedderError_FallsBackToFTS(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)
	mock.errToReturn = errors.New("ollama connection refused")

	repo, err := NewRepository(ctx, dbPath, WithEmbedder(mock))
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}

	obs := []Observation{
		{TopicKey: "tech.go", Type: "fact", Content: "Golang concurrency with channels", Importance: 5},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	if err != nil {
		t.Fatalf("SaveObservations error = %v", err)
	}

	results, err := repo.Search(ctx, "channels", 10)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Content, "Golang concurrency") {
		t.Errorf("content does not contain 'Golang concurrency': %s", results[0].Content)
	}
}

func TestRepository_HybridSearch_TrueHybrid_SemanticAndFTS(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := NewRepository(ctx, dbPath, WithEmbedder(mock))
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}

	obs := []Observation{
		{TopicKey: "auto.maint", Type: "fact", Content: "Car maintenance checklist", Importance: 5},
		{TopicKey: "auto.repair", Type: "fact", Content: "Automobile repair fundamentals", Importance: 4},
		{TopicKey: "food.cake", Type: "fact", Content: "Chocolate cake baking", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	if err != nil {
		t.Fatalf("SaveObservations error = %v", err)
	}

	repo.FlushEmbeddings(ctx)

	results, err := repo.Search(ctx, "maintenance", 10)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	if !strings.Contains(results[0].Content, "maintenance checklist") {
		t.Errorf("result[0] does not contain 'maintenance checklist': %s", results[0].Content)
	}
	if !strings.Contains(results[1].Content, "Automobile repair") {
		t.Errorf("result[1] does not contain 'Automobile repair': %s", results[1].Content)
	}
}

func TestRepository_HybridSearch_ObservationUpdateReindexesVector(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := NewRepository(ctx, dbPath, WithEmbedder(mock))
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}

	obs1 := []Observation{
		{TopicKey: "dynamic.topic", Type: "fact", Content: "Automobile repair shop", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs1)
	if err != nil {
		t.Fatalf("SaveObservations error = %v", err)
	}
	repo.FlushEmbeddings(ctx)

	res, err := repo.Search(ctx, "car maintenance", 10)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}

	obs2 := []Observation{
		{TopicKey: "dynamic.topic", Type: "fact", Content: "Chocolate cake baking recipe", Importance: 4},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs2)
	if err != nil {
		t.Fatalf("SaveObservations error = %v", err)
	}
	repo.FlushEmbeddings(ctx)

	res, err = repo.Search(ctx, "car maintenance", 10)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 results, got %d", len(res))
	}

	res, err = repo.Search(ctx, "cake recipe", 10)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	if !strings.Contains(res[0].Content, "Chocolate cake baking recipe") {
		t.Errorf("content does not contain 'Chocolate cake baking recipe': %s", res[0].Content)
	}
}

func TestRepository_HybridSearch_LimitCap(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := NewRepository(ctx, dbPath, WithEmbedder(mock))
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}

	obs := []Observation{
		{TopicKey: "code.1", Type: "fact", Content: "Golang concurrency code", Importance: 3},
		{TopicKey: "code.2", Type: "fact", Content: "Golang memory optimization code", Importance: 3},
		{TopicKey: "code.3", Type: "fact", Content: "Golang software architecture code", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	if err != nil {
		t.Fatalf("SaveObservations error = %v", err)
	}
	repo.FlushEmbeddings(ctx)

	results, err := repo.Search(ctx, "Golang code", 2)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestRepository_ConcurrentSearchAndIndexing_Race(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := NewRepository(ctx, dbPath, WithEmbedder(mock))
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	if err != nil {
		t.Fatalf("CreateConversation error = %v", err)
	}

	var wg sync.WaitGroup
	workers := 8
	iterations := 25

	for w := 0; w < workers; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if i%2 == 0 {
					_ = repo.SaveObservations(ctx, conv.ID, []Observation{
						{
							TopicKey:   fmt.Sprintf("worker.%d.topic.%d", workerID, i),
							Type:       "fact",
							Content:    fmt.Sprintf("Worker %d software code optimization step %d", workerID, i),
							Importance: 3,
						},
					})
				} else {
					_, _ = repo.Search(ctx, "software code", 5)
				}
			}
		}()
	}

	wg.Wait()
	repo.FlushEmbeddings(ctx)
}

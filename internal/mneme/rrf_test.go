package mneme

import (
	"testing"
)

func TestReciprocalRankFusion_EmptyInputs(t *testing.T) {
	t.Parallel()

	res := ReciprocalRankFusion(nil, 60)
	if len(res) != 0 {
		t.Errorf("expected empty, got %v", res)
	}

	res = ReciprocalRankFusion([][]SearchResult{}, 60)
	if len(res) != 0 {
		t.Errorf("expected empty, got %v", res)
	}

	res = ReciprocalRankFusion([][]SearchResult{{}, {}}, 60)
	if len(res) != 0 {
		t.Errorf("expected empty, got %v", res)
	}
}

func TestReciprocalRankFusion_SingleList(t *testing.T) {
	t.Parallel()

	list := []SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "first"},
		{DocType: "observation", DocID: "obs-2", Content: "second"},
		{DocType: "message", DocID: "msg-1", Content: "third"},
	}

	res := ReciprocalRankFusion([][]SearchResult{list}, 60)
	if len(res) != 3 {
		t.Fatalf("expected len 3, got %d", len(res))
	}
	if res[0].DocID != "obs-1" || res[1].DocID != "obs-2" || res[2].DocID != "msg-1" {
		t.Errorf("unexpected results ordering: %v", res)
	}
}

func TestReciprocalRankFusion_MultiListScoring(t *testing.T) {
	t.Parallel()

	fts := []SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "shared top"},
		{DocType: "observation", DocID: "obs-2", Content: "fts only"},
	}
	vec := []SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "shared top"},
		{DocType: "observation", DocID: "obs-3", Content: "vector only"},
	}

	res := ReciprocalRankFusion([][]SearchResult{fts, vec}, 60)
	if len(res) != 3 {
		t.Fatalf("expected len 3, got %d", len(res))
	}
	if res[0].DocID != "obs-1" || res[1].DocID != "obs-2" || res[2].DocID != "obs-3" {
		t.Errorf("unexpected results: %v", res)
	}
}

func TestReciprocalRankFusion_DeterministicTieBreaking(t *testing.T) {
	t.Parallel()

	listA := []SearchResult{
		{DocType: "observation", DocID: "doc-b", Content: "beta"},
	}
	listB := []SearchResult{
		{DocType: "observation", DocID: "doc-a", Content: "alpha"},
	}

	res := ReciprocalRankFusion([][]SearchResult{listA, listB}, 60)
	if len(res) != 2 {
		t.Fatalf("expected len 2, got %d", len(res))
	}
	if res[0].DocID != "doc-a" || res[1].DocID != "doc-b" {
		t.Errorf("doc-a should precede doc-b on equal score due to doc_id sorting, got %v", res)
	}
}

func TestReciprocalRankFusion_Deduplication(t *testing.T) {
	t.Parallel()

	list := []SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "first occurrence"},
		{DocType: "observation", DocID: "obs-1", Content: "second occurrence"},
	}

	res := ReciprocalRankFusion([][]SearchResult{list}, 60)
	if len(res) != 1 {
		t.Fatalf("expected len 1, got %d", len(res))
	}
	if res[0].DocID != "obs-1" {
		t.Errorf("expected obs-1, got %s", res[0].DocID)
	}
}

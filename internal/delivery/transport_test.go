package delivery

import (
	"context"
	"errors"
	"testing"
)

func TestMockTransport_PushAndCreatePR(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	// 1. Push branch
	err := mock.PushBranch(ctx, "feature/auth")
	if err != nil {
		t.Fatalf("unexpected push error: %v", err)
	}

	if len(mock.PushedBranches) != 1 || mock.PushedBranches[0] != "feature/auth" {
		t.Errorf("expected pushed branch 'feature/auth', got %v", mock.PushedBranches)
	}

	// 2. Create PR
	item := DeliveryItem{
		ID:         "del-1",
		Branch:     "feature/auth",
		BaseBranch: "main",
		PRTitle:    "feat: add auth",
		PRBody:     "description",
	}

	url, err := mock.CreatePullRequest(ctx, item)
	if err != nil {
		t.Fatalf("unexpected create PR error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty PR URL")
	}
	if len(mock.CreatedPRs) != 1 || mock.CreatedPRs[0].ID != "del-1" {
		t.Errorf("expected created PR del-1, got %+v", mock.CreatedPRs)
	}
}

func TestMockTransport_CustomErrors(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	expectedPushErr := errors.New("git remote unreachable")
	mock.PushErr = expectedPushErr

	err := mock.PushBranch(ctx, "feature/auth")
	if !errors.Is(err, expectedPushErr) {
		t.Errorf("expected push error %v, got %v", expectedPushErr, err)
	}

	expectedPRErr := errors.New("gh auth token invalid")
	mock.CreatePRErr = expectedPRErr

	_, err = mock.CreatePullRequest(ctx, DeliveryItem{ID: "del-1"})
	if !errors.Is(err, expectedPRErr) {
		t.Errorf("expected PR error %v, got %v", expectedPRErr, err)
	}
}

func TestExecTransport_Structure(t *testing.T) {
	execT := NewExecTransport("/tmp/test-repo")
	if execT == nil {
		t.Fatal("expected non-nil ExecTransport")
	}
	if execT.WorkDir() != "/tmp/test-repo" {
		t.Errorf("expected workdir /tmp/test-repo, got %s", execT.WorkDir())
	}
}

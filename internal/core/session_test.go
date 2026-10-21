package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type mockRepoSession struct {
	mu        sync.Mutex
	sessionID string
}

func (m *mockRepoSession) SetSessionID(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionID = id
}

func (m *mockRepoSession) GetSessionID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionID
}

func TestSessionManager_Unify(t *testing.T) {
	repo := &mockRepoSession{}
	sm := NewSessionManager(SessionUnify, nil, repo)
	ctx := context.Background()

	err := sm.Route(ctx, "telegram", "hello", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id1 := sm.GetSessionID("telegram")
	if id1 == "" {
		t.Fatalf("expected session ID, got empty")
	}

	err = sm.Route(ctx, "discord", "world", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id2 := sm.GetSessionID("discord")
	if id1 != id2 {
		t.Errorf("expected unified session ID %q, got %q for discord", id1, id2)
	}
}

func TestSessionManager_Isolate(t *testing.T) {
	repo := &mockRepoSession{}
	sm := NewSessionManager(SessionIsolate, nil, repo)
	ctx := context.Background()

	_ = sm.Route(ctx, "telegram", "hello", "alice")
	_ = sm.Route(ctx, "discord", "world", "bob")

	idTel := sm.GetSessionID("telegram")
	idDisc := sm.GetSessionID("discord")

	if idTel == "" || idDisc == "" {
		t.Fatalf("expected non-empty session IDs, got tel=%q disc=%q", idTel, idDisc)
	}
	if idTel == idDisc {
		t.Errorf("expected different session IDs in isolate mode, got both %q", idTel)
	}
}

func TestSessionManager_Status_ShortSessionID(t *testing.T) {
	repo := &mockRepoSession{}
	sm := NewSessionManager(SessionUnify, nil, repo)
	ctx := context.Background()

	// Inject a custom creator that returns short IDs (< 12 chars)
	sm.SetSessionCreator(func(ctx context.Context, name string) (string, error) {
		return "short-1", nil
	})

	_ = sm.Route(ctx, "cli", "test", "")

	// This used to panic with slice bounds out of range [:12]
	status := sm.Status()
	if status == "" {
		t.Errorf("expected non-empty status")
	}

	// Test isolate mode with short ID
	sm.SetMode(SessionIsolate)
	_ = sm.Route(ctx, "cli", "test", "")
	statusIsolate := sm.Status()
	if statusIsolate == "" {
		t.Errorf("expected non-empty status for isolate")
	}
}

func TestSessionManager_PlatformTiers(t *testing.T) {
	sm := NewSessionManager(SessionUnify, nil, nil)
	sm.SetPlatformTier("slack", TierRead)
	sm.SetPlatformTier("cli", TierFull)

	tiers := map[string]PolicyTier{
		"telegram": TierSandbox,
	}
	sm.SetPlatformTiers(tiers)

	if sm.platformTiers["telegram"] != TierSandbox {
		t.Errorf("expected TierSandbox for telegram, got %s", sm.platformTiers["telegram"])
	}
}

func TestSessionManager_Concurrency(t *testing.T) {
	repo := &mockRepoSession{}
	sm := NewSessionManager(SessionUnify, nil, repo)
	ctx := context.Background()

	var wg sync.WaitGroup
	platforms := []string{"telegram", "discord", "slack", "cli", "web"}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		p := platforms[i%len(platforms)]
		go func(platform string, n int) {
			defer wg.Done()
			_ = sm.Route(ctx, platform, fmt.Sprintf("msg %d", n), "user")
			_ = sm.Mode()
			_ = sm.GetSessionID(platform)
			_ = sm.Status()
		}(p, i)
	}

	wg.Wait()
}

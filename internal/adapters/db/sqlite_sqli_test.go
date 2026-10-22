package db

import (
	"context"
	"path/filepath"
	"testing"

	"gaia/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteRepo_SQLInjectionSafety(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gaia.db")
	repo, err := NewSQLiteRepoWithPath(dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Save a message in normal session
	repo.SetSessionID("session-legit")
	err = repo.SaveMessage(ctx, domain.Message{Role: domain.RoleUser, Content: "Secret legit message"})
	require.NoError(t, err)

	count, err := repo.GetMessageCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Set malicious session ID trying to inject OR '1'='1
	maliciousSession := "' OR '1'='1"
	repo.SetSessionID(maliciousSession)

	// In safe parameterized queries, this query will search for session_id = "' OR '1'='1"
	// and will return 0 messages instead of matching all sessions.
	injectedCount, err := repo.GetMessageCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, injectedCount, "SQL injection payload should not match legitimate messages")

	// Same for GetHistoryFrom and GetLastMessages
	msgs, err := repo.GetLastMessages(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, msgs)

	// Malicious ClearMessages should NOT delete the legitimate session's message
	err = repo.ClearMessages(ctx)
	require.NoError(t, err)

	repo.SetSessionID("session-legit")
	countAfter, err := repo.GetMessageCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, countAfter, "Legitimate session message should not have been wiped")
}

func TestSQLiteRepo_Concurrency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrency.db")
	repo, err := NewSQLiteRepoWithPath(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 50; i++ {
			repo.SetSessionID("session-a")
			repo.SetSessionID("session-b")
		}
		close(done)
	}()

	for {
		select {
		case <-done:
			return
		default:
			_ = repo.activeSessionID()
			_ = repo.SaveMessage(ctx, domain.Message{Role: domain.RoleUser, Content: "ping"})
			_, _ = repo.GetHistory(ctx, 5)
			_, _ = repo.GetMessageCount(ctx)
		}
	}
}

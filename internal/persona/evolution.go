package persona

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"gaia/internal/mneme"
)

// evolutionLimit caps how many user-model rows feed the layer.
const evolutionLimit = 5

// userKeyPrefix is trimmed from row keys for readability inside the layer.
const userKeyPrefix = "user/"

// Evolution is the derived persona layer: a guidance block
// assembled from the top user-model rows by confidence. It is computed state —
// freezing hides the layer, resetting clears the underlying rows, and neither
// ever touches SOUL.md.
type Evolution struct {
	mu     sync.RWMutex
	repo   mneme.Store
	logger *slog.Logger
	frozen bool
}

// NewEvolution returns an active (unfrozen) Evolution backed by repo.
func NewEvolution(repo mneme.Store, logger *slog.Logger) *Evolution {
	if logger == nil {
		logger = slog.Default()
	}
	return &Evolution{repo: repo, logger: logger}
}

// Freeze excludes the layer from prompts for the rest of the session.
func (e *Evolution) Freeze() {
	e.mu.Lock()
	e.frozen = true
	e.mu.Unlock()
}

// Frozen reports whether the layer is currently excluded.
func (e *Evolution) Frozen() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.frozen
}

// Layer assembles the guidance block from up to five user-model rows ordered
// by confidence. It returns empty text when frozen or when nothing has been
// learned yet.
func (e *Evolution) Layer(ctx context.Context) string {
	if ctx != nil && ctx.Err() != nil {
		return ""
	}
	if e.Frozen() || e.repo == nil {
		return ""
	}

	rows, err := e.repo.UserModelRows(ctx, evolutionLimit)
	if err != nil {
		e.logger.Warn("persona: reading user model failed; skipping evolution layer", "error", err)
		return ""
	}
	if len(rows) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("How to work with this user (learned so far):\n")
	for _, r := range rows {
		key := strings.TrimPrefix(r.Key, userKeyPrefix)
		fmt.Fprintf(&b, "- %s: %s\n", key, r.Value)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Reset clears every user-model row. The rows are derived data — rebuildable
// from observations via AggregateUserModel — so this returns the evolution
// layer to its seed state without touching long-term memory.
func (e *Evolution) Reset(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if e.repo == nil {
		return nil
	}
	if err := e.repo.ClearUserModel(ctx); err != nil {
		return fmt.Errorf("resetting persona evolution: %w", err)
	}
	return nil
}

// Status is what /persona status renders.
type Status struct {
	Frozen bool
	Rows   int
	Active bool // an unfrozen evolution with at least one learned row
}

// Status reports the current evolution mode and how many rows it draws on.
func (e *Evolution) Status(ctx context.Context) (Status, error) {
	if ctx != nil && ctx.Err() != nil {
		return Status{}, ctx.Err()
	}
	frozen := e.Frozen()
	rows := 0
	if !frozen && e.repo != nil {
		all, err := e.repo.UserModelRows(ctx, 0)
		if err != nil {
			return Status{}, fmt.Errorf("reading user model status: %w", err)
		}
		rows = len(all)
	}
	return Status{Frozen: frozen, Rows: rows, Active: !frozen && rows > 0}, nil
}

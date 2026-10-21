package subagents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gaia/internal/mneme"
	"github.com/google/uuid"
)

// ephemeralRepository implements mneme.Store for isolated child subagent execution.
// It keeps conversation, message, and snapshot state in-memory to prevent pollution
// of the parent persistent storage, while proxying shared knowledge (search,
// observations, skills, user model, audit) to the parent repository.
type ephemeralRepository struct {
	parent    mneme.Store
	convs     map[string]*mneme.Conversation
	latest    *mneme.Conversation
	messages  map[string][]mneme.Message
	snapshots map[string][]mneme.Snapshot
	mu        sync.RWMutex
}

// NewEphemeralRepository creates an isolated in-memory repository wrapping parent.
func NewEphemeralRepository(parent mneme.Store) mneme.Store {
	return &ephemeralRepository{
		parent:    parent,
		convs:     make(map[string]*mneme.Conversation),
		messages:  make(map[string][]mneme.Message),
		snapshots: make(map[string][]mneme.Snapshot),
	}
}

func (r *ephemeralRepository) CreateConversation(ctx context.Context, title string) (*mneme.Conversation, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("subagent-%s", uuid.NewString())
	conv := &mneme.Conversation{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.convs[id] = conv
	r.latest = conv
	return conv, nil
}

func (r *ephemeralRepository) LatestConversation(ctx context.Context) (*mneme.Conversation, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.latest == nil {
		return nil, mneme.ErrNotFound
	}
	c := *r.latest
	return &c, nil
}

func (r *ephemeralRepository) AppendMessage(ctx context.Context, convID string, msg mneme.Message) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.messages[convID] = append(r.messages[convID], msg)
	if c, ok := r.convs[convID]; ok {
		c.UpdatedAt = time.Now()
		c.MessageCount++
	}
	return nil
}

func (r *ephemeralRepository) Messages(ctx context.Context, convID string, limit int) ([]mneme.Message, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	msgs := r.messages[convID]
	if len(msgs) == 0 {
		return []mneme.Message{}, nil
	}
	if limit <= 0 || limit >= len(msgs) {
		copied := make([]mneme.Message, len(msgs))
		copy(copied, msgs)
		return copied, nil
	}

	start := len(msgs) - limit
	copied := make([]mneme.Message, limit)
	copy(copied, msgs[start:])
	return copied, nil
}

func (r *ephemeralRepository) Search(ctx context.Context, query string, limit int) ([]mneme.SearchResult, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.Search(ctx, query, limit)
}

func (r *ephemeralRepository) SaveObservations(ctx context.Context, convID string, obs []mneme.Observation) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.SaveObservations(ctx, convID, obs)
}

func (r *ephemeralRepository) Observations(ctx context.Context, limit int) ([]mneme.Observation, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.Observations(ctx, limit)
}

func (r *ephemeralRepository) ObservationsByProject(ctx context.Context, project string, limit int) ([]mneme.Observation, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.ObservationsByProject(ctx, project, limit)
}

func (r *ephemeralRepository) UpdateConversationSummary(ctx context.Context, convID, summary string) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.convs[convID]
	if !ok {
		return mneme.ErrNotFound
	}
	c.Summary = summary
	return nil
}

func (r *ephemeralRepository) UpsertUserModel(ctx context.Context, rows []mneme.UserModel) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.UpsertUserModel(ctx, rows)
}

func (r *ephemeralRepository) UserModelRows(ctx context.Context, limit int) ([]mneme.UserModel, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.UserModelRows(ctx, limit)
}

func (r *ephemeralRepository) ClearUserModel(ctx context.Context) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.ClearUserModel(ctx)
}

func (r *ephemeralRepository) RecordSessionEvent(ctx context.Context, sessionID, kind, payload string) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.RecordSessionEvent(ctx, sessionID, kind, payload)
}

func (r *ephemeralRepository) AppendAudit(ctx context.Context, entry mneme.AuditEntry) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.AppendAudit(ctx, entry)
}

func (r *ephemeralRepository) AuditTail(ctx context.Context, n int) ([]mneme.AuditEntry, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.AuditTail(ctx, n)
}

func (r *ephemeralRepository) SaveSkill(ctx context.Context, skill mneme.Skill) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.SaveSkill(ctx, skill)
}

func (r *ephemeralRepository) ListSkills(ctx context.Context) ([]mneme.Skill, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.ListSkills(ctx)
}

func (r *ephemeralRepository) RecordSkillUsage(ctx context.Context, name string) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.RecordSkillUsage(ctx, name)
}

func (r *ephemeralRepository) DeleteSkill(ctx context.Context, name string) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.DeleteSkill(ctx, name)
}

func (r *ephemeralRepository) ListConversations(ctx context.Context, limit, offset int) ([]mneme.Conversation, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []mneme.Conversation
	count := 0
	skipped := 0

	for _, c := range r.convs {
		if skipped < offset {
			skipped++
			continue
		}
		result = append(result, *c)
		count++
		if limit > 0 && count >= limit {
			break
		}
	}
	return result, nil
}

func (r *ephemeralRepository) GetConversation(ctx context.Context, id string) (*mneme.Conversation, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.convs[id]
	if !ok {
		return nil, mneme.ErrNotFound
	}
	copyVal := *c
	return &copyVal, nil
}

func (r *ephemeralRepository) RenameConversation(ctx context.Context, id, title string) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.convs[id]
	if !ok {
		return mneme.ErrNotFound
	}
	c.Title = title
	return nil
}

func (r *ephemeralRepository) DeleteConversation(ctx context.Context, id string) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.convs[id]; !ok {
		return mneme.ErrNotFound
	}
	delete(r.convs, id)
	delete(r.messages, id)
	delete(r.snapshots, id)
	if r.latest != nil && r.latest.ID == id {
		r.latest = nil
	}
	return nil
}

func (r *ephemeralRepository) CreateSnapshot(ctx context.Context, convID string) (*mneme.Snapshot, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.convs[convID]
	if !ok {
		return nil, mneme.ErrNotFound
	}

	snap := mneme.Snapshot{
		ID:             fmt.Sprintf("snap-%s", uuid.NewString()),
		ConversationID: convID,
		Title:          c.Title,
		Summary:        c.Summary,
		CreatedAt:      time.Now(),
	}
	r.snapshots[convID] = append(r.snapshots[convID], snap)
	return &snap, nil
}

func (r *ephemeralRepository) ListSnapshots(ctx context.Context, convID string) ([]mneme.Snapshot, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	snaps := r.snapshots[convID]
	if len(snaps) == 0 {
		return []mneme.Snapshot{}, nil
	}
	copied := make([]mneme.Snapshot, len(snaps))
	copy(copied, snaps)
	return copied, nil
}

func (r *ephemeralRepository) Close() error {
	return nil
}

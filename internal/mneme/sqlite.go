package mneme

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// defaultTitle matches the conversations.title DEFAULT in 0001_init.sql.
const defaultTitle = "New session"

// timeLayout is a fixed-width RFC3339 layout. Fixed-width fractional seconds
// (nine digits, trailing zeros kept) are required so that lexicographic
// ordering of the TEXT timestamp columns equals chronological ordering; the
// trimmed form of RFC3339Nano does not sort correctly.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

const conversationCols = `id, title, created_at, updated_at, summary, message_count, goal, next_steps, relevant_files`
const observationCols = `id, topic_key, type, content, importance, project, workspace_dir, status, created_at, updated_at, source_ref`

// Repository is the SQLite + FTS5 implementation of Repository.
type Repository struct {
	db       *sql.DB
	embedder Embedder
	logger   *slog.Logger

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	embedChan chan embeddingBatchMsg
	embedMu   sync.Mutex
	closed    bool
}

var _ Store = (*Repository)(nil)

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// NewRepository opens (or creates) the SQLite database at path, applies the
// embedded migrations, and returns a ready Repository. The caller owns the
// returned Repository and must call Close when done.
func NewRepository(ctx context.Context, path string, opts ...Option) (*Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := configureDB(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := applyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	workerCtx, cancel := context.WithCancel(context.Background())
	r := &Repository{
		db:        db,
		logger:    slog.Default(),
		ctx:       workerCtx,
		cancel:    cancel,
		embedChan: make(chan embeddingBatchMsg, 128),
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.embedder != nil {
		r.wg.Add(1)
		go r.embeddingWorker()
	}

	return r, nil
}

// CreateConversation persists a new conversation and returns it. An empty
// title falls back to the default.
func (r *Repository) CreateConversation(ctx context.Context, title string) (*Conversation, error) {
	if title == "" {
		title = defaultTitle
	}
	now := time.Now().UTC()
	conv := &Conversation{
		ID:            uuid.NewString(),
		Title:         title,
		CreatedAt:     now,
		UpdatedAt:     now,
		Summary:       "",
		Goal:          "",
		NextSteps:     "",
		RelevantFiles: "{}",
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO conversations (`+conversationCols+`)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)`,
		conv.ID, conv.Title, formatTime(conv.CreatedAt), formatTime(conv.UpdatedAt), conv.Summary,
		conv.Goal, conv.NextSteps, conv.RelevantFiles)
	if err != nil {
		return nil, fmt.Errorf("creating conversation: %w", err)
	}
	return conv, nil
}

// LatestConversation returns the most recently updated conversation, or
// ErrNotFound when none exists.
func (r *Repository) LatestConversation(ctx context.Context) (*Conversation, error) {
	conv, err := scanConversation(r.db.QueryRowContext(ctx,
		`SELECT `+conversationCols+`
		 FROM conversations
		 ORDER BY updated_at DESC, id DESC
		 LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading latest conversation: %w", err)
	}
	return conv, nil
}

// AppendMessage persists a message, syncs its FTS5 row, and bumps the
// conversation's updated_at and message_count — all in one transaction.
func (r *Repository) AppendMessage(ctx context.Context, convID string, msg Message) error {
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning append transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, content, created_at)
		 VALUES (?, ?, ?, ?)`,
		convID, string(msg.Role), msg.Content, formatTime(now))
	if err != nil {
		return fmt.Errorf("inserting message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("reading message id: %w", err)
	}

	for _, att := range msg.Attachments {
		attID := uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO attachments (id, message_id, type, mime_type, data, url, name, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			attID, strconv.FormatInt(id, 10), att.Type, att.MimeType, att.Data, att.URL, att.Name, formatTime(now)); err != nil {
			return fmt.Errorf("inserting attachment: %w", err)
		}
	}

	if err := insertFTSRow(ctx, tx, docTypeMessage, strconv.FormatInt(id, 10), msg.Content); err != nil {
		return fmt.Errorf("indexing message: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE conversations
		 SET updated_at = ?, message_count = message_count + 1
		 WHERE id = ?`,
		formatTime(now), convID); err != nil {
		return fmt.Errorf("updating conversation: %w", err)
	}

	return tx.Commit()
}

// Messages returns the conversation's messages in chronological order.
func (r *Repository) Messages(ctx context.Context, convID string, limit int) ([]Message, error) {
	const cols = `id, conversation_id, role, content, created_at`

	query := `SELECT ` + cols + ` FROM messages WHERE conversation_id = ? ORDER BY id ASC`
	args := []any{convID}

	if limit > 0 {
		query = `SELECT ` + cols + ` FROM (
			SELECT ` + cols + ` FROM messages WHERE conversation_id = ? ORDER BY id DESC LIMIT ?
		) ORDER BY id ASC`
		args = []any{convID, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("loading messages: %w", err)
	}
	defer rows.Close()

	msgs := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}
	if len(msgs) > 0 {
		if err := r.populateAttachments(ctx, msgs); err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

func (r *Repository) populateAttachments(ctx context.Context, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	placeholders := make([]string, len(msgs))
	args := make([]any, len(msgs))
	msgMap := make(map[string]int, len(msgs))
	for i, m := range msgs {
		placeholders[i] = "?"
		msgIDStr := strconv.FormatInt(m.ID, 10)
		args[i] = msgIDStr
		msgMap[msgIDStr] = i
	}

	query := `SELECT id, message_id, type, mime_type, data, url, name FROM attachments WHERE message_id IN (` +
		strings.Join(placeholders, ",") + `) ORDER BY rowid ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("loading attachments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			attID string
			msgID string
			att   Attachment
		)
		if err := rows.Scan(&attID, &msgID, &att.Type, &att.MimeType, &att.Data, &att.URL, &att.Name); err != nil {
			return fmt.Errorf("scanning attachment: %w", err)
		}
		if idx, ok := msgMap[msgID]; ok {
			msgs[idx].Attachments = append(msgs[idx].Attachments, att)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating attachments: %w", err)
	}
	return nil
}

// Search returns full-text matches across messages and observations.
func (r *Repository) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []SearchResult{}, nil
	}

	ftsResults, err := r.searchMatches(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("lexical search: %w", err)
	}

	if r.embedder == nil {
		return ftsResults, nil
	}

	queryVec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		r.logger.Warn("hybrid search: embedder failed, falling back to FTS5", "query", query, "error", err)
		return ftsResults, nil
	}
	if len(queryVec) == 0 {
		return ftsResults, nil
	}

	vecResults, err := r.searchVectors(ctx, queryVec, limit)
	if err != nil {
		r.logger.Warn("hybrid search: vector search failed, falling back to FTS5", "error", err)
		return ftsResults, nil
	}

	merged := ReciprocalRankFusion([][]SearchResult{ftsResults, vecResults}, DefaultRRFK)
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// Close releases the underlying database handle and stops background workers.
func (r *Repository) Close() error {
	r.embedMu.Lock()
	if !r.closed {
		r.closed = true
		if r.embedChan != nil {
			close(r.embedChan)
		}
	}
	r.embedMu.Unlock()

	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	return r.db.Close()
}

func clampImportance(v int) int {
	switch {
	case v == 0:
		return 3
	case v < 1:
		return 1
	case v > 5:
		return 5
	default:
		return v
	}
}

// SaveObservations persists a batch of observations, upserting on topic_key.
func (r *Repository) SaveObservations(ctx context.Context, convID string, obs []Observation) error {
	if len(obs) == 0 {
		return nil
	}

	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning observation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	jobs := make([]embeddingJob, 0, len(obs))
	for _, o := range obs {
		id, err := r.upsertObservation(ctx, tx, convID, o, now)
		if err != nil {
			return err
		}
		jobs = append(jobs, embeddingJob{
			docType: docTypeObservation,
			docID:   id,
			content: o.Content,
		})
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing observations: %w", err)
	}

	if r.embedder != nil && len(jobs) > 0 {
		r.embedMu.Lock()
		if !r.closed {
			select {
			case r.embedChan <- embeddingBatchMsg{jobs: jobs}:
			default:
				r.logger.Warn("memory: embedding queue full, dropping background job", "count", len(jobs))
			}
		}
		r.embedMu.Unlock()
	}

	return nil
}

func (r *Repository) upsertObservation(ctx context.Context, tx *sql.Tx, convID string, o Observation, now time.Time) (string, error) {
	if strings.TrimSpace(o.TopicKey) == "" {
		return "", fmt.Errorf("observation has empty topic_key")
	}

	importance := clampImportance(o.Importance)
	project := o.Project
	if project == "" {
		project = "global"
	}
	status := o.Status
	if status == "" {
		status = StatusActive
	}
	nowStr := formatTime(now)

	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM observations WHERE topic_key = ?`, o.TopicKey).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO observations (`+observationCols+`)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, o.TopicKey, o.Type, o.Content, importance, project, o.WorkspaceDir, status, nowStr, nowStr, convID); err != nil {
			return "", fmt.Errorf("inserting observation %q: %w", o.TopicKey, err)
		}
	case err != nil:
		return "", fmt.Errorf("looking up observation %q: %w", o.TopicKey, err)
	default:
		if _, err := tx.ExecContext(ctx,
			`UPDATE observations
			 SET type = ?, content = ?, importance = ?, project = ?, workspace_dir = ?, status = ?, source_ref = ?, updated_at = ?
			 WHERE id = ?`,
			o.Type, o.Content, importance, project, o.WorkspaceDir, status, convID, nowStr, id); err != nil {
			return "", fmt.Errorf("updating observation %q: %w", o.TopicKey, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings WHERE doc_type = ? AND doc_id = ?`, docTypeObservation, id); err != nil {
		return "", fmt.Errorf("deleting observation embedding: %w", err)
	}

	if err := deleteFTSRow(ctx, tx, docTypeObservation, id); err != nil {
		return "", fmt.Errorf("deleting observation FTS row: %w", err)
	}
	if err := insertFTSRow(ctx, tx, docTypeObservation, id, o.Content); err != nil {
		return "", fmt.Errorf("indexing observation: %w", err)
	}
	return id, nil
}

// Observations returns the most recently updated observations, newest first.
func (r *Repository) Observations(ctx context.Context, limit int) ([]Observation, error) {
	query := `SELECT ` + observationCols + `
	          FROM observations
	          ORDER BY updated_at DESC, id DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("loading observations: %w", err)
	}
	defer rows.Close()

	obs := []Observation{}
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating observations: %w", err)
	}
	return obs, nil
}

// ObservationsByProject queries observations belonging to a specific project.
func (r *Repository) ObservationsByProject(ctx context.Context, project string, limit int) ([]Observation, error) {
	query := `SELECT ` + observationCols + `
	          FROM observations
	          WHERE project = ?
	          ORDER BY updated_at DESC, id DESC`
	args := []any{project}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("loading observations by project %q: %w", project, err)
	}
	defer rows.Close()

	obs := []Observation{}
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating observations: %w", err)
	}
	return obs, nil
}

// UpdateObservationStatus updates the lifecycle status ('active' or 'needs_review') of an observation.
func (r *Repository) UpdateObservationStatus(ctx context.Context, id, status string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE observations SET status = ?, updated_at = ? WHERE id = ?`,
		status, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("updating observation status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("updating observation status: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateConversationSummary writes a conversation's summary WITHOUT touching updated_at.
func (r *Repository) UpdateConversationSummary(ctx context.Context, convID, summary string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE conversations SET summary = ? WHERE id = ?`, summary, convID); err != nil {
		return fmt.Errorf("updating conversation summary: %w", err)
	}
	return nil
}

// UpdateConversationSession updates the goal, next_steps, and relevant_files of a conversation.
func (r *Repository) UpdateConversationSession(ctx context.Context, convID, goal, nextSteps, relevantFiles string) error {
	if relevantFiles == "" {
		relevantFiles = "{}"
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE conversations SET goal = ?, next_steps = ?, relevant_files = ? WHERE id = ?`,
		goal, nextSteps, relevantFiles, convID)
	if err != nil {
		return fmt.Errorf("updating conversation session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("updating conversation session: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertUserModel persists user-model rows, upserting on key.
func (r *Repository) UpsertUserModel(ctx context.Context, rows []UserModel) error {
	if len(rows) == 0 {
		return nil
	}

	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning user model transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, u := range rows {
		if err := r.upsertUserModel(ctx, tx, u, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing user model: %w", err)
	}
	return nil
}

func (r *Repository) upsertUserModel(ctx context.Context, tx *sql.Tx, u UserModel, now time.Time) error {
	if strings.TrimSpace(u.Key) == "" {
		return fmt.Errorf("user model row has empty key")
	}

	nowStr := formatTime(now)

	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM user_model WHERE key = ?`, u.Key).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_model (id, key, value, confidence, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			id, u.Key, u.Value, u.Confidence, nowStr); err != nil {
			return fmt.Errorf("inserting user model %q: %w", u.Key, err)
		}
	case err != nil:
		return fmt.Errorf("looking up user model %q: %w", u.Key, err)
	default:
		if _, err := tx.ExecContext(ctx,
			`UPDATE user_model SET value = ?, confidence = ?, updated_at = ? WHERE id = ?`,
			u.Value, u.Confidence, nowStr, id); err != nil {
			return fmt.Errorf("updating user model %q: %w", u.Key, err)
		}
	}
	return nil
}

// UserModelRows returns up to limit user-model rows ordered by confidence descending, then key ascending.
func (r *Repository) UserModelRows(ctx context.Context, limit int) ([]UserModel, error) {
	q := `SELECT id, key, value, confidence, updated_at FROM user_model`
	args := []any{}
	if limit > 0 {
		q += ` ORDER BY confidence DESC, key ASC LIMIT ?`
		args = append(args, limit)
	} else {
		q += ` ORDER BY confidence DESC, key ASC`
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing user model: %w", err)
	}
	defer rows.Close()

	var out []UserModel
	for rows.Next() {
		var u UserModel
		var updatedStr string
		if err := rows.Scan(&u.ID, &u.Key, &u.Value, &u.Confidence, &updatedStr); err != nil {
			return nil, fmt.Errorf("scanning user model: %w", err)
		}
		updated, err := parseTime(updatedStr)
		if err != nil {
			return nil, fmt.Errorf("parsing user model updated_at: %w", err)
		}
		u.UpdatedAt = updated
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user model: %w", err)
	}
	return out, nil
}

// ClearUserModel deletes every user-model row.
func (r *Repository) ClearUserModel(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM user_model`); err != nil {
		return fmt.Errorf("clearing user model: %w", err)
	}
	return nil
}

var sessionEventKinds = map[string]bool{
	"nudge":   true,
	"summary": true,
	"skill":   true,
}

// RecordSessionEvent appends one observability row about learning-loop activity.
func (r *Repository) RecordSessionEvent(ctx context.Context, sessionID, kind, payload string) error {
	if !sessionEventKinds[kind] {
		return fmt.Errorf("invalid session event kind %q", kind)
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO session_events (session_id, kind, payload, created_at)
		 VALUES (?, ?, ?, ?)`,
		sessionID, kind, payload, formatTime(time.Now().UTC())); err != nil {
		return fmt.Errorf("recording session event %q: %w", kind, err)
	}
	return nil
}

func scanConversation(s rowScanner) (*Conversation, error) {
	var (
		conv                 Conversation
		createdAt, updatedAt string
	)
	if err := s.Scan(&conv.ID, &conv.Title, &createdAt, &updatedAt, &conv.Summary, &conv.MessageCount, &conv.Goal, &conv.NextSteps, &conv.RelevantFiles); err != nil {
		return nil, err
	}
	var err error
	if conv.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if conv.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	return &conv, nil
}

func scanMessage(s rowScanner) (Message, error) {
	var (
		m         Message
		role      string
		createdAt string
	)
	if err := s.Scan(&m.ID, &m.ConversationID, &role, &m.Content, &createdAt); err != nil {
		return Message{}, err
	}
	m.Role = Role(role)
	t, err := parseTime(createdAt)
	if err != nil {
		return Message{}, err
	}
	m.CreatedAt = t
	return m, nil
}

func scanObservation(s rowScanner) (Observation, error) {
	var (
		o                    Observation
		createdAt, updatedAt string
	)
	if err := s.Scan(&o.ID, &o.TopicKey, &o.Type, &o.Content, &o.Importance, &o.Project, &o.WorkspaceDir, &o.Status, &createdAt, &updatedAt, &o.SourceRef); err != nil {
		return Observation{}, err
	}
	var err error
	if o.CreatedAt, err = parseTime(createdAt); err != nil {
		return Observation{}, err
	}
	if o.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return Observation{}, err
	}
	return o, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing timestamp %q: %w", s, err)
	}
	return t, nil
}

var skillSources = map[string]bool{
	SourceImported: true,
	SourceAgent:    true,
}

// SaveSkill persists one skill, upserting on the unique name.
func (r *Repository) SaveSkill(ctx context.Context, skill Skill) error {
	if strings.TrimSpace(skill.Name) == "" {
		return fmt.Errorf("skill has empty name")
	}
	if strings.TrimSpace(skill.Content) == "" {
		return fmt.Errorf("skill %q has empty content", skill.Name)
	}
	if !skillSources[skill.Source] {
		return fmt.Errorf("invalid skill source %q", skill.Source)
	}

	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning skill transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM skills WHERE name = ?`, skill.Name).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO skills (id, name, description, "trigger", content, source, usage_count, last_used, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, 0, '', ?)`,
			id, skill.Name, skill.Description, skill.Trigger, skill.Content,
			skill.Source, formatTime(now)); err != nil {
			return fmt.Errorf("inserting skill %q: %w", skill.Name, err)
		}
	case err != nil:
		return fmt.Errorf("looking up skill %q: %w", skill.Name, err)
	default:
		if _, err := tx.ExecContext(ctx,
			`UPDATE skills SET description = ?, "trigger" = ?, content = ?, source = ? WHERE id = ?`,
			skill.Description, skill.Trigger, skill.Content, skill.Source, id); err != nil {
			return fmt.Errorf("updating skill %q: %w", skill.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing skill %q: %w", skill.Name, err)
	}
	return nil
}

// ListSkills returns every known skill ordered by last_used descending, then by name ascending.
func (r *Repository) ListSkills(ctx context.Context) ([]Skill, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, "trigger", content, source, usage_count, last_used, created_at
		 FROM skills
		 ORDER BY last_used DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing skills: %w", err)
	}
	defer rows.Close()

	var out []Skill
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating skills: %w", err)
	}
	return out, nil
}

// RecordSkillUsage increments the usage counter and stamps last_used for the named skill.
func (r *Repository) RecordSkillUsage(ctx context.Context, name string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE skills SET usage_count = usage_count + 1, last_used = ?
		 WHERE name = ?`, formatTime(time.Now().UTC()), name)
	if err != nil {
		return fmt.Errorf("recording skill usage %q: %w", name, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("recording skill usage %q: %w", name, ErrNotFound)
	}
	return nil
}

// DeleteSkill permanently removes the named skill from the repository.
func (r *Repository) DeleteSkill(ctx context.Context, name string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM skills WHERE name = ?`, name); err != nil {
		return fmt.Errorf("deleting skill %q: %w", name, err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSkill(sc scanner) (Skill, error) {
	var (
		s            Skill
		lastUsedStr  string
		createdAtStr string
	)
	if err := sc.Scan(&s.ID, &s.Name, &s.Description, &s.Trigger, &s.Content,
		&s.Source, &s.UsageCount, &lastUsedStr, &createdAtStr); err != nil {
		return Skill{}, fmt.Errorf("scanning skill: %w", err)
	}
	if lastUsedStr != "" {
		t, err := parseTime(lastUsedStr)
		if err != nil {
			return Skill{}, fmt.Errorf("parsing skill last_used: %w", err)
		}
		s.LastUsed = t
	}
	created, err := parseTime(createdAtStr)
	if err != nil {
		return Skill{}, fmt.Errorf("parsing skill created_at: %w", err)
	}
	s.CreatedAt = created
	return s, nil
}

// AppendAudit records one security-relevant event.
func (r *Repository) AppendAudit(ctx context.Context, e AuditEntry) error {
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log (ts, backend, category, subject, decision, scope)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		formatTime(time.Now().UTC()), e.Backend, e.Category, e.Subject, e.Decision, e.Scope); err != nil {
		return fmt.Errorf("appending audit entry: %w", err)
	}
	return nil
}

// AuditTail returns up to n audit entries, newest first.
func (r *Repository) AuditTail(ctx context.Context, n int) ([]AuditEntry, error) {
	q := `SELECT id, ts, backend, category, subject, decision, scope FROM audit_log ORDER BY id DESC`
	args := []any{}
	if n > 0 {
		q += ` LIMIT ?`
		args = append(args, n)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("reading audit tail: %w", err)
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var (
			e     AuditEntry
			tsStr string
		)
		if err := rows.Scan(&e.ID, &tsStr, &e.Backend, &e.Category, &e.Subject, &e.Decision, &e.Scope); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		ts, err := parseTime(tsStr)
		if err != nil {
			return nil, fmt.Errorf("parsing audit ts: %w", err)
		}
		e.Time = ts
		out = append(out, e)
	}
	return out, rows.Err()
}

const conversationOrder = `ORDER BY updated_at DESC, id DESC`

// ListConversations returns conversations ordered updated_at DESC, id DESC.
func (r *Repository) ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error) {
	q := `SELECT ` + conversationCols + ` FROM conversations ` + conversationOrder
	args := []any{}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
		if offset > 0 {
			q += ` OFFSET ?`
			args = append(args, offset)
		}
	} else if offset > 0 {
		q += ` LIMIT -1 OFFSET ?`
		args = append(args, offset)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing conversations: %w", err)
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		conv, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating conversations: %w", err)
	}
	return out, nil
}

// GetConversation returns one conversation by id.
func (r *Repository) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	conv, err := scanConversation(r.db.QueryRowContext(ctx,
		`SELECT `+conversationCols+` FROM conversations WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading conversation %s: %w", id, err)
	}
	return conv, nil
}

// RenameConversation updates a conversation's title and bumps updated_at.
func (r *Repository) RenameConversation(ctx context.Context, id, title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("title must not be empty")
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`,
		title, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("renaming conversation %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("renaming conversation %s: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteConversation permanently removes a conversation and its cascaded data.
func (r *Repository) DeleteConversation(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting conversation %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows for %s: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateSnapshot captures a point-in-time copy of a conversation.
func (r *Repository) CreateSnapshot(ctx context.Context, convID string) (*Snapshot, error) {
	conv, err := r.GetConversation(ctx, convID)
	if err != nil {
		return nil, err
	}
	msgs, err := r.Messages(ctx, convID, 0)
	if err != nil {
		return nil, fmt.Errorf("loading messages for snapshot: %w", err)
	}
	data, err := json.Marshal(msgs)
	if err != nil {
		return nil, fmt.Errorf("marshaling snapshot messages: %w", err)
	}
	now := time.Now().UTC()
	snap := &Snapshot{
		ID:             uuid.NewString(),
		ConversationID: conv.ID,
		Title:          conv.Title,
		Summary:        conv.Summary,
		MessagesJSON:   string(data),
		CreatedAt:      now,
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO snapshots (id, conversation_id, title, summary, messages_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		snap.ID, snap.ConversationID, snap.Title, snap.Summary, snap.MessagesJSON, formatTime(now)); err != nil {
		return nil, fmt.Errorf("inserting snapshot: %w", err)
	}
	return snap, nil
}

// ListSnapshots returns snapshots for a conversation, newest first.
func (r *Repository) ListSnapshots(ctx context.Context, convID string) ([]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, conversation_id, title, summary, messages_json, created_at
		 FROM snapshots WHERE conversation_id = ? ORDER BY created_at DESC, id DESC`, convID)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	defer rows.Close()

	var out []Snapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating snapshots: %w", err)
	}
	return out, nil
}

func scanSnapshot(s rowScanner) (Snapshot, error) {
	var snap Snapshot
	var createdAt string
	if err := s.Scan(&snap.ID, &snap.ConversationID, &snap.Title, &snap.Summary, &snap.MessagesJSON, &createdAt); err != nil {
		return Snapshot{}, err
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return Snapshot{}, err
	}
	snap.CreatedAt = t
	return snap, nil
}

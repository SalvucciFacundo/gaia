package mneme

import (
	"context"
	"errors"
)

// fakeChatProvider is a Provider double whose Chat returns a fixed
// response or error and records every request.
type fakeChatProvider struct {
	chatResp ChatResponse
	chatErr  error
	requests []ChatRequest
}

var _ Provider = (*fakeChatProvider)(nil)

func (f *fakeChatProvider) Chat(_ context.Context, req ChatRequest) (ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.chatErr != nil {
		return ChatResponse{}, f.chatErr
	}
	return f.chatResp, nil
}

func (f *fakeChatProvider) Stream(context.Context, ChatRequest) (<-chan StreamEvent, error) {
	return nil, errors.New("Stream not used")
}

func (f *fakeChatProvider) Models() []ModelInfo { return nil }

// recordingRepo is a Store double that records the sequence of calls.
type recordingRepo struct {
	calls []string

	savedObservations []Observation
	savedUserModel    []UserModel
	summary           string
	summaryConvID     string
	events            []recordedEvent
	savedSkills       []Skill
	skills            []Skill
	usedSkill         string
	userModelRows     []UserModel
	auditEntries      []AuditEntry
}

type recordedEvent struct {
	sessionID string
	kind      string
	payload   string
}

var _ Store = (*recordingRepo)(nil)

func (r *recordingRepo) CreateConversation(context.Context, string) (*Conversation, error) {
	r.calls = append(r.calls, "CreateConversation")
	return &Conversation{ID: "conv-1"}, nil
}

func (r *recordingRepo) LatestConversation(context.Context) (*Conversation, error) {
	r.calls = append(r.calls, "LatestConversation")
	return &Conversation{ID: "conv-1"}, nil
}

func (r *recordingRepo) AppendMessage(context.Context, string, Message) error {
	r.calls = append(r.calls, "AppendMessage")
	return nil
}

func (r *recordingRepo) Messages(context.Context, string, int) ([]Message, error) {
	r.calls = append(r.calls, "Messages")
	return nil, nil
}

func (r *recordingRepo) Search(context.Context, string, int) ([]SearchResult, error) {
	r.calls = append(r.calls, "Search")
	return nil, nil
}

func (r *recordingRepo) SaveObservations(_ context.Context, _ string, obs []Observation) error {
	r.calls = append(r.calls, "SaveObservations")
	r.savedObservations = append(r.savedObservations, obs...)
	return nil
}

func (r *recordingRepo) Observations(context.Context, int) ([]Observation, error) {
	r.calls = append(r.calls, "Observations")
	return nil, nil
}

func (r *recordingRepo) ObservationsByProject(context.Context, string, int) ([]Observation, error) {
	r.calls = append(r.calls, "ObservationsByProject")
	return nil, nil
}

func (r *recordingRepo) UpdateConversationSummary(_ context.Context, convID, summary string) error {
	r.calls = append(r.calls, "UpdateConversationSummary")
	r.summaryConvID = convID
	r.summary = summary
	return nil
}

func (r *recordingRepo) UpsertUserModel(_ context.Context, rows []UserModel) error {
	r.calls = append(r.calls, "UpsertUserModel")
	r.savedUserModel = append(r.savedUserModel, rows...)
	return nil
}

func (r *recordingRepo) RecordSessionEvent(_ context.Context, sessionID, kind, payload string) error {
	r.calls = append(r.calls, "RecordSessionEvent")
	r.events = append(r.events, recordedEvent{sessionID: sessionID, kind: kind, payload: payload})
	return nil
}

func (r *recordingRepo) SaveSkill(_ context.Context, skill Skill) error {
	r.calls = append(r.calls, "SaveSkill")
	r.savedSkills = append(r.savedSkills, skill)
	return nil
}

func (r *recordingRepo) ListSkills(context.Context) ([]Skill, error) {
	r.calls = append(r.calls, "ListSkills")
	return r.skills, nil
}

func (r *recordingRepo) RecordSkillUsage(_ context.Context, name string) error {
	r.calls = append(r.calls, "RecordSkillUsage")
	r.usedSkill = name
	return nil
}

func (r *recordingRepo) DeleteSkill(_ context.Context, name string) error {
	r.calls = append(r.calls, "DeleteSkill")
	return nil
}

func (r *recordingRepo) UserModelRows(context.Context, int) ([]UserModel, error) {
	r.calls = append(r.calls, "UserModelRows")
	return r.userModelRows, nil
}

func (r *recordingRepo) ClearUserModel(context.Context) error {
	r.calls = append(r.calls, "ClearUserModel")
	return nil
}

func (r *recordingRepo) AppendAudit(_ context.Context, e AuditEntry) error {
	r.calls = append(r.calls, "AppendAudit")
	r.auditEntries = append(r.auditEntries, e)
	return nil
}

func (r *recordingRepo) AuditTail(context.Context, int) ([]AuditEntry, error) {
	r.calls = append(r.calls, "AuditTail")
	return r.auditEntries, nil
}

func (r *recordingRepo) ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error) {
	return nil, nil
}
func (r *recordingRepo) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	return nil, ErrNotFound
}
func (r *recordingRepo) RenameConversation(ctx context.Context, id, title string) error { return nil }
func (r *recordingRepo) DeleteConversation(ctx context.Context, id string) error        { return nil }
func (r *recordingRepo) CreateSnapshot(ctx context.Context, convID string) (*Snapshot, error) {
	return &Snapshot{ID: "snap-1"}, nil
}
func (r *recordingRepo) ListSnapshots(ctx context.Context, convID string) ([]Snapshot, error) {
	return nil, nil
}

func (r *recordingRepo) Close() error { return nil }

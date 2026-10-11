package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"gaia/internal/mneme"
)

type fakeStore struct {
	mneme.Store
	convs    map[string]*mneme.Conversation
	messages map[string][]mneme.Message
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		convs:    make(map[string]*mneme.Conversation),
		messages: make(map[string][]mneme.Message),
	}
}

func (f *fakeStore) CreateConversation(ctx context.Context, title string) (*mneme.Conversation, error) {
	if title == "" {
		title = "New conversation"
	}
	id := "conv-" + time.Now().Format("150405.000000")
	conv := &mneme.Conversation{
		ID:        id,
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.convs[id] = conv
	return conv, nil
}

func (f *fakeStore) GetConversation(ctx context.Context, id string) (*mneme.Conversation, error) {
	conv, ok := f.convs[id]
	if !ok {
		return nil, mneme.ErrNotFound
	}
	return conv, nil
}

func (f *fakeStore) RenameConversation(ctx context.Context, id, title string) error {
	conv, ok := f.convs[id]
	if !ok {
		return mneme.ErrNotFound
	}
	conv.Title = title
	return nil
}

func (f *fakeStore) DeleteConversation(ctx context.Context, id string) error {
	delete(f.convs, id)
	delete(f.messages, id)
	return nil
}

func (f *fakeStore) ListConversations(ctx context.Context, limit, offset int) ([]mneme.Conversation, error) {
	var list []mneme.Conversation
	for _, c := range f.convs {
		list = append(list, *c)
	}
	return list, nil
}

func (f *fakeStore) Messages(ctx context.Context, convID string, limit int) ([]mneme.Message, error) {
	return f.messages[convID], nil
}

func (f *fakeStore) CreateSnapshot(ctx context.Context, convID string) (*mneme.Snapshot, error) {
	return &mneme.Snapshot{ID: "snap-" + convID, ConversationID: convID}, nil
}

func TestManager_Lifecycle(t *testing.T) {
	store := newFakeStore()
	mgr := New(store, nil, nil)

	ctx := context.Background()

	// NewSession
	conv, err := mgr.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if mgr.ActiveID() != conv.ID {
		t.Errorf("activeID = %q, want %q", mgr.ActiveID(), conv.ID)
	}

	// Rename
	if err := mgr.Rename(ctx, conv.ID, "Custom Work Session"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	updated, _ := store.GetConversation(ctx, conv.ID)
	if updated.Title != "Custom Work Session" {
		t.Errorf("title = %q, want 'Custom Work Session'", updated.Title)
	}

	// Export Markdown
	store.messages[conv.ID] = []mneme.Message{
		{Role: mneme.RoleUser, Content: "Hello assistant"},
		{Role: mneme.RoleAssistant, Content: "Hello user"},
	}
	md, err := mgr.Export(ctx, conv.ID, ExportFormatMarkdown)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if !strings.Contains(md, "# Custom Work Session") || !strings.Contains(md, "Hello assistant") {
		t.Errorf("export output mismatch:\n%s", md)
	}

	// Delete
	if err := mgr.Delete(ctx, conv.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mgr.ActiveID() != "" {
		t.Errorf("activeID after delete = %q, want empty", mgr.ActiveID())
	}
}

func TestManager_ChannelRoute(t *testing.T) {
	store := newFakeStore()
	mgr := New(store, nil, nil)

	ctx := context.Background()

	// Route WhatsApp message
	conv1, err := mgr.ChannelRoute(ctx, "whatsapp", "+5491112345678")
	if err != nil {
		t.Fatalf("ChannelRoute failed: %v", err)
	}

	// Route same channel again -> should return same convID
	conv2, err := mgr.ChannelRoute(ctx, "whatsapp", "+5491112345678")
	if err != nil {
		t.Fatalf("ChannelRoute 2 failed: %v", err)
	}
	if conv1 != conv2 {
		t.Errorf("expected same convID %q, got %q", conv1, conv2)
	}

	// Different channel (Telegram) -> should return different convID
	convTelegram, err := mgr.ChannelRoute(ctx, "telegram", "chat-999")
	if err != nil {
		t.Fatalf("ChannelRoute telegram failed: %v", err)
	}
	if convTelegram == conv1 {
		t.Errorf("expected distinct convID for telegram, got same: %q", convTelegram)
	}
}

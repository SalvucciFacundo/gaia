package commands

import (
	"context"
	"testing"
)

func TestDispatcher_NonCommand(t *testing.T) {
	d := New(nil)
	ctx := context.Background()

	handled, err := d.Dispatch(ctx, "hello world")
	if handled {
		t.Errorf("expected handled=false for regular text, got true")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDispatcher_NilBrain(t *testing.T) {
	d := New(nil)
	ctx := context.Background()

	handled, err := d.Dispatch(ctx, "/help")
	if handled {
		t.Errorf("expected handled=false for nil brain, got true")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDispatcher_ContextCancelled(t *testing.T) {
	d := New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handled, err := d.Dispatch(ctx, "/help")
	if handled {
		t.Errorf("expected handled=false for cancelled context, got true")
	}
	if err == nil {
		t.Errorf("expected error for cancelled context, got nil")
	}
}

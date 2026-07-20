package core

import (
	"testing"

	"gaia/internal/core/domain"
)

func TestConfirmGuard_AlwaysMode(t *testing.T) {
	g := NewConfirmGuard(domain.TrustAlways, false)
	if !g.ShouldConfirm("shell_exec") {
		t.Error("always mode should require confirmation")
	}
	if !g.ShouldConfirm("file_read") {
		t.Error("always mode should require confirmation for all tools")
	}
}

func TestConfirmGuard_NeverMode(t *testing.T) {
	g := NewConfirmGuard(domain.TrustNever, false)
	if g.ShouldConfirm("shell_exec") {
		t.Error("never mode should not require confirmation")
	}
}

func TestConfirmGuard_PerSessionMode(t *testing.T) {
	g := NewConfirmGuard(domain.TrustPerSession, false)

	// First call: should confirm
	if !g.ShouldConfirm("shell_exec") {
		t.Error("per-session: first call should confirm")
	}

	// Approve the tool
	g.Approve("shell_exec")

	// Second call: should NOT confirm
	if g.ShouldConfirm("shell_exec") {
		t.Error("per-session: approved tool should not re-confirm")
	}

	// Different tool: should still confirm
	if !g.ShouldConfirm("file_read") {
		t.Error("per-session: unapproved tool should confirm")
	}
}

func TestConfirmGuard_PerActionMode(t *testing.T) {
	g := NewConfirmGuard(domain.TrustPerAction, false)

	if !g.ShouldConfirm("shell_exec") {
		t.Error("per-action should always confirm")
	}

	g.Approve("shell_exec")

	// Per-action ignores approvals
	if !g.ShouldConfirm("shell_exec") {
		t.Error("per-action should confirm even after approve")
	}
}

func TestConfirmGuard_HeadlessMode(t *testing.T) {
	g := NewConfirmGuard(domain.TrustAlways, true)
	if g.ShouldConfirm("shell_exec") {
		t.Error("headless mode should default to never")
	}
	if g.Mode() != domain.TrustNever {
		t.Errorf("headless mode should be never, got %s", g.Mode())
	}
}

func TestConfirmGuard_SetMode(t *testing.T) {
	g := NewConfirmGuard(domain.TrustAlways, false)
	g.SetMode(domain.TrustNever)
	if g.ShouldConfirm("shell_exec") {
		t.Error("after setMode to never, should not confirm")
	}
}

func TestConfirmGuard_Reset(t *testing.T) {
	g := NewConfirmGuard(domain.TrustPerSession, false)
	g.Approve("tool_a")
	if g.ShouldConfirm("tool_a") {
		t.Error("approved tool should not prompt")
	}
	g.Reset()
	if !g.ShouldConfirm("tool_a") {
		t.Error("after reset, approved tool should prompt again")
	}
}

func TestConfirmGuard_DefaultMode(t *testing.T) {
	g := NewConfirmGuard("", false)
	if g.Mode() != domain.TrustAlways {
		t.Errorf("default mode should be always, got %s", g.Mode())
	}
}

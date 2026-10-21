package agent

import (
	"testing"
)

func TestDetectODDRoute_InlineByDefault(t *testing.T) {
	msgs := []string{
		"fix typo on line 42",
		"explain what this function does",
		"update variable name in auth.go",
	}

	for _, msg := range msgs {
		res := DetectODDRoute(msg)
		if res.Route != RouteInline {
			t.Errorf("expected RouteInline for %q, got %v", msg, res.Route)
		}
	}
}

func TestDetectODDRoute_ExplicitCommands(t *testing.T) {
	// /inline
	resInline := DetectODDRoute("/inline fix the bug")
	if resInline.Route != RouteInline {
		t.Errorf("expected RouteInline for /inline, got %v", resInline.Route)
	}

	// Legacy /direct
	resDirect := DetectODDRoute("/direct fix the bug")
	if resDirect.Route != RouteInline || !resDirect.IsDeprecatedAlias {
		t.Errorf("expected RouteInline with IsDeprecatedAlias for /direct, got %+v", resDirect)
	}

	// /odd
	resODD := DetectODDRoute("/odd overhaul storage layer")
	if resODD.Route != RouteFeatureTracking {
		t.Errorf("expected RouteFeatureTracking for /odd, got %v", resODD.Route)
	}

	// Legacy /sdd
	resSDD := DetectODDRoute("/sdd overhaul storage layer")
	if resSDD.Route != RouteFeatureTracking || !resSDD.IsDeprecatedAlias {
		t.Errorf("expected RouteFeatureTracking with IsDeprecatedAlias for /sdd, got %+v", resSDD)
	}
}

func TestDetectODDRoute_DelegatedWorkerTriggers(t *testing.T) {
	// Explorer trigger
	resExp := DetectODDRoute("please map codebase and trace all references")
	if resExp.Route != RouteDelegatedWorker || resExp.WorkerRole != "explorer" {
		t.Errorf("expected RouteDelegatedWorker (explorer), got %+v", resExp)
	}

	// Writer trigger
	resWrite := DetectODDRoute("refactor multiple files in the package")
	if resWrite.Route != RouteDelegatedWorker || resWrite.WorkerRole != "writer" {
		t.Errorf("expected RouteDelegatedWorker (writer), got %+v", resWrite)
	}

	// Verifier trigger
	resVer := DetectODDRoute("run full test suite and verify build")
	if resVer.Route != RouteDelegatedWorker || resVer.WorkerRole != "verifier" {
		t.Errorf("expected RouteDelegatedWorker (verifier), got %+v", resVer)
	}
}

func TestDetectODDRoute_ArchitecturalFeature(t *testing.T) {
	res := DetectODDRoute("we have a breaking change in auth")
	if res.Route != RouteFeatureTracking {
		t.Errorf("expected RouteFeatureTracking for breaking change, got %+v", res)
	}
}

func TestMidTurnBudgetTracker_ConcurrencyAndLimits(t *testing.T) {
	tracker := NewMidTurnBudgetTracker()

	done := make(chan bool)

	// Goroutine 1: Record calls
	go func() {
		for i := 0; i < 20; i++ {
			tracker.RecordToolCall("file_read", map[string]interface{}{"path": "main.go"}, "sample output")
			tracker.RecordToolCall("file_write", map[string]interface{}{"path": "file1.go"}, "written")
		}
		done <- true
	}()

	// Goroutine 2: Check triggers and reset
	go func() {
		for i := 0; i < 20; i++ {
			_, _, _ = tracker.CheckDelegationTrigger()
		}
		done <- true
	}()

	<-done
	<-done

	// Verify delegation triggers
	tracker.Reset()
	for i := 0; i < MaxInlineSequentialRead+1; i++ {
		tracker.RecordToolCall("file_read", map[string]interface{}{"path": "test.go"}, "data")
	}
	role, _, triggered := tracker.CheckDelegationTrigger()
	if !triggered || role != "explorer" {
		t.Fatalf("expected explorer trigger on sequential reads, got role=%s triggered=%v", role, triggered)
	}

	tracker.Reset()
	tracker.RecordToolCall("file_write", map[string]interface{}{"path": "a.go"}, "data")
	tracker.RecordToolCall("file_write", map[string]interface{}{"path": "b.go"}, "data")
	role, _, triggered = tracker.CheckDelegationTrigger()
	if !triggered || role != "writer" {
		t.Fatalf("expected writer trigger on multi-file writes, got role=%s triggered=%v", role, triggered)
	}
}

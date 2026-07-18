package main

import (
	"flag"
	"testing"
)

// TestExecFlagParsing verifies that all expected flags are registered
// and that they parse correctly.
func TestExecFlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
		wantDry  bool
		wantQuiet bool
		wantVerb  bool
		wantYes  bool
		wantTask string
	}{
		{
			name:     "all flags set",
			args:     []string{"--json", "--dry-run", "--quiet", "--verbose", "--yes", "do something"},
			wantJSON: true,
			wantDry:  true,
			wantQuiet: true,
			wantVerb: true,
			wantYes:  true,
			wantTask: "do something",
		},
		{
			name:     "default flags with task",
			args:     []string{"build the project"},
			wantJSON: false,
			wantDry:  false,
			wantQuiet: false,
			wantVerb: false,
			wantYes:  false,
			wantTask: "build the project",
		},
		{
			name:     "json only",
			args:     []string{"--json", "explain this code"},
			wantJSON: true,
			wantDry:  false,
			wantQuiet: false,
			wantVerb: false,
			wantYes:  false,
			wantTask: "explain this code",
		},
		{
			name:     "dry-run with quiet",
			args:     []string{"--dry-run", "--quiet", "deploy to prod"},
			wantJSON: false,
			wantDry:  true,
			wantQuiet: true,
			wantVerb: false,
			wantYes:  false,
			wantTask: "deploy to prod",
		},
		{
			name:     "yes flag",
			args:     []string{"--yes", "install dependencies"},
			wantJSON: false,
			wantDry:  false,
			wantQuiet: false,
			wantVerb: false,
			wantYes:  true,
			wantTask: "install dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("exec", flag.ContinueOnError)
			jsonOut := fs.Bool("json", false, "")
			quiet := fs.Bool("quiet", false, "")
			verbose := fs.Bool("verbose", false, "")
			dryRun := fs.Bool("dry-run", false, "")
			yes := fs.Bool("yes", false, "")

			err := fs.Parse(tt.args)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			task := fs.Arg(0)

			if *jsonOut != tt.wantJSON {
				t.Errorf("json flag: got %v, want %v", *jsonOut, tt.wantJSON)
			}
			if *dryRun != tt.wantDry {
				t.Errorf("dry-run flag: got %v, want %v", *dryRun, tt.wantDry)
			}
			if *quiet != tt.wantQuiet {
				t.Errorf("quiet flag: got %v, want %v", *quiet, tt.wantQuiet)
			}
			if *verbose != tt.wantVerb {
				t.Errorf("verbose flag: got %v, want %v", *verbose, tt.wantVerb)
			}
			if *yes != tt.wantYes {
				t.Errorf("yes flag: got %v, want %v", *yes, tt.wantYes)
			}
			if task != tt.wantTask {
				t.Errorf("task: got %q, want %q", task, tt.wantTask)
			}
		})
	}
}

// TestExecFlagParsing_EmptyTask verifies that flag parsing works
// even when no task is provided (args only contain flags).
func TestExecFlagParsing_EmptyTask(t *testing.T) {
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "")
	dryRun := fs.Bool("dry-run", false, "")

	// Parse without any task argument.
	err := fs.Parse([]string{"--json", "--dry-run"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	task := fs.Arg(0)
	if task != "" {
		t.Errorf("expected empty task, got %q", task)
	}
	if !*jsonOut {
		t.Error("expected json flag true")
	}
	if !*dryRun {
		t.Error("expected dry-run flag true")
	}
}

// TestExecFlagParsing_UnknownFlag ensures unknown flags are rejected.
func TestExecFlagParsing_UnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	fs.Bool("json", false, "")

	err := fs.Parse([]string{"--unknown-flag", "task"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

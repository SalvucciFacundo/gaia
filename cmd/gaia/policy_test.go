package main

import (
	"flag"
	"testing"
)

func TestPolicyCLIFlagParsing(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantTier   string
		wantGlobal bool
	}{
		{
			name:       "default flags",
			args:       []string{"init"},
			wantTier:   "sandbox",
			wantGlobal: false,
		},
		{
			name:       "flags after init",
			args:       []string{"init", "--tier=full", "--global"},
			wantTier:   "full",
			wantGlobal: true,
		},
		{
			name:       "flags before init",
			args:       []string{"--tier=read", "init"},
			wantTier:   "read",
			wantGlobal: false,
		},
		{
			name:       "space separated tier",
			args:       []string{"init", "--tier", "full", "--global"},
			wantTier:   "full",
			wantGlobal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("policy", flag.ContinueOnError)
			tier := fs.String("tier", "sandbox", "Default tier: read, sandbox, or full")
			isGlobal := fs.Bool("global", false, "Create global policy instead of project-level")

			var flagArgs []string
			hasInit := false
			for _, arg := range tt.args {
				if arg == "init" && !hasInit {
					hasInit = true
					continue
				}
				flagArgs = append(flagArgs, arg)
			}

			if !hasInit {
				t.Fatal("expected hasInit to be true")
			}

			if err := fs.Parse(flagArgs); err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			if *tier != tt.wantTier {
				t.Errorf("tier = %q, want %q", *tier, tt.wantTier)
			}
			if *isGlobal != tt.wantGlobal {
				t.Errorf("isGlobal = %v, want %v", *isGlobal, tt.wantGlobal)
			}
		})
	}
}

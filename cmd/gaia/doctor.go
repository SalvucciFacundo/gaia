package main

import (
	"context"
	"fmt"
	"os"

	"gaia/internal/doctor"
)

// handleDoctor runs the system diagnostics ("gaia doctor").
// Usage: gaia doctor [--json]
func handleDoctor(args []string) {
	jsonOut := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
		}
	}

	ctx := context.Background()
	results := doctor.RunAll(ctx)

	if jsonOut {
		fmt.Println("[")
		for i, r := range results {
			comma := ","
			if i == len(results)-1 {
				comma = ""
			}
			fmt.Printf(`  {"name":%q, "status":%q, "message":%q, "duration":%q}%s`,
				r.Name, r.Status, r.Message, r.Duration.String(), comma)
			fmt.Println()
		}
		fmt.Println("]")
		return
	}

	// Print formatted table
	fmt.Print(doctor.FormatTable(results))

	// Check for failures
	hasFailures := false
	for _, r := range results {
		if r.Status == doctor.StatusFail {
			hasFailures = true
			break
		}
	}

	if hasFailures {
		os.Exit(1)
	}
}

package main

import (
	"context"
	"fmt"
	"os"

	"gaia/internal/adapters/db"
)

func handleSessionCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia session <command> [args]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  list               List recent sessions")
		fmt.Println("  restore <id>       Restore a session by ID")
		return
	}

	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()

	switch args[0] {
	case "list":
		sessions, err := repo.ListSessions(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return
		}
		fmt.Printf("Recent sessions (%d):\n", len(sessions))
		for _, s := range sessions {
			fmt.Printf("  %s  %-30s %s\n", s.ID[:12], s.Name, s.CreatedAt.Format("2006-01-02 15:04"))
		}

	case "restore":
		if len(args) < 2 {
			fmt.Println("Usage: gaia session restore <id>")
			return
		}
		sessionID := args[1]
		msgs, err := repo.GetMessages(ctx, sessionID, 100)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		fmt.Printf("Session %s restored (%d messages):\n", sessionID, len(msgs))
		for _, m := range msgs {
			preview := m.Content
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			fmt.Printf("  [%s] %s\n", m.Role, preview)
		}

	default:
		fmt.Printf("Unknown session command: %s\n", args[0])
	}
}

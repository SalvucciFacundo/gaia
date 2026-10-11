package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gaia/internal/adapters/db"
	"gaia/internal/config"
	"gaia/internal/core/domain"
	"gaia/internal/cron"
)

// handleCronCLI implements the "gaia cron" subcommand family.
// Usage: gaia cron [create|list|pause|resume|remove|start]
func handleCronCLI(args []string) {
	if len(args) == 0 {
		printCronUsage()
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]

	// Initialize DB for cron operations.
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}

	// Ensure cron table exists.
	if err := repo.MigrateCron(); err != nil {
		fmt.Fprintf(os.Stderr, "Error migrating cron table: %v\n", err)
		os.Exit(1)
	}

	scheduler := cron.NewScheduler(repo)
	ctx := context.Background()

	switch cmd {
	case "create":
		handleCronCreate(ctx, scheduler, cmdArgs)
	case "list":
		handleCronList(ctx, scheduler)
	case "pause":
		handleCronPause(ctx, scheduler, cmdArgs)
	case "resume":
		handleCronResume(ctx, scheduler, cmdArgs)
	case "remove":
		handleCronRemove(ctx, scheduler, cmdArgs)
	case "start":
		handleCronStart(ctx, scheduler, repo)
	default:
		fmt.Printf("Unknown cron command: %s\n", cmd)
		printCronUsage()
	}
}

func printCronUsage() {
	fmt.Println("Usage: gaia cron <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  create <schedule> <task> [--name <name>] [--deliver terminal|telegram]")
	fmt.Println("  list                              List all cron jobs")
	fmt.Println("  pause <id>                        Pause a cron job")
	fmt.Println("  resume <id>                       Resume a cron job")
	fmt.Println("  remove <id>                       Remove a cron job")
	fmt.Println("  start                             Start the cron scheduler (blocking)")
	fmt.Println()
	fmt.Println("Schedule format: 5-field cron expression (minute hour day month weekday)")
	fmt.Println("Example: gaia cron create \"0 2 * * *\" \"run backup\" --name \"nightly-backup\"")
}

func handleCronCreate(ctx context.Context, s *cron.Scheduler, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: gaia cron create <schedule> <task> [--name <name>] [--deliver terminal|telegram]")
		return
	}

	schedule := args[0]
	task := args[1]

	// Parse optional flags
	name := ""
	deliverTo := "terminal"
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--deliver":
			if i+1 < len(args) {
				deliverTo = args[i+1]
				i++
			}
		}
	}

	if name == "" {
		name = task
		if len(name) > 50 {
			name = name[:50]
		}
	}

	job := domain.CronJob{
		Name:      name,
		Schedule:  schedule,
		Task:      task,
		DeliverTo: deliverTo,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	id, err := s.CreateJob(ctx, job)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cron job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Cron job created: %s\n", id)
	fmt.Printf("  Name:     %s\n", name)
	fmt.Printf("  Schedule: %s\n", schedule)
	fmt.Printf("  Task:     %s\n", task)
	fmt.Printf("  Deliver:  %s\n", deliverTo)
}

func handleCronList(ctx context.Context, s *cron.Scheduler) {
	jobs, err := s.ListJobs(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing cron jobs: %v\n", err)
		os.Exit(1)
	}

	if len(jobs) == 0 {
		fmt.Println("No cron jobs found.")
		return
	}

	fmt.Printf("Cron Jobs (%d):\n", len(jobs))
	fmt.Println(strings.Repeat("-", 80))
	for _, j := range jobs {
		status := "enabled"
		if !j.Enabled {
			status = "paused"
		}
		fmt.Printf("ID:       %s\n", j.ID)
		fmt.Printf("Name:     %s\n", j.Name)
		fmt.Printf("Schedule: %s\n", j.Schedule)
		fmt.Printf("Task:     %s\n", j.Task)
		fmt.Printf("Status:   %s\n", status)
		if !j.LastRun.IsZero() {
			fmt.Printf("Last Run: %s\n", j.LastRun.Format(time.RFC3339))
		}
		if !j.NextRun.IsZero() {
			fmt.Printf("Next Run: %s\n", j.NextRun.Format(time.RFC3339))
		}
		fmt.Println(strings.Repeat("-", 80))
	}
}

func handleCronPause(ctx context.Context, s *cron.Scheduler, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia cron pause <id>")
		return
	}

	id := args[0]
	if err := s.PauseJob(ctx, id); err != nil {
		fmt.Fprintf(os.Stderr, "Error pausing job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Job %s paused.\n", id)
}

func handleCronResume(ctx context.Context, s *cron.Scheduler, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia cron resume <id>")
		return
	}

	id := args[0]
	if err := s.ResumeJob(ctx, id); err != nil {
		fmt.Fprintf(os.Stderr, "Error resuming job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Job %s resumed.\n", id)
}

func handleCronRemove(ctx context.Context, s *cron.Scheduler, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia cron remove <id>")
		return
	}

	id := args[0]
	if err := s.RemoveJob(ctx, id); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Job %s removed.\n", id)
}

func handleCronStart(ctx context.Context, s *cron.Scheduler, repo *db.SQLiteRepo) {
	fmt.Println("Starting cron scheduler...")
	fmt.Println("Press Ctrl+C to stop.")

	// Load config for terminal backend
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
	}

	fmt.Printf("Using %d registered modules\n", 3)
	_ = cfg

	if err := s.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Scheduler error: %v\n", err)
		os.Exit(1)
	}
}

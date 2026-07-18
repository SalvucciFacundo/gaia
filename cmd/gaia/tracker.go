package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gaia/internal/adapters/db"
	"gaia/internal/tracker"
)

// handleTrackerCLI implements the "gaia tracker" subcommand family.
// Usage: gaia tracker <check|report|port> [args]
func handleTrackerCLI(args []string) {
	if len(args) == 0 {
		printTrackerUsage()
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "check":
		handleTrackerCheck()
	case "report":
		handleTrackerReport()
	case "port":
		handleTrackerPort(cmdArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown tracker command: %s\n", cmd)
		printTrackerUsage()
		os.Exit(1)
	}
}

func printTrackerUsage() {
	fmt.Println("Usage: gaia tracker <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  check           Fetch latest upstream release and show delta")
	fmt.Println("  report          Print full port status as markdown table")
	fmt.Println("  port <feature>  Mark a feature as ported in the manifest")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gaia tracker check")
	fmt.Println("  gaia tracker report")
	fmt.Println("  gaia tracker port streaming-tools")
}

// handleTrackerCheck fetches the latest upstream release and shows new changes.
func handleTrackerCheck() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	manifestPath := filepath.Join(projectRoot, "tracker", "manifest.yaml")

	// Initialize dependencies.
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	trackerRepo := db.NewTrackerRepo(repo.DB())
	monitor := tracker.NewGitHubReleaseMonitor("Gentleman-Programming", "gentle-ai", trackerRepo)
	analyzer := &tracker.ChangelogAnalyzer{}

	manifest, err := tracker.LoadManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading manifest: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	release, hasNew, err := monitor.CheckLatest(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking releases: %v\n", err)
		os.Exit(1)
	}

	if !hasNew {
		fmt.Println("No new upstream release detected (ETag cache hit).")
		fmt.Printf("Manifest has %d tracked features.\n", len(manifest.Features))
		return
	}

	fmt.Printf("New release detected: %s (published %s)\n", release.Tag, release.PublishedAt.Format("2006-01-02"))

	// Cache the release.
	if err := trackerRepo.SaveRelease(ctx, *release); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not cache release: %v\n", err)
	}

	// Analyze the release and show delta.
	changes := analyzer.AnalyzeRelease(*release)
	if len(changes) == 0 {
		fmt.Println("No classified changes found in release body.")
		return
	}

	fmt.Printf("Found %d changes:\n\n", len(changes))
	for _, c := range changes {
		fmt.Printf("  [%s] %s\n", c.Type, c.Description)
	}

	// Show delta report.
	deltaReport := tracker.GenerateDelta(nil, changes, manifest)
	fmt.Println()
	fmt.Println(deltaReport)
}

// handleTrackerReport prints a full markdown port status report.
func handleTrackerReport() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	manifestPath := filepath.Join(projectRoot, "tracker", "manifest.yaml")

	manifest, err := tracker.LoadManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(tracker.GenerateReport(manifest))
}

// handleTrackerPort marks a feature as ported in the manifest.
// Usage: gaia tracker port <feature-name>
// Optionally creates a GitHub issue via "gh issue create".
func handleTrackerPort(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gaia tracker port <feature-name>")
		os.Exit(1)
	}

	featureName := args[0]

	// Sanitize feature name: reject shell metacharacters.
	if !isSafeFeatureName(featureName) {
		fmt.Fprintf(os.Stderr, "Error: feature name %q contains unsafe characters\n", featureName)
		os.Exit(1)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	manifestPath := filepath.Join(projectRoot, "tracker", "manifest.yaml")

	manifest, err := tracker.LoadManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading manifest: %v\n", err)
		os.Exit(1)
	}

	// Update the entry.
	entry := manifest.FindEntry(featureName)
	if entry == nil {
		fmt.Printf("Feature %q not found in manifest. Adding as new ported entry.\n", featureName)
	}
	manifest.UpdateEntry(featureName, tracker.StatusPorted, "marked via gaia tracker port")

	if err := manifest.Save(manifestPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Feature %q marked as ported.\n", featureName)

	// Auto-create a GitHub issue for tracking.
	createTrackingIssue(featureName)
}

// createTrackingIssue attempts to create a GitHub issue via `gh issue create`.
// Falls back to printing a tip if gh is not installed.
func createTrackingIssue(featureName string) {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		fmt.Println("Tip: to create a tracking issue, install gh CLI or run:")
		fmt.Printf("  gh issue create --title \"Port upstream feature: %s\" --body \"Port %s from upstream Gentle AI.\"\n",
			featureName, featureName)
		return
	}

	title := fmt.Sprintf("Port upstream feature: %s", featureName)
	body := fmt.Sprintf("Port %s from upstream Gentle AI.\n\nSee tracker/manifest.yaml for details.", featureName)
	cmd := exec.Command(ghPath, "issue", "create",
		"--title", title,
		"--body", body,
	)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Warning: could not create GitHub issue: %v\nTip: run manually:\n", err)
		fmt.Printf("  gh issue create --title %q --body %q\n", title, body)
		return
	}

	url := strings.TrimSpace(string(output))
	fmt.Printf("✅ Tracking issue created: %s\n", url)
}

// safeNameRe matches feature names composed of letters, digits, hyphens, and underscores.
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)

func isSafeFeatureName(name string) bool {
	return safeNameRe.MatchString(name)
}


package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	"gaia/internal/delivery"
	"gaia/internal/review/gates"
)

// handleDeliveryCLI implements the "gaia delivery" subcommand family.
// Usage: gaia delivery <command> [flags] [args]
func handleDeliveryCLI(args []string) {
	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	queuePath := filepath.Join(repoRoot, ".gaia", "delivery", "queue.json")
	q, err := delivery.NewFileQueue(queuePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing delivery queue: %v\n", err)
		os.Exit(1)
	}

	casStore := gates.NewCASReceiptStore(repoRoot)
	transport := delivery.NewExecTransport(repoRoot)
	eng := delivery.NewEngine(q, transport, casStore, repoRoot)

	if err := runDeliveryCLI(args, os.Stdout, os.Stderr, q, eng, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDeliveryCLI(args []string, out, errOut io.Writer, q delivery.Queue, eng *delivery.Engine, repoRoot string) error {
	if len(args) == 0 {
		printDeliveryUsage(out)
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "list":
		return handleDeliveryList(cmdArgs, out, q)
	case "status":
		return handleDeliveryStatus(out, q)
	case "release":
		return handleDeliveryRelease(cmdArgs, out, errOut, q, eng, repoRoot)
	case "discard":
		return handleDeliveryDiscard(cmdArgs, out, errOut, q)
	case "diff":
		return handleDeliveryDiff(cmdArgs, out, errOut, q, repoRoot)
	default:
		fmt.Fprintf(errOut, "Unknown delivery command: %s\n", cmd)
		printDeliveryUsage(out)
		return fmt.Errorf("unknown delivery command: %s", cmd)
	}
}

func printDeliveryUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: gaia delivery <command> [flags] [args]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  list       List queued delivery items (default: all or filtered by status)")
	fmt.Fprintln(out, "  status     Show queue summary metrics")
	fmt.Fprintln(out, "  release    Release one item by ID or all standby items with --all")
	fmt.Fprintln(out, "  discard    Discard a queued item by ID")
	fmt.Fprintln(out, "  diff       Show PR metadata and git diff for a queued item")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "List flags:")
	fmt.Fprintln(out, "  --status <status>   Filter by status: standby, released, discarded, failed")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Release flags:")
	fmt.Fprintln(out, "  --all               Release all standby items in topological branch order")
}

func handleDeliveryList(args []string, out io.Writer, q delivery.Queue) error {
	fs := flag.NewFlagSet("delivery-list", flag.ContinueOnError)
	statusFlag := fs.String("status", "", "Filter by status: standby, released, discarded, failed")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var filter []delivery.DeliveryStatus
	if *statusFlag != "" {
		filter = append(filter, delivery.DeliveryStatus(*statusFlag))
	}

	items, err := q.List(filter...)
	if err != nil {
		return fmt.Errorf("list delivery queue: %w", err)
	}

	if len(items) == 0 {
		if *statusFlag != "" {
			fmt.Fprintf(out, "No delivery items found with status %q.\n", *statusFlag)
		} else {
			fmt.Fprintln(out, "No delivery items in queue.")
		}
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCHANGE\tBRANCH\tBASE\tSTATUS\tPR URL\tCREATED AT")
	fmt.Fprintln(w, "--\t------\t------\t----\t------\t------\t----------")

	for _, it := range items {
		prURL := it.PRURL
		if prURL == "" {
			prURL = "-"
		}
		createdAt := it.CreatedAt.Format("2006-01-02 15:04:05")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			it.ID, it.ChangeName, it.Branch, it.BaseBranch, it.Status, prURL, createdAt)
	}
	return w.Flush()
}

func handleDeliveryStatus(out io.Writer, q delivery.Queue) error {
	items, err := q.List()
	if err != nil {
		return fmt.Errorf("read delivery queue: %w", err)
	}

	var standby, released, discarded, failed int
	for _, it := range items {
		switch it.Status {
		case delivery.StatusStandby:
			standby++
		case delivery.StatusReleased:
			released++
		case delivery.StatusDiscarded:
			discarded++
		case delivery.StatusFailed:
			failed++
		}
	}

	fmt.Fprintln(out, "Delivery Queue Status Summary:")
	fmt.Fprintf(out, "  Total:     %d\n", len(items))
	fmt.Fprintf(out, "  Standby:   %d\n", standby)
	fmt.Fprintf(out, "  Released:  %d\n", released)
	fmt.Fprintf(out, "  Failed:    %d\n", failed)
	fmt.Fprintf(out, "  Discarded: %d\n", discarded)
	return nil
}

func handleDeliveryRelease(args []string, out, errOut io.Writer, q delivery.Queue, eng *delivery.Engine, repoRoot string) error {
	fs := flag.NewFlagSet("delivery-release", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Release all standby items in topological dependency order")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if eng == nil {
		casStore := gates.NewCASReceiptStore(repoRoot)
		transport := delivery.NewExecTransport(repoRoot)
		eng = delivery.NewEngine(q, transport, casStore, repoRoot)
	}

	ctx := context.Background()

	if *allFlag {
		results, err := eng.ReleaseAll(ctx)
		if err != nil {
			fmt.Fprintf(errOut, "ReleaseAll stopped with error: %v\n", err)
		}
		successCount := 0
		for _, res := range results {
			if res.Success {
				successCount++
				fmt.Fprintf(out, "✓ [%s] %s -> %s\n", res.ItemID, res.Branch, res.PRURL)
			} else {
				fmt.Fprintf(out, "✗ [%s] %s failed: %s\n", res.ItemID, res.Branch, res.Error)
			}
		}
		fmt.Fprintf(out, "Released %d item(s).\n", successCount)
		return err
	}

	targetID := fs.Arg(0)
	if targetID == "" {
		return fmt.Errorf("missing delivery item ID or --all flag")
	}

	res, err := eng.Release(ctx, targetID)
	if err != nil {
		fmt.Fprintf(errOut, "Failed to release %q: %v\n", targetID, err)
		return err
	}

	fmt.Fprintf(out, "✓ Successfully released [%s] on branch %q\n", res.ItemID, res.Branch)
	if res.PRURL != "" {
		fmt.Fprintf(out, "  Pull Request: %s\n", res.PRURL)
	}
	return nil
}

func handleDeliveryDiscard(args []string, out, errOut io.Writer, q delivery.Queue) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gaia delivery discard <id>")
	}
	id := args[0]
	if err := q.MarkDiscarded(id); err != nil {
		return fmt.Errorf("mark discarded: %w", err)
	}
	fmt.Fprintf(out, "✓ Delivery item %q marked as discarded.\n", id)
	return nil
}

func handleDeliveryDiff(args []string, out, errOut io.Writer, q delivery.Queue, repoRoot string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gaia delivery diff <id>")
	}
	id := args[0]
	item, err := q.Get(id)
	if err != nil {
		return fmt.Errorf("get delivery item: %w", err)
	}

	fmt.Fprintf(out, "Delivery Item: %s\n", item.ID)
	fmt.Fprintf(out, "Change:        %s\n", item.ChangeName)
	fmt.Fprintf(out, "Branch:        %s (target: %s)\n", item.Branch, item.BaseBranch)
	fmt.Fprintf(out, "Commit SHA:    %s\n", item.CommitSHA)
	fmt.Fprintf(out, "Receipt:       %s\n", item.ReceiptLineage)
	fmt.Fprintf(out, "Status:        %s\n", item.Status)
	fmt.Fprintf(out, "\nPR Title: %s\n", item.PRTitle)
	fmt.Fprintf(out, "PR Body:\n%s\n", item.PRBody)

	// Try to get git diff if in a git repo
	if item.BaseBranch != "" && item.Branch != "" {
		cmd := exec.Command("git", "diff", "--stat", fmt.Sprintf("%s...%s", item.BaseBranch, item.Branch))
		cmd.Dir = repoRoot
		if diffStat, err := cmd.Output(); err == nil && len(diffStat) > 0 {
			fmt.Fprintf(out, "\nGit Diff Stat:\n%s\n", string(diffStat))
		}
	}

	return nil
}

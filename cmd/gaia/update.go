package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"gaia/internal/config"
	"gaia/internal/updater"
	"gaia/internal/version"
)

var (
	testGitHubBaseURL string
	testTargetExePath string
)

func handleUpdateCLI(args []string) {
	code := RunUpdateCLI(args, os.Stdout, os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
}

// RunUpdateCLI routes the `gaia update` self-updater subcommand.
func RunUpdateCLI(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)

	check := fs.Bool("check", false, "check for updates without modifying the binary")
	backup := fs.Bool("backup", false, "backup $GAIA_HOME state before updating")
	targetVer := fs.String("version", "", "target specific release version tag (e.g. v0.5.0)")
	force := fs.Bool("force", false, "force update even if binary is up to date")

	fs.Usage = func() {
		printUpdateUsage(stdout)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gaia update: %v\n", err)
		return 2
	}

	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "gaia update: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		printUpdateUsage(stderr)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var clientOpts []updater.Option
	if testGitHubBaseURL != "" {
		clientOpts = append(clientOpts, updater.WithBaseURL(testGitHubBaseURL))
	}
	client := updater.NewClient("SalvucciFacundo/gaia", clientOpts...)

	var rel *updater.Release
	var err error
	if *targetVer != "" {
		rel, err = client.FetchReleaseByTag(ctx, *targetVer)
	} else {
		rel, err = client.FetchLatestRelease(ctx)
	}
	if err != nil {
		fmt.Fprintf(stderr, "gaia update: fetching release metadata: %v\n", err)
		return 1
	}

	currVer := version.Version
	targetVersion := rel.TagName
	isNewer := version.IsNewer(targetVersion, currVer)

	if *check {
		if isNewer {
			fmt.Fprintf(stdout, "Update available: %s -> %s\n", currVer, targetVersion)
		} else {
			fmt.Fprintf(stdout, "gaia is already up to date (%s)\n", currVer)
		}
		return 0
	}

	if !isNewer && !*force {
		fmt.Fprintf(stdout, "gaia is already up to date (%s). Use --force to re-install.\n", currVer)
		return 0
	}

	if *backup {
		gaiaHome := config.BaseHome()
		archivePath, err := updater.CreateBackup(gaiaHome, "")
		if err != nil {
			fmt.Fprintf(stderr, "gaia update: backup failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "Created backup archive at %s\n", archivePath)
	}

	binAsset, chkAsset, err := updater.FindAssetForPlatform(rel, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		fmt.Fprintf(stderr, "gaia update: resolving platform assets: %v\n", err)
		return 1
	}

	binBytes, err := client.DownloadAsset(ctx, binAsset)
	if err != nil {
		fmt.Fprintf(stderr, "gaia update: downloading binary asset: %v\n", err)
		return 1
	}

	chkBytes, err := client.DownloadAsset(ctx, chkAsset)
	if err != nil {
		fmt.Fprintf(stderr, "gaia update: downloading checksums: %v\n", err)
		return 1
	}

	if err := updater.VerifyChecksum(binBytes, binAsset.Name, string(chkBytes)); err != nil {
		fmt.Fprintf(stderr, "gaia update: checksum verification failed: %v\n", err)
		return 1
	}

	rawBin, err := updater.ExtractBinaryFromAsset(binAsset.Name, binBytes)
	if err != nil {
		fmt.Fprintf(stderr, "gaia update: extracting binary from asset: %v\n", err)
		return 1
	}

	exePath := testTargetExePath
	if exePath == "" {
		exePath, err = os.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "gaia update: determining current executable path: %v\n", err)
			return 1
		}
	}

	if err := updater.ApplyBinary(rawBin, exePath); err != nil {
		fmt.Fprintf(stderr, "gaia update: applying binary: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Successfully updated gaia to %s\n", targetVersion)
	return 0
}

func printUpdateUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: gaia update [flags]\n\n")
	fmt.Fprintf(w, "Self-update the GAIA binary from GitHub releases with checksum verification and backup.\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  --check           Check for available updates without modifying the executable\n")
	fmt.Fprintf(w, "  --backup          Create a .tar.gz snapshot of $GAIA_HOME state before updating\n")
	fmt.Fprintf(w, "  --version string  Target specific release version tag (e.g. v0.5.0)\n")
	fmt.Fprintf(w, "  --force           Force re-download and replacement even if already up to date\n")
	fmt.Fprintf(w, "  -h, --help        Show help\n")
}

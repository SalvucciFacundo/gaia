package gates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"gaia/internal/core/domain"
)

// CASReceiptStore implements ReceiptStore by storing review receipts as immutable
// Git objects (blobs and refs) when inside a Git repository, with an automatic
// graceful fallback to the filesystem (FSReceiptStore) for non-Git workspaces.
type CASReceiptStore struct {
	projectRoot string
	fsFallback  *FSReceiptStore
}

// NewCASReceiptStore creates a new CAS-backed receipt store.
func NewCASReceiptStore(projectRoot string) *CASReceiptStore {
	if projectRoot == "" {
		projectRoot = "."
	}
	return &CASReceiptStore{
		projectRoot: projectRoot,
		fsFallback:  NewFSReceiptStore(projectRoot),
	}
}

// isGitRepo checks whether projectRoot is part of an active Git worktree.
func (s *CASReceiptStore) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = s.projectRoot
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// sanitizeRefName cleans a change name to form a valid Git ref path.
func sanitizeRefName(name string) string {
	cleaned := strings.ReplaceAll(name, " ", "-")
	cleaned = strings.ReplaceAll(cleaned, "/", "-")
	cleaned = strings.ReplaceAll(cleaned, "\\", "-")
	return cleaned
}

// SaveReceipt stores the receipt in Git's object store (CAS) and mirrors it to disk.
// If not inside a Git repository, it falls back seamlessly to FSReceiptStore.
func (s *CASReceiptStore) SaveReceipt(receipt *domain.ReviewReceipt, changeName string) error {
	if receipt == nil {
		return fmt.Errorf("receipt cannot be nil")
	}

	// Always mirror to filesystem for non-git tool compatibility
	if err := s.fsFallback.SaveReceipt(receipt, changeName); err != nil {
		return fmt.Errorf("save receipt to filesystem mirror: %w", err)
	}

	// If not in a git repo, filesystem mirror is sufficient (graceful fallback)
	if !s.isGitRepo() {
		return nil
	}

	// 1. Serialize receipt to JSON bytes
	data, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("marshal receipt JSON: %w", err)
	}

	// 2. Write blob into Git Object Database (CAS): git hash-object -w --stdin
	hashCmd := exec.Command("git", "hash-object", "-w", "--stdin")
	hashCmd.Dir = s.projectRoot
	hashCmd.Stdin = bytes.NewReader(data)
	var hashOut, hashErr bytes.Buffer
	hashCmd.Stdout = &hashOut
	hashCmd.Stderr = &hashErr
	if err := hashCmd.Run(); err != nil {
		// Non-fatal: filesystem fallback already saved the receipt
		return nil
	}
	blobSHA := strings.TrimSpace(hashOut.String())

	// 3. Update Git ref: refs/gaia-reviews/<changeName> -> blobSHA
	refName := fmt.Sprintf("refs/gaia-reviews/%s", sanitizeRefName(changeName))
	updateCmd := exec.Command("git", "update-ref", refName, blobSHA)
	updateCmd.Dir = s.projectRoot
	_ = updateCmd.Run()

	return nil
}

// LatestReceipt retrieves the latest approved receipt from Git CAS refs,
// falling back to filesystem storage if not in a Git repository or if the ref is absent.
func (s *CASReceiptStore) LatestReceipt(changeName string) (*domain.ReviewReceipt, error) {
	if !s.isGitRepo() {
		return s.fsFallback.LatestReceipt(changeName)
	}

	// 1. Check Git ref: git rev-parse refs/gaia-reviews/<changeName>
	refName := fmt.Sprintf("refs/gaia-reviews/%s", sanitizeRefName(changeName))
	revCmd := exec.Command("git", "rev-parse", refName)
	revCmd.Dir = s.projectRoot
	out, err := revCmd.Output()
	if err != nil {
		// Fallback to filesystem if ref does not exist
		return s.fsFallback.LatestReceipt(changeName)
	}
	blobSHA := strings.TrimSpace(string(out))

	// 2. Read object from Git CAS: git cat-file -p <blobSHA>
	catCmd := exec.Command("git", "cat-file", "-p", blobSHA)
	catCmd.Dir = s.projectRoot
	blobData, err := catCmd.Output()
	if err != nil {
		return s.fsFallback.LatestReceipt(changeName)
	}

	var receipt domain.ReviewReceipt
	if err := json.Unmarshal(blobData, &receipt); err != nil {
		return s.fsFallback.LatestReceipt(changeName)
	}

	return &receipt, nil
}

// ListReceipts lists all review receipts from Git refs, merging with filesystem summaries.
func (s *CASReceiptStore) ListReceipts() ([]ReceiptSummary, error) {
	if !s.isGitRepo() {
		return s.fsFallback.ListReceipts()
	}

	// List refs matching refs/gaia-reviews/*
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)|%(objectname)", "refs/gaia-reviews")
	cmd.Dir = s.projectRoot
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return s.fsFallback.ListReceipts()
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var summaries []ReceiptSummary
	seen := make(map[string]bool)

	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			continue
		}
		refShort := parts[0]
		blobSHA := parts[1]
		changeName := strings.TrimPrefix(refShort, "refs/gaia-reviews/")
		changeName = strings.TrimPrefix(changeName, "gaia-reviews/")

		catCmd := exec.Command("git", "cat-file", "-p", blobSHA)
		catCmd.Dir = s.projectRoot
		blobData, catErr := catCmd.Output()
		if catErr != nil {
			continue
		}

		var receipt domain.ReviewReceipt
		if jsonErr := json.Unmarshal(blobData, &receipt); jsonErr == nil {
			seen[changeName] = true
			summaries = append(summaries, ReceiptSummary{
				ChangeName: changeName,
				State:      string(receipt.State),
				RiskLevel:  receipt.RiskLevel,
				CreatedAt:  receipt.CreatedAt,
			})
		}
	}

	// Merge any filesystem-only summaries
	fsSummaries, _ := s.fsFallback.ListReceipts()
	for _, fss := range fsSummaries {
		if !seen[fss.ChangeName] {
			summaries = append(summaries, fss)
		}
	}

	return summaries, nil
}

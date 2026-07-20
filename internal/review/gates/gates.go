// Package gates provides review gate validators that verify a valid
// review receipt is present and matches current file content. Gates
// are shell-script wrappers around gaia review validate.
package gates

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/review"
)

// GateName identifies a specific review gate.
type GateName string

const (
	PreCommitGate GateName = "pre-commit"
	PrePushGate   GateName = "pre-push"
	PrePRGate     GateName = "pre-pr"
)

// Gate represents a named review gate with validation logic.
type Gate struct {
	Name GateName
}

// GateResult is the outcome of a gate validation.
type GateResult struct {
	Passed        bool                    `json:"passed"`
	Receipt       *domain.ReviewReceipt   `json:"receipt,omitempty"`
	Reason        string                  `json:"reason"`
	SnapshotHash  string                  `json:"snapshot_hash,omitempty"`
	CheckedAt     time.Time               `json:"checked_at"`
}

// ReceiptStore abstracts receipt persistence so gates work with both
// Engram (agent runtime) and local filesystem (CLI/hooks).
type ReceiptStore interface {
	// LatestReceipt returns the most recent approved review receipt
	// for the given change name. Returns nil if no receipt exists.
	LatestReceipt(changeName string) (*domain.ReviewReceipt, error)
	// SaveReceipt persists a receipt so gate validators can find it.
	SaveReceipt(receipt *domain.ReviewReceipt, changeName string) error
	// ListReceipts returns all receipt metadata (state, date, risk level).
	ListReceipts() ([]ReceiptSummary, error)
}

// ReceiptSummary is a lightweight view of a receipt for listing.
type ReceiptSummary struct {
	ChangeName string    `json:"change_name"`
	State      string    `json:"state"`
	RiskLevel  string    `json:"risk_level"`
	CreatedAt  time.Time `json:"created_at"`
}

// Pre-defined gates.
var (
	GatePreCommit = &Gate{Name: PreCommitGate}
	GatePrePush   = &Gate{Name: PrePushGate}
	GatePrePR     = &Gate{Name: PrePRGate}
)

// Validate checks whether the current file snapshot matches the approved
// review receipt. The store provides receipt lookup; files are the paths
// relative to projectRoot that should be validated.
//
// Validation logic:
//   1. Receipt must exist
//   2. Receipt state must be "approved"
//   3. Current snapshot_hash must match the receipt's snapshot_hash
func (g *Gate) Validate(projectRoot string, files []string, store ReceiptStore) (*GateResult, error) {
	checkedAt := time.Now()

	// Load the latest receipt.
	changeName := deriveChangeName(files)
	receipt, err := store.LatestReceipt(changeName)
	if err != nil {
		return &GateResult{
			Passed:    false,
			Reason:    fmt.Sprintf("failed to load receipt: %v", err),
			CheckedAt: checkedAt,
		}, err
	}
	if receipt == nil {
		return &GateResult{
			Passed:    false,
			Reason:    fmt.Sprintf("no review receipt found for %q — run 'gaia review start' first", changeName),
			CheckedAt: checkedAt,
		}, nil
	}

	// Check receipt state.
	if receipt.State != domain.ReviewStateApproved {
		return &GateResult{
			Passed:    false,
			Receipt:   receipt,
			Reason:    fmt.Sprintf("review receipt state is %q, must be %q", receipt.State, domain.ReviewStateApproved),
			CheckedAt: checkedAt,
		}, nil
	}

	// Compute current snapshot hash.
	snapshots, err := review.SnapshotFiles(projectRoot, files)
	if err != nil {
		return &GateResult{
			Passed:    false,
			Receipt:   receipt,
			Reason:    fmt.Sprintf("failed to snapshot files: %v", err),
			CheckedAt: checkedAt,
		}, err
	}
	currentHash := review.ComputeSnapshotHash(snapshots)

	result := &GateResult{
		Receipt:      receipt,
		SnapshotHash: currentHash,
		CheckedAt:    checkedAt,
	}

	if currentHash != receipt.SnapshotHash {
		result.Passed = false
		result.Reason = fmt.Sprintf(
			"content changed since review was approved: current snapshot %s does not match receipt snapshot %s",
			currentHash, receipt.SnapshotHash,
		)
		return result, nil
	}

	result.Passed = true
	lineagePrefix := receipt.LineageID
	if len(lineagePrefix) > 16 {
		lineagePrefix = lineagePrefix[:16]
	}
	result.Reason = fmt.Sprintf("gate %q passed — receipt %s matches current content", g.Name, lineagePrefix)
	return result, nil
}

// validateGateStub validates a gate based purely on the receipt state and
// snapshot hash, without a ReceiptStore. Used internally by gate hook wrappers.
func (g *Gate) validateReceiptSnapshot(receipt *domain.ReviewReceipt, currentHash string) *GateResult {
	checkedAt := time.Now()
	if receipt.State != domain.ReviewStateApproved {
		return &GateResult{
			Passed:    false,
			Receipt:   receipt,
			Reason:    fmt.Sprintf("review receipt state is %q, must be %q", receipt.State, domain.ReviewStateApproved),
			CheckedAt: checkedAt,
		}
	}

	result := &GateResult{
		Receipt:      receipt,
		SnapshotHash: currentHash,
		CheckedAt:    checkedAt,
	}

	if currentHash != receipt.SnapshotHash {
		result.Passed = false
		result.Reason = fmt.Sprintf(
			"content changed since review was approved: snapshot %s vs receipt %s",
			currentHash, receipt.SnapshotHash,
		)
		return result
	}

	result.Passed = true
	result.Reason = fmt.Sprintf("gate %q passed", g.Name)
	return result
}

// deriveChangeName creates a human-readable change name from file paths.
func deriveChangeName(files []string) string {
	if len(files) == 0 {
		return "empty-change"
	}
	parts := strings.SplitN(files[0], "/", 2)
	if len(parts) > 1 {
		return parts[0] + "-changes"
	}
	return strings.TrimSuffix(parts[0], ".go") + "-changes"
}

// --- Filesystem-based ReceiptStore ---

// FSReceiptStore stores receipts as JSON files under .gaia/reviews/.
// This is used by the CLI because hook scripts cannot access Engram.
type FSReceiptStore struct {
	reviewDir string // .gaia/reviews/
}

// NewFSReceiptStore creates a filesystem-based receipt store rooted at
// projectRoot/.gaia/reviews/. The directory is created if missing.
func NewFSReceiptStore(projectRoot string) *FSReceiptStore {
	dir := filepath.Join(projectRoot, ".gaia", "reviews")
	return &FSReceiptStore{reviewDir: dir}
}

func (s *FSReceiptStore) ensureDir() error {
	return os.MkdirAll(s.reviewDir, 0755)
}

func (s *FSReceiptStore) LatestReceipt(changeName string) (*domain.ReviewReceipt, error) {
	// Look for receipt files with the change name prefix.
	pattern := filepath.Join(s.reviewDir, changeName+"*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob receipts: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}

	// Sort by modification time, take the most recent.
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})

	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("read receipt %s: %w", matches[0], err)
	}

	var receipt domain.ReviewReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("parse receipt %s: %w", matches[0], err)
	}
	return &receipt, nil
}

func (s *FSReceiptStore) SaveReceipt(receipt *domain.ReviewReceipt, changeName string) error {
	if err := s.ensureDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal receipt: %w", err)
	}

	// Filename: {changeName}-{lineageID prefix}.json
	filename := fmt.Sprintf("%s-%s.json", changeName, receipt.LineageID[:min(16, len(receipt.LineageID))])
	path := filepath.Join(s.reviewDir, filename)
	return os.WriteFile(path, data, 0644)
}

func (s *FSReceiptStore) ListReceipts() ([]ReceiptSummary, error) {
	entries, err := os.ReadDir(s.reviewDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read review dir: %w", err)
	}

	var summaries []ReceiptSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.reviewDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var receipt domain.ReviewReceipt
		if err := json.Unmarshal(data, &receipt); err != nil {
			continue
		}

		// Derive change name from filename.
		baseName := strings.TrimSuffix(entry.Name(), ".json")
		// Format: {changeName}-{lineagePrefix}
		parts := strings.SplitN(baseName, "-", 2)
		changeName := baseName
		if len(parts) > 0 {
			changeName = parts[0]
		}

		summaries = append(summaries, ReceiptSummary{
			ChangeName: changeName,
			State:      string(receipt.State),
			RiskLevel:  receipt.RiskLevel,
			CreatedAt:  receipt.CreatedAt,
		})
	}

	return summaries, nil
}

// ComputeFilesHash computes the snapshot hash for the given files without
// needing the full gate validation. Exported for use by the CLI.
func ComputeFilesHash(projectRoot string, files []string) (string, error) {
	snapshots, err := review.SnapshotFiles(projectRoot, files)
	if err != nil {
		return "", err
	}
	return review.ComputeSnapshotHash(snapshots), nil
}

// sha256Hex is exported for tests.
func Sha256Hex(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:])
}

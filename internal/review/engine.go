package review

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/review/agentsmd"
)

// Engine is the core review engine. It orchestrates file snapshotting,
// risk classification, lens selection and execution, and receipt
// generation. LLM calls happen only inside lens implementations via the
// LensLLM interface.
type Engine struct {
	projectRoot string
	standards   *agentsmd.Standards // team standards from AGENTS.md (nil if not found)
	llm         LensLLM
}

// Transaction tracks a single review lifecycle.
type Transaction struct {
	ID           string                 // unique transaction identifier (SHA256 prefix)
	ChangeName   string                 // human-readable change name
	State        domain.ReviewState     // current state in the review state machine
	SnapshotHash string                 // hash of the initial file snapshot
	Files        []string               // paths of files under review
	Receipt      *domain.ReviewReceipt  // populated when state reaches "approved"
	Findings     []domain.ReviewFinding // accumulated findings from lenses
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewEngine creates a review engine for the given project. It attempts
// to load AGENTS.md standards from projectRoot, walking up parent
// directories. Returns an engine even if AGENTS.md is not found
// (standards will be nil).
func NewEngine(projectRoot string, llm LensLLM) *Engine {
	standards, err := agentsmd.FindAndParse(projectRoot)
	if err != nil {
		standards = nil
	}
	return &Engine{
		projectRoot: projectRoot,
		standards:   standards,
		llm:         llm,
	}
}

// Standards returns the parsed AGENTS.md standards, or nil if not found.
func (e *Engine) Standards() *agentsmd.Standards {
	return e.standards
}

// Start initiates a review for the given files. It snapshots the files,
// creates a transaction, and sets the state to "reviewing".
func (e *Engine) Start(files []string) (*Transaction, error) {
	snapshots, err := SnapshotFiles(e.projectRoot, files)
	if err != nil {
		return nil, fmt.Errorf("start review: %w", err)
	}

	snapshotHash := ComputeSnapshotHash(snapshots)
	now := time.Now()

	tx := &Transaction{
		ID:           generateTxID(files, snapshotHash),
		ChangeName:   deriveChangeName(files),
		State:        StateReviewing,
		SnapshotHash: snapshotHash,
		Files:        files,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return tx, nil
}

// ClassifyRisk analyzes the diff and files to determine risk codes and level.
func (e *Engine) ClassifyRisk(diff string) ([]RiskCode, string) {
	codes := ClassifyRisk(diff, e.listReviewFiles())
	level := DetermineRiskLevel(codes)
	return codes, level
}

// SelectLenses determines which lenses to run based on the risk level
// and file types.
func (e *Engine) SelectLenses(riskLevel string) []string {
	return SelectLenses(riskLevel, e.listReviewFiles())
}

// listReviewFiles lists all staged files (to be set by the caller or
// from snapshot). Default implementation returns an empty list; the
// caller should provide the actual file list.
func (e *Engine) listReviewFiles() []string {
	// In practice, files are provided via Start() or passed externally.
	// This method is used by ClassifyRisk and SelectLenses which receive
	// files from outside.
	return nil
}

// RunLenses executes the selected lenses against the current snapshot.
// Each lens returns classified findings which are accumulated. The
// transaction state is advanced to findings_frozen after all lenses complete.
func (e *Engine) RunLenses(ctx context.Context, tx *Transaction, lensNames []string) ([]domain.ReviewFinding, error) {
	if err := Transition(tx.State, StateFindingsFrozen); err != nil {
		return nil, fmt.Errorf("run lenses: %w", err)
	}

	// Re-snapshot files to get current content for lenses.
	snapshots, err := SnapshotFiles(e.projectRoot, tx.Files)
	if err != nil {
		return nil, fmt.Errorf("run lenses: %w", err)
	}

	var allFindings []domain.ReviewFinding
	for _, name := range lensNames {
		lens := e.createLens(name)
		if lens == nil {
			return nil, fmt.Errorf("unknown lens: %s", name)
		}

		findings, err := lens.Analyze(ctx, snapshots)
		if err != nil {
			return nil, fmt.Errorf("lens %s: %w", name, err)
		}
		allFindings = append(allFindings, findings...)
	}

	tx.State = StateFindingsFrozen
	tx.Findings = allFindings
	tx.UpdatedAt = time.Now()
	return allFindings, nil
}

// GenerateReceipt creates a bounded review receipt from the transaction
// and findings. It computes the lineage_id, transitions state to
// approved, and returns the populated receipt.
func (e *Engine) GenerateReceipt(tx *Transaction, findings []domain.ReviewFinding, riskCodes []RiskCode, riskLevel string) (*domain.ReviewReceipt, error) {
	if err := Transition(tx.State, StateApproved); err != nil {
		// Allow generation from findings_frozen if lenses haven't yet frozen.
		if tx.State == StateFindingsFrozen {
			// Need to advance through intermediate states.
			if err := Transition(tx.State, StateEvidenceClassified); err != nil {
				return nil, fmt.Errorf("generate receipt: %w", err)
			}
			tx.State = StateEvidenceClassified
			if err := Transition(tx.State, StateReadyFinalVerification); err != nil {
				return nil, fmt.Errorf("generate receipt: %w", err)
			}
			tx.State = StateReadyFinalVerification
			if err := Transition(tx.State, StateFinalVerifying); err != nil {
				return nil, fmt.Errorf("generate receipt: %w", err)
			}
			tx.State = StateFinalVerifying
			if err := Transition(tx.State, StateApproved); err != nil {
				return nil, fmt.Errorf("generate receipt: %w", err)
			}
		} else {
			return nil, fmt.Errorf("generate receipt: %w", err)
		}
	}

	// Build selected lenses list.
	lensNames := SelectLenses(riskLevel, tx.Files)

	// Convert risk codes to strings.
	riskReasons := make([]string, len(riskCodes))
	for i, c := range riskCodes {
		riskReasons[i] = string(c)
	}

	// Compute lineage_id as SHA256 of the snapshot hash + transaction ID.
	lineageID := computeLineageID(tx.SnapshotHash, tx.ID)

	now := time.Now()
	receipt := &domain.ReviewReceipt{
		Schema:                "gentle-ai.review-receipt/v2",
		LineageID:             lineageID,
		SnapshotHash:          tx.SnapshotHash,
		SelectedLenses:        lensNames,
		RiskLevel:             riskLevel,
		RiskReasons:           riskReasons,
		CorrectionBudget:      85, // default correction budget
		CorrectionUsed:        0,
		State:                 StateApproved,
		FinalVerificationHash: "",
		Findings:              findings,
		CreatedAt:             tx.CreatedAt,
		UpdatedAt:             now,
	}

	tx.State = StateApproved
	tx.Receipt = receipt
	tx.UpdatedAt = now
	return receipt, nil
}

// createLens returns a Lens implementation for the given name, or nil
// if the name is unknown.
func (e *Engine) createLens(name string) Lens {
	switch name {
	case "review-risk":
		return NewLensRisk(e.llm)
	case "review-resilience":
		return NewLensResilience(e.llm)
	case "review-readability":
		return NewLensReadability(e.llm)
	case "review-reliability":
		return NewLensReliability(e.llm)
	default:
		return nil
	}
}

// TransactionFromStart creates a Transaction at the reviewing state
// without snapshotting (for when snapshots are already managed externally).
// Deprecated: Use Engine.Start() for full lifecycle.
func TransactionFromStart(files []string, snapshotHash string) *Transaction {
	now := time.Now()
	return &Transaction{
		ID:           generateTxID(files, snapshotHash),
		State:        StateReviewing,
		SnapshotHash: snapshotHash,
		Files:        files,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// generateTxID creates a unique transaction ID from the files and hash.
func generateTxID(files []string, snapshotHash string) string {
	h := sha256.New()
	h.Write([]byte(snapshotHash))
	for _, f := range files {
		h.Write([]byte(f))
	}
	return fmt.Sprintf("tx-%x", h.Sum(nil))[:16]
}

// deriveChangeName creates a human-readable change name from file paths.
func deriveChangeName(files []string) string {
	if len(files) == 0 {
		return "empty-review"
	}
	// Use the first top-level directory or the first file as a name hint.
	parts := strings.SplitN(files[0], "/", 2)
	if len(parts) > 1 {
		return parts[0] + "-changes"
	}
	return strings.TrimSuffix(parts[0], ".go") + "-changes"
}

// computeLineageID hashes the snapshot hash and transaction ID together.
func computeLineageID(snapshotHash, txID string) string {
	h := sha256.New()
	h.Write([]byte(snapshotHash))
	h.Write([]byte(txID))
	return fmt.Sprintf("%x", h.Sum(nil))
}

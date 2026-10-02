package delivery

import (
	"context"
	"fmt"
	"sort"

	"gaia/internal/core/domain"
	"gaia/internal/review"
	"gaia/internal/review/gates"
)

// DeliveryMode defines the execution policy for archiving and delivery.
type DeliveryMode string

const (
	DeliveryModeDeferred    DeliveryMode = "deferred"
	DeliveryModeInteractive DeliveryMode = "interactive"
	DeliveryModeAuto        DeliveryMode = "auto"
)

// ReleaseResult reports the outcome of a single delivery item release.
type ReleaseResult struct {
	ItemID  string `json:"item_id"`
	Branch  string `json:"branch"`
	PRURL   string `json:"pr_url,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// Engine coordinates queue inspection, CAS receipt verification, and remote transport.
type Engine struct {
	queue     Queue
	transport Transport
	casStore  gates.ReceiptStore
	repoRoot  string
}

// NewEngine creates a new Engine instance.
func NewEngine(queue Queue, transport Transport, casStore gates.ReceiptStore, repoRoot string) *Engine {
	return &Engine{
		queue:     queue,
		transport: transport,
		casStore:  casStore,
		repoRoot:  repoRoot,
	}
}

// Release validates the review receipt for a specific item and executes remote transport if valid.
func (e *Engine) Release(ctx context.Context, itemID string) (*ReleaseResult, error) {
	item, err := e.queue.Get(itemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	// CAS Receipt Verification
	if e.casStore != nil {
		modeStatus := review.IsEnabled(e.repoRoot)
		receipt, err := e.casStore.LatestReceipt(item.ChangeName)
		if err != nil {
			failReason := fmt.Sprintf("failed to load review receipt: %v", err)
			_ = e.queue.MarkFailed(itemID, failReason)
			return &ReleaseResult{
				ItemID:  item.ID,
				Branch:  item.Branch,
				Success: false,
				Error:   failReason,
			}, err
		}

		receiptRequired := (item.ReceiptLineage != "") || (modeStatus.Mode == review.ModeEnabled)
		if receiptRequired && receipt == nil {
			driftErr := fmt.Errorf("%w: no review receipt found for %q", ErrContentDrift, item.ChangeName)
			_ = e.queue.MarkFailed(itemID, driftErr.Error())
			return &ReleaseResult{
				ItemID:  item.ID,
				Branch:  item.Branch,
				Success: false,
				Error:   driftErr.Error(),
			}, driftErr
		}

		if receipt != nil {
			if receipt.State != domain.ReviewStateApproved {
				driftErr := fmt.Errorf("%w: review receipt is in state %q, expected %q", ErrContentDrift, receipt.State, domain.ReviewStateApproved)
				_ = e.queue.MarkFailed(itemID, driftErr.Error())
				return &ReleaseResult{
					ItemID:  item.ID,
					Branch:  item.Branch,
					Success: false,
					Error:   driftErr.Error(),
				}, driftErr
			}

			if item.ReceiptLineage != "" && receipt.LineageID != "" && item.ReceiptLineage != receipt.LineageID {
				driftErr := fmt.Errorf("%w: receipt lineage mismatch (item=%s, receipt=%s)", ErrContentDrift, item.ReceiptLineage, receipt.LineageID)
				_ = e.queue.MarkFailed(itemID, driftErr.Error())
				return &ReleaseResult{
					ItemID:  item.ID,
					Branch:  item.Branch,
					Success: false,
					Error:   driftErr.Error(),
				}, driftErr
			}
		}
	}

	// Remote Transport: Push Branch
	if err := e.transport.PushBranch(ctx, item.Branch); err != nil {
		_ = e.queue.MarkFailed(itemID, err.Error())
		return &ReleaseResult{
			ItemID:  item.ID,
			Branch:  item.Branch,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Remote Transport: Create PR
	prURL, err := e.transport.CreatePullRequest(ctx, *item)
	if err != nil {
		_ = e.queue.MarkFailed(itemID, err.Error())
		return &ReleaseResult{
			ItemID:  item.ID,
			Branch:  item.Branch,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Mark Released in Queue
	if err := e.queue.MarkReleased(itemID, prURL); err != nil {
		return &ReleaseResult{
			ItemID:  item.ID,
			Branch:  item.Branch,
			PRURL:   prURL,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &ReleaseResult{
		ItemID:  item.ID,
		Branch:  item.Branch,
		PRURL:   prURL,
		Success: true,
	}, nil
}

// ReleaseAll retrieves all standby items, orders them topologically by branch dependencies, and releases them.
func (e *Engine) ReleaseAll(ctx context.Context) ([]ReleaseResult, error) {
	items, err := e.queue.List(StatusStandby)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []ReleaseResult{}, nil
	}

	ordered := sortTopological(items)
	results := make([]ReleaseResult, 0, len(ordered))

	for _, item := range ordered {
		res, err := e.Release(ctx, item.ID)
		if err != nil {
			if res != nil {
				results = append(results, *res)
			} else {
				results = append(results, ReleaseResult{
					ItemID:  item.ID,
					Branch:  item.Branch,
					Success: false,
					Error:   err.Error(),
				})
			}
			return results, err
		}
		results = append(results, *res)
	}

	return results, nil
}

// sortTopological arranges delivery items so that base branches appear before their dependent child branches.
func sortTopological(items []DeliveryItem) []DeliveryItem {
	if len(items) <= 1 {
		return items
	}

	branchMap := make(map[string]DeliveryItem, len(items))
	indexMap := make(map[string]int, len(items))
	inDegree := make(map[string]int, len(items))
	graph := make(map[string][]string, len(items))

	for i, it := range items {
		branchMap[it.Branch] = it
		indexMap[it.Branch] = i
		inDegree[it.Branch] = 0
	}

	for _, it := range items {
		if _, exists := branchMap[it.BaseBranch]; exists {
			graph[it.BaseBranch] = append(graph[it.BaseBranch], it.Branch)
			inDegree[it.Branch]++
		}
	}

	// Ready queue holds branches with in-degree 0
	var ready []string
	for _, it := range items {
		if inDegree[it.Branch] == 0 {
			ready = append(ready, it.Branch)
		}
	}

	// Sort ready queue by original FIFO index
	sort.Slice(ready, func(i, j int) bool {
		return indexMap[ready[i]] < indexMap[ready[j]]
	})

	var sorted []DeliveryItem
	visited := make(map[string]bool, len(items))

	for len(ready) > 0 {
		currBranch := ready[0]
		ready = ready[1:]

		if visited[currBranch] {
			continue
		}
		visited[currBranch] = true
		sorted = append(sorted, branchMap[currBranch])

		for _, childBranch := range graph[currBranch] {
			inDegree[childBranch]--
			if inDegree[childBranch] == 0 {
				ready = append(ready, childBranch)
			}
		}

		// Keep ready queue ordered by initial index
		sort.Slice(ready, func(i, j int) bool {
			return indexMap[ready[i]] < indexMap[ready[j]]
		})
	}

	// If there are remaining unvisited items (e.g. circular dependency), append them
	for _, it := range items {
		if !visited[it.Branch] {
			sorted = append(sorted, it)
		}
	}

	return sorted
}

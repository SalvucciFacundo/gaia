package delivery

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Standard errors returned by the delivery subsystem.
var (
	ErrItemNotFound  = errors.New("delivery item not found")
	ErrInvalidStatus = errors.New("invalid delivery status")
	ErrContentDrift  = errors.New("local content drifted after review receipt issuance")
)

// DeliveryStatus represents the delivery lifecycle state.
type DeliveryStatus string

const (
	StatusStandby   DeliveryStatus = "standby"
	StatusReleased  DeliveryStatus = "released"
	StatusDiscarded DeliveryStatus = "discarded"
	StatusFailed    DeliveryStatus = "failed"
)

// IsTerminal returns true if the status is released or discarded.
func (s DeliveryStatus) IsTerminal() bool {
	return s == StatusReleased || s == StatusDiscarded
}

// DeliveryItem represents a single local work unit ready for remote delivery.
type DeliveryItem struct {
	ID             string         `json:"id"`                        // Unique slice identifier (e.g., del-1)
	ChangeName     string         `json:"change_name"`               // Associated OpenSpec change name
	Branch         string         `json:"branch"`                    // Local git branch name
	BaseBranch     string         `json:"base_branch"`               // Base target branch (e.g., main or parent slice branch)
	CommitSHA      string         `json:"commit_sha"`                // Local git commit hash
	ReceiptLineage string         `json:"receipt_lineage"`           // Approved review receipt lineage SHA256
	PRTitle        string         `json:"pr_title"`                  // Generated pull request title
	PRBody         string         `json:"pr_body"`                   // Generated markdown pull request body
	Status         DeliveryStatus `json:"status"`                    // Current lifecycle status
	ErrorMessage   string         `json:"error_message,omitempty"`  // Error description if delivery failed
	PRURL          string         `json:"pr_url,omitempty"`         // Created pull request URL
	CreatedAt      time.Time      `json:"created_at"`                // Enqueue timestamp
	ReleasedAt     *time.Time     `json:"released_at,omitempty"`     // Release timestamp
}

// Validate checks that required fields are present in the delivery item.
func (item *DeliveryItem) Validate() error {
	if strings.TrimSpace(item.ID) == "" {
		return fmt.Errorf("delivery item ID is required")
	}
	if strings.TrimSpace(item.Branch) == "" {
		return fmt.Errorf("delivery branch is required")
	}
	if strings.TrimSpace(item.CommitSHA) == "" {
		return fmt.Errorf("delivery commit SHA is required")
	}
	// Note: ReceiptLineage may be empty or unreviewed when review mode is disabled.
	return nil
}

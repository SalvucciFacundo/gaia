package delivery

import (
	"testing"
	"time"
)

func TestDeliveryItem_Validation(t *testing.T) {
	tests := []struct {
		name    string
		item    DeliveryItem
		wantErr bool
	}{
		{
			name: "valid delivery item",
			item: DeliveryItem{
				ID:             "del-1",
				ChangeName:     "feat-auth",
				Branch:         "feature/auth-models",
				BaseBranch:     "main",
				CommitSHA:      "a1b2c3d4e5f678901234567890abcdef12345678",
				ReceiptLineage: "sha256:789abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456",
				PRTitle:        "feat(auth): implement models",
				PRBody:         "## Summary\n...",
				Status:         StatusStandby,
				CreatedAt:      time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			item: DeliveryItem{
				Branch:         "feature/auth",
				CommitSHA:      "a1b2c3d4",
				ReceiptLineage: "sha256:123",
				Status:         StatusStandby,
			},
			wantErr: true,
		},
		{
			name: "missing Branch",
			item: DeliveryItem{
				ID:             "del-1",
				CommitSHA:      "a1b2c3d4",
				ReceiptLineage: "sha256:123",
				Status:         StatusStandby,
			},
			wantErr: true,
		},
		{
			name: "missing CommitSHA",
			item: DeliveryItem{
				ID:             "del-1",
				Branch:         "feature/auth",
				ReceiptLineage: "sha256:123",
				Status:         StatusStandby,
			},
			wantErr: true,
		},
		{
			name: "missing ReceiptLineage is permitted when unreviewed",
			item: DeliveryItem{
				ID:        "del-1",
				Branch:    "feature/auth",
				CommitSHA: "a1b2c3d4",
				Status:    StatusStandby,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeliveryStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   DeliveryStatus
		terminal bool
	}{
		{StatusStandby, false},
		{StatusFailed, false},
		{StatusReleased, true},
		{StatusDiscarded, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Errorf("IsTerminal(%s) = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}

package delivery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Queue defines the contract for persistent local outbox storage.
type Queue interface {
	Enqueue(item DeliveryItem) error
	List(filter ...DeliveryStatus) ([]DeliveryItem, error)
	Get(id string) (*DeliveryItem, error)
	MarkReleased(id string, prURL string) error
	MarkDiscarded(id string) error
	MarkFailed(id string, reason string) error
	Clear() error
}

// FileQueue is a thread-safe JSON file-backed delivery queue.
type FileQueue struct {
	mu       sync.RWMutex
	filePath string
}

// NewFileQueue creates a new FileQueue instance bound to the given file path.
func NewFileQueue(filePath string) (*FileQueue, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create delivery queue directory: %w", err)
	}
	return &FileQueue{filePath: filePath}, nil
}

// Enqueue adds a new delivery item to the queue or updates an existing item.
func (q *FileQueue) Enqueue(item DeliveryItem) error {
	if err := item.Validate(); err != nil {
		return fmt.Errorf("enqueue validation: %w", err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	items, err := q.loadUnlocked()
	if err != nil {
		return err
	}

	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.Status == "" {
		item.Status = StatusStandby
	}

	// Update existing or append new
	updated := false
	for i, existing := range items {
		if existing.ID == item.ID {
			items[i] = item
			updated = true
			break
		}
	}
	if !updated {
		items = append(items, item)
	}

	return q.saveUnlocked(items)
}

// List returns all queued items, optionally filtered by status, preserving FIFO order.
func (q *FileQueue) List(filter ...DeliveryStatus) ([]DeliveryItem, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	items, err := q.loadUnlocked()
	if err != nil {
		return nil, err
	}

	if len(filter) == 0 {
		return items, nil
	}

	filterMap := make(map[DeliveryStatus]bool)
	for _, f := range filter {
		filterMap[f] = true
	}

	var matched []DeliveryItem
	for _, item := range items {
		if filterMap[item.Status] {
			matched = append(matched, item)
		}
	}
	return matched, nil
}

// Get returns a single delivery item by ID.
func (q *FileQueue) Get(id string) (*DeliveryItem, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	items, err := q.loadUnlocked()
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.ID == id {
			copied := item
			return &copied, nil
		}
	}
	return nil, ErrItemNotFound
}

// MarkReleased marks an item as released and records its PR URL and release timestamp.
func (q *FileQueue) MarkReleased(id string, prURL string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	items, err := q.loadUnlocked()
	if err != nil {
		return err
	}

	now := time.Now()
	for i, item := range items {
		if item.ID == id {
			items[i].Status = StatusReleased
			items[i].PRURL = prURL
			items[i].ReleasedAt = &now
			items[i].ErrorMessage = ""
			return q.saveUnlocked(items)
		}
	}
	return ErrItemNotFound
}

// MarkDiscarded marks an item as discarded.
func (q *FileQueue) MarkDiscarded(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	items, err := q.loadUnlocked()
	if err != nil {
		return err
	}

	for i, item := range items {
		if item.ID == id {
			items[i].Status = StatusDiscarded
			return q.saveUnlocked(items)
		}
	}
	return ErrItemNotFound
}

// MarkFailed marks an item as failed with an error message.
func (q *FileQueue) MarkFailed(id string, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	items, err := q.loadUnlocked()
	if err != nil {
		return err
	}

	for i, item := range items {
		if item.ID == id {
			items[i].Status = StatusFailed
			items[i].ErrorMessage = reason
			return q.saveUnlocked(items)
		}
	}
	return ErrItemNotFound
}

// Clear removes all items from the queue.
func (q *FileQueue) Clear() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.saveUnlocked([]DeliveryItem{})
}

func (q *FileQueue) loadUnlocked() ([]DeliveryItem, error) {
	data, err := os.ReadFile(q.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []DeliveryItem{}, nil
		}
		return nil, fmt.Errorf("read delivery queue file: %w", err)
	}

	if len(data) == 0 {
		return []DeliveryItem{}, nil
	}

	var items []DeliveryItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("unmarshal delivery queue: %w", err)
	}
	return items, nil
}

func (q *FileQueue) saveUnlocked(items []DeliveryItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal delivery queue: %w", err)
	}
	return os.WriteFile(q.filePath, data, 0644)
}

var _ Queue = (*FileQueue)(nil)

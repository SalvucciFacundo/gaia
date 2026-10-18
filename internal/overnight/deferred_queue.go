package overnight

import (
	"fmt"
	"sync"
	"time"
)

// DeferredQueue manages intercepted sensitive actions requiring human review.
type DeferredQueue struct {
	mu      sync.RWMutex
	actions map[string][]DeferredAction // keyed by planID
}

// NewDeferredQueue creates an in-memory deferred actions queue.
func NewDeferredQueue() *DeferredQueue {
	return &DeferredQueue{
		actions: make(map[string][]DeferredAction),
	}
}

// Enqueue adds an intercepted action to the queue under the given plan ID.
func (q *DeferredQueue) Enqueue(planID, toolName string, args map[string]interface{}, reason string) DeferredAction {
	q.mu.Lock()
	defer q.mu.Unlock()

	action := DeferredAction{
		ID:        fmt.Sprintf("act-%d-%d", time.Now().UnixNano(), len(q.actions[planID])+1),
		PlanID:    planID,
		ToolName:  toolName,
		Arguments: args,
		Reason:    reason,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	q.actions[planID] = append(q.actions[planID], action)
	return action
}

// List returns all deferred actions for the specified plan ID.
func (q *DeferredQueue) List(planID string) []DeferredAction {
	q.mu.RLock()
	defer q.mu.RUnlock()

	list := q.actions[planID]
	res := make([]DeferredAction, len(list))
	copy(res, list)
	return res
}

// UpdateStatus marks an action as approved or rejected.
func (q *DeferredQueue) UpdateStatus(planID, actionID, status string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	list, ok := q.actions[planID]
	if !ok {
		return fmt.Errorf("plan %q not found in queue", planID)
	}

	for i, act := range list {
		if act.ID == actionID {
			list[i].Status = status
			return nil
		}
	}

	return fmt.Errorf("action %q not found in plan %q", actionID, planID)
}

// Clear clears actions for a completed or discarded plan.
func (q *DeferredQueue) Clear(planID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.actions, planID)
}

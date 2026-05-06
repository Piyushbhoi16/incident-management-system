package domain

import "time"

// WorkItemStatus is the lifecycle of an incident work item in PostgreSQL.
type WorkItemStatus string

const (
	StatusOpen          WorkItemStatus = "OPEN"
	StatusInvestigating WorkItemStatus = "INVESTIGATING"
	StatusResolved      WorkItemStatus = "RESOLVED"
	StatusClosed        WorkItemStatus = "CLOSED"
)

// WorkItem is the durable incident record (PostgreSQL). id matches the UUID
// stored in Redis debounce and referenced from MongoDB raw_signals.work_item_id.
// MTTR is set when an RCA is saved (end_time - start_time); nil until then.
type WorkItem struct {
	ID          string // UUID string (same as debounce work_item_id)
	ComponentID string
	Severity    Severity
	Status      WorkItemStatus
	MTTR        *time.Duration
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

package domain

import (
	"fmt"
	"strings"
	"time"
)

// RCA is the root-cause analysis record for a work item (one per work item).
type RCA struct {
	WorkItemID string
	StartTime  time.Time
	EndTime    time.Time
	RootCause  string
	Fix        string
	Prevention string
}

// Validate checks all required fields and time ordering before persistence.
func (r RCA) Validate() error {
	if strings.TrimSpace(r.WorkItemID) == "" {
		return fmt.Errorf("work_item_id is required")
	}
	if r.StartTime.IsZero() {
		return fmt.Errorf("start_time is required")
	}
	if r.EndTime.IsZero() {
		return fmt.Errorf("end_time is required")
	}
	if !r.EndTime.After(r.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	if strings.TrimSpace(r.RootCause) == "" {
		return fmt.Errorf("root_cause is required")
	}
	if strings.TrimSpace(r.Fix) == "" {
		return fmt.Errorf("fix is required")
	}
	if strings.TrimSpace(r.Prevention) == "" {
		return fmt.Errorf("prevention is required")
	}
	return nil
}

// MTTR returns end_time - start_time for a valid RCA.
func (r RCA) MTTR() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

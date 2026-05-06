package domain

import "time"

type Severity string

const (
	SeverityP0 Severity = "P0"
	SeverityP1 Severity = "P1"
	SeverityP2 Severity = "P2"
)

type Signal struct {
	RequestID   string    `json:"request_id,omitempty"`
	ComponentID string    `json:"component_id"`
	Severity    Severity  `json:"severity"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

func IsValidSeverity(severity Severity) bool {
	switch severity {
	case SeverityP0, SeverityP1, SeverityP2:
		return true
	default:
		return false
	}
}

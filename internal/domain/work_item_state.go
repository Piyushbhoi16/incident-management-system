package domain

import "fmt"

type WorkItemState interface {
	Status() WorkItemStatus
	CanTransitionTo(next WorkItemStatus) bool
}

type openState struct{}
type investigatingState struct{}
type resolvedState struct{}
type closedState struct{}

func (openState) Status() WorkItemStatus { return StatusOpen }
func (openState) CanTransitionTo(next WorkItemStatus) bool {
	return next == StatusInvestigating
}

func (investigatingState) Status() WorkItemStatus { return StatusInvestigating }
func (investigatingState) CanTransitionTo(next WorkItemStatus) bool {
	return next == StatusResolved
}

func (resolvedState) Status() WorkItemStatus { return StatusResolved }
func (resolvedState) CanTransitionTo(next WorkItemStatus) bool {
	return next == StatusClosed
}

func (closedState) Status() WorkItemStatus { return StatusClosed }
func (closedState) CanTransitionTo(WorkItemStatus) bool {
	return false
}

func StateFor(status WorkItemStatus) (WorkItemState, error) {
	switch status {
	case StatusOpen:
		return openState{}, nil
	case StatusInvestigating:
		return investigatingState{}, nil
	case StatusResolved:
		return resolvedState{}, nil
	case StatusClosed:
		return closedState{}, nil
	default:
		return nil, fmt.Errorf("unknown work item status: %s", status)
	}
}

func IsValidWorkItemStatus(status WorkItemStatus) bool {
	_, err := StateFor(status)
	return err == nil
}

package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"ims/internal/repositories"
)

const debounceTTL = 10 * time.Second

type DebounceService struct {
	repo repositories.DebounceRepository
}

type DebounceResult struct {
	WorkItemID string
	Created    bool
}

func NewDebounceService(repo repositories.DebounceRepository) *DebounceService {
	return &DebounceService{repo: repo}
}

// GetOrCreateWorkItemID returns the work_item_id for this component's debounce window.
// On a new burst it generates a UUID v4, stores it as the Redis key value, and returns it.
// If the debounce key already exists, it returns the id read from Redis so MongoDB and
// PostgreSQL always agree on the same identifier.
func (s *DebounceService) GetOrCreateWorkItemID(ctx context.Context, componentID string) (DebounceResult, error) {
	candidate := uuid.NewString()

	workItemID, created, err := s.repo.GetOrCreateWorkItemID(ctx, componentID, candidate, debounceTTL)
	if err != nil {
		return DebounceResult{}, fmt.Errorf("debounce work item id: %w", err)
	}

	return DebounceResult{
		WorkItemID: workItemID,
		Created:    created,
	}, nil
}

package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"ims/internal/domain"
	"ims/internal/repositories"
)

const activeWorkItemsCacheTTL = 5 * time.Second

type WorkItemService struct {
	repo   repositories.WorkItemRepository
	rca    repositories.RCAReader
	cache  repositories.WorkItemCache
	logger *slog.Logger
}

// NewWorkItemService wires work item persistence with optional RCAReader for CLOSED enforcement.
// Pass nil rca only in isolated tests that never transition to CLOSED.
func NewWorkItemService(repo repositories.WorkItemRepository, rca repositories.RCAReader) *WorkItemService {
	return NewWorkItemServiceWithLogger(repo, rca, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

func NewWorkItemServiceWithLogger(repo repositories.WorkItemRepository, rca repositories.RCAReader, logger *slog.Logger) *WorkItemService {
	return &WorkItemService{repo: repo, rca: rca, logger: logger}
}

func NewWorkItemServiceWithCache(repo repositories.WorkItemRepository, rca repositories.RCAReader, cache repositories.WorkItemCache) *WorkItemService {
	service := NewWorkItemService(repo, rca)
	service.cache = cache
	return service
}

// CreateOpen inserts OPEN work item with the given id (must equal debounceResult.WorkItemID from Redis).
func (s *WorkItemService) CreateOpen(ctx context.Context, id, componentID string, severity domain.Severity) error {
	item := domain.WorkItem{
		ID:          id,
		ComponentID: componentID,
		Severity:    severity,
		Status:      domain.StatusOpen,
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return fmt.Errorf("create open work item: %w", err)
	}
	s.invalidateActiveCache(ctx)

	return nil
}

func (s *WorkItemService) ListActive(ctx context.Context) ([]domain.WorkItem, error) {
	if s.cache != nil {
		items, err := s.cache.GetActive(ctx)
		if err == nil {
			sortWorkItemsBySeverity(items)
			return items, nil
		}
		if !errors.Is(err, repositories.ErrWorkItemCacheMiss) {
			s.logger.Error("active_work_items_cache_read_failed", slog.String("error", err.Error()))
		}
	}

	items, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active work items: %w", err)
	}
	sortWorkItemsBySeverity(items)

	if s.cache != nil {
		if err := s.cache.SetActive(ctx, items, activeWorkItemsCacheTTL); err != nil {
			s.logger.Error("active_work_items_cache_write_failed", slog.String("error", err.Error()))
		}
	}

	return items, nil
}

func (s *WorkItemService) GetByID(ctx context.Context, id string) (domain.WorkItem, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.WorkItem{}, fmt.Errorf("get work item: %w", err)
	}

	return item, nil
}

func (s *WorkItemService) Transition(ctx context.Context, id string, next domain.WorkItemStatus) (domain.WorkItem, error) {
	if !domain.IsValidWorkItemStatus(next) {
		return domain.WorkItem{}, fmt.Errorf("invalid target status: %s", next)
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.WorkItem{}, fmt.Errorf("load work item: %w", err)
	}

	// State machine: each status allows exactly one forward edge (no skip, no backward).
	state, err := domain.StateFor(current.Status)
	if err != nil {
		return domain.WorkItem{}, err
	}
	if !state.CanTransitionTo(next) {
		return domain.WorkItem{}, fmt.Errorf("invalid transition: %s -> %s", current.Status, next)
	}

	// Business rule: CLOSED requires a persisted RCA for this work item.
	if next == domain.StatusClosed {
		if s.rca == nil {
			return domain.WorkItem{}, fmt.Errorf("cannot transition to CLOSED: RCA verification unavailable")
		}
		has, err := s.rca.HasRCAForWorkItem(ctx, id)
		if err != nil {
			return domain.WorkItem{}, fmt.Errorf("check RCA for close: %w", err)
		}
		if !has {
			return domain.WorkItem{}, fmt.Errorf("cannot transition to CLOSED without RCA")
		}
	}

	updated, err := s.repo.UpdateStatus(ctx, id, next)
	if err != nil {
		return domain.WorkItem{}, fmt.Errorf("persist transition: %w", err)
	}
	s.invalidateActiveCache(ctx)

	s.logger.Info(
		"work_item_state_transition",
		slog.String("work_item_id", id),
		slog.String("from_status", string(current.Status)),
		slog.String("to_status", string(next)),
	)

	return updated, nil
}

func (s *WorkItemService) invalidateActiveCache(ctx context.Context) {
	if s.cache == nil {
		return
	}
	if err := s.cache.DeleteActive(ctx); err != nil {
		s.logger.Error("active_work_items_cache_delete_failed", slog.String("error", err.Error()))
	}
}

func sortWorkItemsBySeverity(items []domain.WorkItem) {
	sort.SliceStable(items, func(i, j int) bool {
		left := severityRank(items[i].Severity)
		right := severityRank(items[j].Severity)
		if left != right {
			return left < right
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
}

func severityRank(severity domain.Severity) int {
	switch severity {
	case domain.SeverityP0:
		return 0
	case domain.SeverityP1:
		return 1
	case domain.SeverityP2:
		return 2
	default:
		return 3
	}
}

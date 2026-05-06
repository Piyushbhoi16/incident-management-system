package services

import (
	"context"
	"fmt"

	"ims/internal/domain"
	"ims/internal/repositories"
)

type RCAService struct {
	rcaRepo      repositories.RCARepository
	workItemRepo repositories.WorkItemRepository
}

func NewRCAService(rcaRepo repositories.RCARepository, workItemRepo repositories.WorkItemRepository) *RCAService {
	return &RCAService{rcaRepo: rcaRepo, workItemRepo: workItemRepo}
}

// Create validates the RCA payload, persists it, and writes MTTR (end_time - start_time) on the work item.
func (s *RCAService) Create(ctx context.Context, rca domain.RCA) error {
	if err := rca.Validate(); err != nil {
		return fmt.Errorf("validate RCA: %w", err)
	}

	has, err := s.rcaRepo.HasRCAForWorkItem(ctx, rca.WorkItemID)
	if err != nil {
		return fmt.Errorf("check existing RCA: %w", err)
	}
	if has {
		return fmt.Errorf("RCA already exists for work item %s", rca.WorkItemID)
	}

	if err := s.rcaRepo.Create(ctx, rca); err != nil {
		return fmt.Errorf("save RCA: %w", err)
	}

	mttrNs := rca.MTTR().Nanoseconds()
	if err := s.workItemRepo.UpdateMTTR(ctx, rca.WorkItemID, mttrNs); err != nil {
		return fmt.Errorf("update work item MTTR: %w", err)
	}

	return nil
}

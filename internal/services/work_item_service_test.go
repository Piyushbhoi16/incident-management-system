package services

import (
	"context"
	"testing"
	"time"

	"ims/internal/domain"
)

type fakeStateWorkItemRepo struct {
	item domain.WorkItem
}

func (r *fakeStateWorkItemRepo) Create(context.Context, domain.WorkItem) error { return nil }
func (r *fakeStateWorkItemRepo) GetByID(context.Context, string) (domain.WorkItem, error) {
	return r.item, nil
}
func (r *fakeStateWorkItemRepo) ListActive(context.Context) ([]domain.WorkItem, error) {
	return []domain.WorkItem{r.item}, nil
}
func (r *fakeStateWorkItemRepo) UpdateStatus(_ context.Context, _ string, next domain.WorkItemStatus) (domain.WorkItem, error) {
	r.item.Status = next
	r.item.UpdatedAt = time.Now().UTC()
	return r.item, nil
}
func (r *fakeStateWorkItemRepo) UpdateMTTR(context.Context, string, int64) error { return nil }

type fakeStateRCAReader struct {
	has bool
}

func (r *fakeStateRCAReader) HasRCAForWorkItem(context.Context, string) (bool, error) { return r.has, nil }

func TestTransitionRejectsSkippedState(t *testing.T) {
	repo := &fakeStateWorkItemRepo{item: domain.WorkItem{ID: "wid-1", Status: domain.StatusOpen}}
	svc := NewWorkItemService(repo, &fakeStateRCAReader{has: true})

	if _, err := svc.Transition(context.Background(), "wid-1", domain.StatusResolved); err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestTransitionToClosedRequiresRCA(t *testing.T) {
	repo := &fakeStateWorkItemRepo{item: domain.WorkItem{ID: "wid-1", Status: domain.StatusResolved}}
	svc := NewWorkItemService(repo, &fakeStateRCAReader{has: false})

	if _, err := svc.Transition(context.Background(), "wid-1", domain.StatusClosed); err == nil {
		t.Fatal("expected RCA requirement error")
	}
}

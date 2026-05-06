package services

import (
	"context"
	"testing"
	"time"

	"ims/internal/domain"
)

type fakeRCARepo struct {
	has    bool
	saved  []domain.RCA
	hasErr error
}

func (r *fakeRCARepo) EnsureSchema(context.Context) error { return nil }
func (r *fakeRCARepo) HasRCAForWorkItem(_ context.Context, _ string) (bool, error) {
	return r.has, r.hasErr
}
func (r *fakeRCARepo) Create(_ context.Context, rca domain.RCA) error {
	r.saved = append(r.saved, rca)
	return nil
}

type fakeWorkItemRepo struct {
	mttrByID map[string]int64
}

func (r *fakeWorkItemRepo) Create(context.Context, domain.WorkItem) error { return nil }
func (r *fakeWorkItemRepo) GetByID(context.Context, string) (domain.WorkItem, error) {
	return domain.WorkItem{}, nil
}
func (r *fakeWorkItemRepo) ListActive(context.Context) ([]domain.WorkItem, error) {
	return nil, nil
}
func (r *fakeWorkItemRepo) UpdateStatus(context.Context, string, domain.WorkItemStatus) (domain.WorkItem, error) {
	return domain.WorkItem{}, nil
}
func (r *fakeWorkItemRepo) UpdateMTTR(_ context.Context, id string, mttrNs int64) error {
	if r.mttrByID == nil {
		r.mttrByID = map[string]int64{}
	}
	r.mttrByID[id] = mttrNs
	return nil
}

func TestRCAServiceCreateStoresRCAAndMTTR(t *testing.T) {
	rcaRepo := &fakeRCARepo{}
	workItems := &fakeWorkItemRepo{}
	svc := NewRCAService(rcaRepo, workItems)

	start := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	end := start.Add(42 * time.Minute)
	rca := domain.RCA{
		WorkItemID: "11111111-1111-1111-1111-111111111111",
		StartTime:  start,
		EndTime:    end,
		RootCause:  "Cache node exhaustion",
		Fix:        "Replaced failing node",
		Prevention: "Capacity alerting and autoscaling policy",
	}

	if err := svc.Create(context.Background(), rca); err != nil {
		t.Fatalf("expected create success, got %v", err)
	}
	if len(rcaRepo.saved) != 1 {
		t.Fatalf("expected 1 RCA saved, got %d", len(rcaRepo.saved))
	}
	if got := workItems.mttrByID[rca.WorkItemID]; got != rca.MTTR().Nanoseconds() {
		t.Fatalf("expected mttr %d, got %d", rca.MTTR().Nanoseconds(), got)
	}
}

func TestRCAServiceCreateRejectsInvalidRCA(t *testing.T) {
	svc := NewRCAService(&fakeRCARepo{}, &fakeWorkItemRepo{})
	err := svc.Create(context.Background(), domain.RCA{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

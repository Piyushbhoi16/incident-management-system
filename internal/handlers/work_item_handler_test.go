package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ims/internal/domain"
	"ims/internal/repositories"
	"ims/internal/services"
)

type fakeWorkItemRepository struct {
	item  domain.WorkItem
	items []domain.WorkItem
	err   error
}

func (r *fakeWorkItemRepository) Create(_ context.Context, item domain.WorkItem) error {
	r.item = item
	return nil
}

func (r *fakeWorkItemRepository) GetByID(_ context.Context, _ string) (domain.WorkItem, error) {
	if r.err != nil {
		return domain.WorkItem{}, r.err
	}
	return r.item, nil
}

func (r *fakeWorkItemRepository) ListActive(_ context.Context) ([]domain.WorkItem, error) {
	var active []domain.WorkItem
	for _, item := range r.items {
		if item.Status != domain.StatusClosed {
			active = append(active, item)
		}
	}
	return active, nil
}

func (r *fakeWorkItemRepository) UpdateStatus(_ context.Context, _ string, status domain.WorkItemStatus) (domain.WorkItem, error) {
	r.item.Status = status
	r.item.UpdatedAt = time.Date(2026, 5, 4, 10, 30, 0, 0, time.UTC)
	return r.item, nil
}

func (r *fakeWorkItemRepository) UpdateMTTR(_ context.Context, _ string, _ int64) error {
	return nil
}

type fakeRCAReader struct {
	has bool
}

func (r *fakeRCAReader) HasRCAForWorkItem(_ context.Context, _ string) (bool, error) {
	return r.has, nil
}

func TestUpdateWorkItemStatusAcceptedTransition(t *testing.T) {
	router, repo := newWorkItemTestRouter()
	repo.item = domain.WorkItem{
		ID:          "11111111-1111-1111-1111-111111111111",
		ComponentID: "CACHE_CLUSTER_01",
		Severity:    domain.SeverityP2,
		Status:      domain.StatusOpen,
		CreatedAt:   time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/work-items/11111111-1111-1111-1111-111111111111/status",
		bytes.NewReader([]byte(`{"status":"INVESTIGATING"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if repo.item.Status != domain.StatusInvestigating {
		t.Fatalf("expected status %s, got %s", domain.StatusInvestigating, repo.item.Status)
	}
}

func TestUpdateWorkItemStatusRejectsSkippedTransition(t *testing.T) {
	router, repo := newWorkItemTestRouter()
	repo.item = domain.WorkItem{
		ID:          "11111111-1111-1111-1111-111111111111",
		ComponentID: "CACHE_CLUSTER_01",
		Severity:    domain.SeverityP2,
		Status:      domain.StatusOpen,
		CreatedAt:   time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/work-items/11111111-1111-1111-1111-111111111111/status",
		bytes.NewReader([]byte(`{"status":"RESOLVED"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if repo.item.Status != domain.StatusOpen {
		t.Fatalf("expected status %s, got %s", domain.StatusOpen, repo.item.Status)
	}
}

func TestGetWorkItemByIDReturnsSuccessResponse(t *testing.T) {
	router, repo := newWorkItemTestRouter()
	repo.item = domain.WorkItem{
		ID:          "11111111-1111-1111-1111-111111111111",
		ComponentID: "CACHE_CLUSTER_01",
		Severity:    domain.SeverityP1,
		Status:      domain.StatusInvestigating,
		CreatedAt:   time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 4, 10, 30, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(http.MethodGet, "/work-items/11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	for _, expected := range []string{
		`"status":"success"`,
		`"component_id":"CACHE_CLUSTER_01"`,
		`"status":"INVESTIGATING"`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(expected)) {
			t.Fatalf("expected response to include %s, got %s", expected, rec.Body.String())
		}
	}
}

func TestGetWorkItemByIDReturnsNotFound(t *testing.T) {
	router, repo := newWorkItemTestRouter()
	repo.err = repositories.ErrWorkItemNotFound

	req := httptest.NewRequest(http.MethodGet, "/work-items/11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestListActiveWorkItemsReturnsDashboardIncidentsBySeverity(t *testing.T) {
	router, repo := newWorkItemTestRouter()
	baseTime := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	repo.items = []domain.WorkItem{
		{
			ID:          "22222222-2222-2222-2222-222222222222",
			ComponentID: "CACHE_CLUSTER_01",
			Severity:    domain.SeverityP2,
			Status:      domain.StatusOpen,
			CreatedAt:   baseTime.Add(2 * time.Minute),
		},
		{
			ID:          "33333333-3333-3333-3333-333333333333",
			ComponentID: "API_GATEWAY_01",
			Severity:    domain.SeverityP0,
			Status:      domain.StatusInvestigating,
			CreatedAt:   baseTime,
		},
		{
			ID:          "44444444-4444-4444-4444-444444444444",
			ComponentID: "WORKER_01",
			Severity:    domain.SeverityP1,
			Status:      domain.StatusClosed,
			CreatedAt:   baseTime.Add(time.Minute),
		},
		{
			ID:          "55555555-5555-5555-5555-555555555555",
			ComponentID: "DB_PRIMARY_01",
			Severity:    domain.SeverityP1,
			Status:      domain.StatusResolved,
			CreatedAt:   baseTime.Add(3 * time.Minute),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/work-items", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"status":"success"`)) {
		t.Fatalf("expected success wrapper, got %s", rec.Body.String())
	}

	expectedOrder := []string{"API_GATEWAY_01", "DB_PRIMARY_01", "CACHE_CLUSTER_01"}
	for _, componentID := range expectedOrder {
		if !bytes.Contains(rec.Body.Bytes(), []byte(componentID)) {
			t.Fatalf("expected response to include component %s, got %s", componentID, rec.Body.String())
		}
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("WORKER_01")) {
		t.Fatalf("expected closed work item to be excluded, got %s", rec.Body.String())
	}
	if first, second := bytes.Index(rec.Body.Bytes(), []byte(expectedOrder[0])), bytes.Index(rec.Body.Bytes(), []byte(expectedOrder[1])); first > second {
		t.Fatalf("expected %s before %s, got %s", expectedOrder[0], expectedOrder[1], rec.Body.String())
	}
	if second, third := bytes.Index(rec.Body.Bytes(), []byte(expectedOrder[1])), bytes.Index(rec.Body.Bytes(), []byte(expectedOrder[2])); second > third {
		t.Fatalf("expected %s before %s, got %s", expectedOrder[1], expectedOrder[2], rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"work_item_id":"33333333-3333-3333-3333-333333333333"`)) {
		t.Fatalf("expected work_item_id in active list response, got %s", rec.Body.String())
	}
}

func TestUpdateWorkItemStatusRejectsCloseWithoutRCA(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &fakeWorkItemRepository{
		item: domain.WorkItem{
			ID:          "11111111-1111-1111-1111-111111111111",
			ComponentID: "CACHE_CLUSTER_01",
			Severity:    domain.SeverityP2,
			Status:      domain.StatusResolved,
			CreatedAt:   time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
		},
	}
	service := services.NewWorkItemService(repo, &fakeRCAReader{has: false})
	handler := NewWorkItemHandler(service)
	router := gin.New()
	router.PATCH("/work-items/:id/status", handler.UpdateStatus)

	req := httptest.NewRequest(
		http.MethodPatch,
		"/work-items/11111111-1111-1111-1111-111111111111/status",
		bytes.NewReader([]byte(`{"status":"CLOSED"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if repo.item.Status != domain.StatusResolved {
		t.Fatalf("expected status %s, got %s", domain.StatusResolved, repo.item.Status)
	}
}

func newWorkItemTestRouter() (*gin.Engine, *fakeWorkItemRepository) {
	gin.SetMode(gin.TestMode)

	repo := &fakeWorkItemRepository{}
	service := services.NewWorkItemService(repo, &fakeRCAReader{has: true})
	handler := NewWorkItemHandler(service)

	router := gin.New()
	router.GET("/work-items", handler.ListActive)
	router.GET("/work-items/:id", handler.GetByID)
	router.PATCH("/work-items/:id/status", handler.UpdateStatus)

	return router, repo
}

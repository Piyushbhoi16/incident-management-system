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
	"ims/internal/services"
)

type fakeRCAHandlerRepo struct {
	has   bool
	saved []domain.RCA
}

func (r *fakeRCAHandlerRepo) EnsureSchema(context.Context) error { return nil }
func (r *fakeRCAHandlerRepo) HasRCAForWorkItem(_ context.Context, _ string) (bool, error) {
	return r.has, nil
}
func (r *fakeRCAHandlerRepo) Create(_ context.Context, rca domain.RCA) error {
	r.saved = append(r.saved, rca)
	return nil
}

type fakeRCAWorkItemRepo struct {
	mttrByID map[string]int64
}

func (r *fakeRCAWorkItemRepo) Create(context.Context, domain.WorkItem) error { return nil }
func (r *fakeRCAWorkItemRepo) GetByID(context.Context, string) (domain.WorkItem, error) {
	return domain.WorkItem{}, nil
}
func (r *fakeRCAWorkItemRepo) ListActive(context.Context) ([]domain.WorkItem, error) {
	return nil, nil
}
func (r *fakeRCAWorkItemRepo) UpdateStatus(context.Context, string, domain.WorkItemStatus) (domain.WorkItem, error) {
	return domain.WorkItem{}, nil
}
func (r *fakeRCAWorkItemRepo) UpdateMTTR(_ context.Context, id string, mttrNs int64) error {
	if r.mttrByID == nil {
		r.mttrByID = map[string]int64{}
	}
	r.mttrByID[id] = mttrNs
	return nil
}

func TestCreateRCAAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rcaRepo := &fakeRCAHandlerRepo{}
	workItems := &fakeRCAWorkItemRepo{}
	service := services.NewRCAService(rcaRepo, workItems)
	handler := NewRCAHandler(service)
	router := gin.New()
	router.POST("/rca", handler.Create)

	start := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	body := []byte(`{
		"work_item_id":"11111111-1111-1111-1111-111111111111",
		"start_time":"` + start.Format(time.RFC3339) + `",
		"end_time":"` + end.Format(time.RFC3339) + `",
		"root_cause":"Database connection pool exhaustion",
		"fix":"Increased max connections and restarted pods",
		"prevention":"Pool saturation alerts and load-test gate"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/rca", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}
	if len(rcaRepo.saved) != 1 {
		t.Fatalf("expected 1 RCA saved, got %d", len(rcaRepo.saved))
	}
}

func TestCreateRCARejectsInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := services.NewRCAService(&fakeRCAHandlerRepo{}, &fakeRCAWorkItemRepo{})
	handler := NewRCAHandler(service)
	router := gin.New()
	router.POST("/rca", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/rca", bytes.NewReader([]byte(`{"work_item_id":""}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

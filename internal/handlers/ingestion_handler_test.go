package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ims/internal/domain"
	"ims/internal/services"
)

type fakeSignalQueue struct {
	signals []domain.Signal
	err     error
}

func (q *fakeSignalQueue) Enqueue(_ context.Context, signal domain.Signal) error {
	if q.err != nil {
		return q.err
	}

	q.signals = append(q.signals, signal)
	return nil
}

func TestIngestAcceptedSignal(t *testing.T) {
	router, queue := newIngestionTestRouter(nil)

	body := []byte(`{
		"component_id": "CACHE_CLUSTER_01",
		"severity": "P2",
		"message": "High latency detected",
		"timestamp": "2026-05-03T10:15:00Z"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
	if len(queue.signals) != 1 {
		t.Fatalf("expected 1 enqueued signal, got %d", len(queue.signals))
	}
	if queue.signals[0].ComponentID != "CACHE_CLUSTER_01" {
		t.Fatalf("expected component id CACHE_CLUSTER_01, got %q", queue.signals[0].ComponentID)
	}
}

func TestIngestRejectsInvalidSignal(t *testing.T) {
	router, queue := newIngestionTestRouter(nil)

	body := []byte(`{
		"component_id": "",
		"severity": "P9",
		"message": "",
		"timestamp": "not-a-date"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if len(queue.signals) != 0 {
		t.Fatalf("expected no enqueued signals, got %d", len(queue.signals))
	}
}

func TestIngestReturnsUnavailableWhenQueueFails(t *testing.T) {
	router, queue := newIngestionTestRouter(errors.New("redis unavailable"))

	body := []byte(`{
		"component_id": "API_GATEWAY_01",
		"severity": "P1",
		"message": "Error rate spike",
		"timestamp": "2026-05-03T10:15:00Z"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if len(queue.signals) != 0 {
		t.Fatalf("expected no enqueued signals, got %d", len(queue.signals))
	}
}

func newIngestionTestRouter(queueErr error) (*gin.Engine, *fakeSignalQueue) {
	gin.SetMode(gin.TestMode)

	queue := &fakeSignalQueue{err: queueErr}
	service := services.NewIngestionService(queue)
	handler := NewIngestionHandler(service)

	router := gin.New()
	router.POST("/ingest", handler.Ingest)

	return router, queue
}

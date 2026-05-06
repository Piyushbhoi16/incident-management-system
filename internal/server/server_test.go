package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ims/internal/config"
	"ims/internal/handlers"
)

func TestHealthEndpoint(t *testing.T) {
	router := New(config.Config{
		AppName:  "ims-backend",
		Env:      "test",
		HTTPAddr: ":0",
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body handlers.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %q", body.Status)
	}
	if body.Service != "ims-backend" {
		t.Fatalf("expected service ims-backend, got %q", body.Service)
	}
	if body.Time == "" {
		t.Fatal("expected time to be set")
	}
}

package services

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"ims/internal/domain"
	"ims/internal/requestctx"
)

type fakeQueue struct {
	err      error
	block    bool
	deadline bool
}

func (q *fakeQueue) Enqueue(ctx context.Context, _ domain.Signal) error {
	if _, ok := ctx.Deadline(); ok {
		q.deadline = true
	}

	if q.block {
		<-ctx.Done()
		return ctx.Err()
	}

	return q.err
}

func TestIngestUsesQueuePublishTimeout(t *testing.T) {
	queue := &fakeQueue{block: true}
	service := NewIngestionServiceWithLogger(queue, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))

	start := time.Now()
	err := service.Ingest(context.Background(), testSignal())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !queue.deadline {
		t.Fatal("expected queue context to include a deadline")
	}
	if elapsed < queuePublishTimeout {
		t.Fatalf("expected timeout after at least %s, got %s", queuePublishTimeout, elapsed)
	}
	if elapsed > queuePublishTimeout+500*time.Millisecond {
		t.Fatalf("expected timeout near %s, got %s", queuePublishTimeout, elapsed)
	}
}

func TestIngestLogsQueuePushFailure(t *testing.T) {
	var logs bytes.Buffer
	queue := &fakeQueue{err: errors.New("redis down")}
	service := NewIngestionServiceWithLogger(queue, slog.New(slog.NewJSONHandler(&logs, nil)))

	ctx := requestctx.WithRequestID(context.Background(), "req-123")
	err := service.Ingest(ctx, testSignal())

	if err == nil {
		t.Fatal("expected queue error")
	}

	output := logs.String()
	for _, expected := range []string{
		"signal_enqueue_failed",
		"req-123",
		"CACHE_CLUSTER_01",
		"P2",
		"redis down",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log to contain %q, got %s", expected, output)
		}
	}
}

func testSignal() domain.Signal {
	return domain.Signal{
		ComponentID: "CACHE_CLUSTER_01",
		Severity:    domain.SeverityP2,
		Message:     "High latency detected",
		Timestamp:   time.Date(2026, 5, 3, 10, 15, 0, 0, time.UTC),
	}
}

package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"ims/internal/domain"
	"ims/internal/queues"
	"ims/internal/requestctx"
)

const queuePublishTimeout = 2 * time.Second

type IngestionService struct {
	queue  queues.SignalQueue
	logger *slog.Logger
}

func NewIngestionService(queue queues.SignalQueue) *IngestionService {
	return NewIngestionServiceWithLogger(queue, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

func NewIngestionServiceWithLogger(queue queues.SignalQueue, logger *slog.Logger) *IngestionService {
	return &IngestionService{
		queue:  queue,
		logger: logger,
	}
}

func (s *IngestionService) Ingest(ctx context.Context, signal domain.Signal) error {
	publishCtx, cancel := context.WithTimeout(ctx, queuePublishTimeout)
	defer cancel()

	if err := s.queue.Enqueue(publishCtx, signal); err != nil {
		s.logger.Error(
			"signal_enqueue_failed",
			slog.String("request_id", requestctx.RequestID(ctx)),
			slog.String("component_id", signal.ComponentID),
			slog.String("severity", string(signal.Severity)),
			slog.String("error", err.Error()),
		)

		return fmt.Errorf("enqueue signal: %w", err)
	}

	return nil
}

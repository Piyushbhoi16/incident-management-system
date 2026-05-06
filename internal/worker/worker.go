package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"ims/internal/domain"
	"ims/internal/queues"
	"ims/internal/repositories"
	"ims/internal/services"
)

const (
	defaultWorkerCount = 5
	maxProcessAttempts = 3
	maxWorkItemCreateAttempts = 3
	processTimeout     = 2 * time.Second
	retryDelay         = 200 * time.Millisecond
	throughputWindow   = 5 * time.Second
)

type SignalWorker struct {
	queue          queues.SignalConsumerQueue
	rawSignals     repositories.RawSignalRepository
	debounce       *services.DebounceService
	workItems      WorkItemCreator
	workerCount    int
	logger         *slog.Logger
	processedCount atomic.Int64
}

// WorkItemCreator is satisfied by WorkItemService; nil means NEW_WORK_ITEM cannot be persisted (tests / misconfig).
type WorkItemCreator interface {
	CreateOpen(ctx context.Context, id, componentID string, severity domain.Severity) error
}

func NewSignalWorker(
	queue queues.SignalConsumerQueue,
	rawSignals repositories.RawSignalRepository,
	debounce *services.DebounceService,
	workItems WorkItemCreator,
	workerCount int,
) *SignalWorker {
	return NewSignalWorkerWithLogger(queue, rawSignals, debounce, workItems, workerCount, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

func NewSignalWorkerWithLogger(
	queue queues.SignalConsumerQueue,
	rawSignals repositories.RawSignalRepository,
	debounce *services.DebounceService,
	workItems WorkItemCreator,
	workerCount int,
	logger *slog.Logger,
) *SignalWorker {
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
	}

	return &SignalWorker{
		queue:       queue,
		rawSignals:  rawSignals,
		debounce:    debounce,
		workItems:   workItems,
		workerCount: workerCount,
		logger:      logger,
	}
}

func (w *SignalWorker) Start(ctx context.Context) {
	w.logger.Info("signal_worker_starting", slog.Int("worker_count", w.workerCount))

	var wg sync.WaitGroup

	// Concurrency design:
	// Start a fixed-size worker pool. Each goroutine blocks on the queue and
	// processes one message at a time, which prevents unbounded goroutine growth
	// during bursts while still allowing parallel processing.
	for id := 1; id <= w.workerCount; id++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.consume(ctx, workerID)
		}(id)
	}

	go w.logThroughput(ctx)

	go func() {
		wg.Wait()
		w.logger.Info("signal_worker_stopped")
	}()
}

func (w *SignalWorker) consume(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		payload, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			w.logger.Error(
				"signal_dequeue_failed",
				slog.Int("worker_id", workerID),
				slog.String("error", err.Error()),
			)
			time.Sleep(retryDelay)
			continue
		}

		if err := w.processWithRetry(ctx, workerID, payload); err != nil {
			metadata := signalMetadataFromPayload(payload)

			w.logger.Error(
				"signal_processing_failed",
				slog.Int("worker_id", workerID),
				slog.String("request_id", metadata.RequestID),
				slog.String("component_id", metadata.ComponentID),
				slog.String("error", err.Error()),
			)

			if dlqErr := w.queue.EnqueueRaw(ctx, payload); dlqErr != nil {
				w.logger.Error(
					"signal_dlq_push_failed",
					slog.Int("worker_id", workerID),
					slog.String("request_id", metadata.RequestID),
					slog.String("component_id", metadata.ComponentID),
					slog.String("error", dlqErr.Error()),
				)
				continue
			}

			w.logger.Error(
				"signal_sent_to_dlq",
				slog.Int("worker_id", workerID),
				slog.String("request_id", metadata.RequestID),
				slog.String("component_id", metadata.ComponentID),
			)
		}
	}
}

func (w *SignalWorker) processWithRetry(ctx context.Context, workerID int, payload []byte) error {
	var lastErr error

	// Retry logic:
	// Processing can fail because of malformed payloads now, and later because
	// Mongo/Postgres/debounce dependencies are temporarily unavailable. We retry
	// each dequeued message a small, bounded number of times with a short delay.
	for attempt := 1; attempt <= maxProcessAttempts; attempt++ {
		processCtx, cancel := context.WithTimeout(ctx, processTimeout)
		err := w.process(processCtx, payload, workerID, attempt)
		cancel()

		if err != nil {
			lastErr = err
			w.logger.Error(
				"signal_processing_attempt_failed",
				slog.Int("worker_id", workerID),
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
			)

			if attempt < maxProcessAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(retryDelay):
				}
			}
			continue
		}

		w.processedCount.Add(1)
		return nil
	}

	return fmt.Errorf("signal processing failed after %d attempts: %w", maxProcessAttempts, lastErr)
}

func (w *SignalWorker) process(ctx context.Context, payload []byte, workerID, attempt int) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("process signal: %w", err)
	}

	var signal domain.Signal
	if err := json.Unmarshal(payload, &signal); err != nil {
		return fmt.Errorf("parse signal JSON: %w", err)
	}

	w.logger.Info(
		"signal_processing_started",
		slog.Int("worker_id", workerID),
		slog.Int("attempt", attempt),
		slog.String("request_id", signal.RequestID),
		slog.String("component_id", signal.ComponentID),
		slog.String("severity", string(signal.Severity)),
	)

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("process signal: %w", err)
	}

	// Debounce logic:
	// The first signal for a component writes a generated work_item_id to
	// debounce:{component_id} with a 10s TTL. Any additional signal during that
	// TTL reuses the same work_item_id, so all raw signals are linked while only
	// one future incident is triggered for the burst.
	debounceResult, err := w.debounce.GetOrCreateWorkItemID(ctx, signal.ComponentID)
	if err != nil {
		return fmt.Errorf("debounce signal: %w", err)
	}

	// Audit path unchanged: every signal is stored in Mongo with the debounced work_item_id.
	if err := w.rawSignals.Store(ctx, signal, &debounceResult.WorkItemID); err != nil {
		return fmt.Errorf("store raw signal: %w", err)
	}

	// PostgreSQL row is created only when this goroutine won the debounce window (NEW_WORK_ITEM).
	// Debounced signals reuse the same id in Mongo but do not insert another work_items row.
	if debounceResult.Created {
		if w.workItems == nil {
			return fmt.Errorf("work item service is not configured")
		}
		if err := w.createWorkItemWithRetry(ctx, workerID, signal, debounceResult.WorkItemID); err != nil {
			return fmt.Errorf("create work item: %w", err)
		}
	}

	w.logger.Info(
		"signal_stored",
		slog.Int("worker_id", workerID),
		slog.String("request_id", signal.RequestID),
		slog.String("component_id", signal.ComponentID),
		slog.String("severity", string(signal.Severity)),
		slog.String("work_item_id", debounceResult.WorkItemID),
	)

	if debounceResult.Created {
		w.logger.Info(
			"NEW_WORK_ITEM",
			slog.Int("worker_id", workerID),
			slog.String("request_id", signal.RequestID),
			slog.String("component_id", signal.ComponentID),
			slog.String("severity", string(signal.Severity)),
			slog.String("work_item_id", debounceResult.WorkItemID),
		)
	} else {
		w.logger.Info(
			"DEBOUNCED_SIGNAL",
			slog.Int("worker_id", workerID),
			slog.String("request_id", signal.RequestID),
			slog.String("component_id", signal.ComponentID),
			slog.String("severity", string(signal.Severity)),
			slog.String("work_item_id", debounceResult.WorkItemID),
		)
	}

	w.logger.Info(
		"signal_processing_completed",
		slog.Int("worker_id", workerID),
		slog.String("request_id", signal.RequestID),
		slog.String("component_id", signal.ComponentID),
		slog.String("severity", string(signal.Severity)),
		slog.String("work_item_id", debounceResult.WorkItemID),
	)

	return nil
}

func (w *SignalWorker) createWorkItemWithRetry(ctx context.Context, workerID int, signal domain.Signal, workItemID string) error {
	var lastErr error

	// Retry NEW_WORK_ITEM persistence independently so brief PostgreSQL hiccups
	// do not immediately fail the full signal processing attempt.
	for attempt := 1; attempt <= maxWorkItemCreateAttempts; attempt++ {
		if err := w.workItems.CreateOpen(ctx, workItemID, signal.ComponentID, signal.Severity); err != nil {
			lastErr = err
			w.logger.Error(
				"work_item_create_attempt_failed",
				slog.Int("worker_id", workerID),
				slog.Int("attempt", attempt),
				slog.String("request_id", signal.RequestID),
				slog.String("component_id", signal.ComponentID),
				slog.String("work_item_id", workItemID),
				slog.String("error", err.Error()),
			)

			if attempt < maxWorkItemCreateAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(retryDelay):
				}
			}
			continue
		}

		return nil
	}

	w.logger.Error(
		"work_item_create_failed",
		slog.Int("worker_id", workerID),
		slog.String("request_id", signal.RequestID),
		slog.String("component_id", signal.ComponentID),
		slog.String("work_item_id", workItemID),
		slog.Int("max_attempts", maxWorkItemCreateAttempts),
		slog.String("error", lastErr.Error()),
	)

	return fmt.Errorf("work item create failed after %d attempts: %w", maxWorkItemCreateAttempts, lastErr)
}

func (w *SignalWorker) logThroughput(ctx context.Context) {
	ticker := time.NewTicker(throughputWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed := w.processedCount.Swap(0)
			perSecond := float64(processed) / throughputWindow.Seconds()

			w.logger.Info(
				"signals processed per second",
				slog.Float64("signals_per_second", perSecond),
				slog.Int64("signals_processed", processed),
				slog.String("window", throughputWindow.String()),
			)
		}
	}
}

type signalMetadata struct {
	RequestID   string `json:"request_id"`
	ComponentID string `json:"component_id"`
}

func signalMetadataFromPayload(payload []byte) signalMetadata {
	var metadata signalMetadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return signalMetadata{}
	}

	return metadata
}

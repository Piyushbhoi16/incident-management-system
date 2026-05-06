package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"ims/internal/domain"
	"ims/internal/repositories"
	"ims/internal/services"
)

type fakeConsumerQueue struct {
	payloads    [][]byte
	dlqPayloads [][]byte
	dequeueErr  error
	cancel      context.CancelFunc
}

func (q *fakeConsumerQueue) Dequeue(_ context.Context) ([]byte, error) {
	if q.dequeueErr != nil {
		return nil, q.dequeueErr
	}
	if len(q.payloads) == 0 {
		return nil, errors.New("empty queue")
	}

	payload := q.payloads[0]
	q.payloads = q.payloads[1:]
	return payload, nil
}

func (q *fakeConsumerQueue) EnqueueRaw(_ context.Context, payload []byte) error {
	q.dlqPayloads = append(q.dlqPayloads, payload)
	if q.cancel != nil {
		q.cancel()
	}
	return nil
}

type fakeRawSignalRepository struct {
	signals     []domain.Signal
	workItemIDs []*string
	err         error
}

func (r *fakeRawSignalRepository) Store(_ context.Context, signal domain.Signal, workItemID *string) error {
	if r.err != nil {
		return r.err
	}

	r.signals = append(r.signals, signal)
	r.workItemIDs = append(r.workItemIDs, workItemID)
	return nil
}

func (r *fakeRawSignalRepository) EnsureIndexes(_ context.Context) error {
	return nil
}

type fakeDebounceRepository struct {
	acquired       bool
	existingID     string
	err            error
	keys           []string
	workItemIDArgs []string
	ttls           []time.Duration
}

func (r *fakeDebounceRepository) GetOrCreateWorkItemID(_ context.Context, componentID string, candidate string, ttl time.Duration) (string, bool, error) {
	if r.err != nil {
		return "", false, r.err
	}

	r.keys = append(r.keys, componentID)
	r.workItemIDArgs = append(r.workItemIDArgs, candidate)
	r.ttls = append(r.ttls, ttl)
	if r.acquired {
		return candidate, true, nil
	}
	if r.existingID != "" {
		return r.existingID, false, nil
	}
	return "existing-work-item-id", false, nil
}

type fakeWorkItemCreator struct {
	createdItems []domain.WorkItem
	err          error
	calls        int
}

func (s *fakeWorkItemCreator) CreateOpen(_ context.Context, id, componentID string, severity domain.Severity) error {
	s.calls++
	if s.err != nil {
		return s.err
	}

	s.createdItems = append(s.createdItems, domain.WorkItem{
		ID:          id,
		ComponentID: componentID,
		Severity:    severity,
		Status:      domain.StatusOpen,
	})
	return nil
}

func TestProcessWithRetryProcessesValidSignal(t *testing.T) {
	var logs bytes.Buffer
	rawSignals := &fakeRawSignalRepository{}
	debounceRepo := &fakeDebounceRepository{acquired: true}
	workItems := &fakeWorkItemCreator{}
	worker := newTestWorker(&fakeConsumerQueue{}, rawSignals, debounceRepo, workItems, &logs)

	payload := []byte(`{
		"component_id": "CACHE_CLUSTER_01",
		"severity": "P2",
		"message": "High latency detected",
		"timestamp": "2026-05-03T10:15:00Z"
	}`)

	if err := worker.processWithRetry(context.Background(), 1, payload); err != nil {
		t.Fatalf("expected valid signal to process, got %v", err)
	}

	if processed := worker.processedCount.Load(); processed != 1 {
		t.Fatalf("expected processed count 1, got %d", processed)
	}
	if len(rawSignals.signals) != 1 {
		t.Fatalf("expected 1 stored raw signal, got %d", len(rawSignals.signals))
	}
	if len(rawSignals.workItemIDs) != 1 || rawSignals.workItemIDs[0] == nil || *rawSignals.workItemIDs[0] == "" {
		t.Fatalf("expected stored raw signal to include generated work item id, got %#v", rawSignals.workItemIDs)
	}
	if len(debounceRepo.keys) != 1 || debounceRepo.keys[0] != "CACHE_CLUSTER_01" {
		t.Fatalf("expected debounce for CACHE_CLUSTER_01, got %#v", debounceRepo.keys)
	}
	if len(workItems.createdItems) != 1 {
		t.Fatalf("expected one created work item, got %d", len(workItems.createdItems))
	}
	if workItems.createdItems[0].ID != *rawSignals.workItemIDs[0] {
		t.Fatalf("expected work item id %s, got %s", *rawSignals.workItemIDs[0], workItems.createdItems[0].ID)
	}

	output := logs.String()
	for _, expected := range []string{"signal_stored", "NEW_WORK_ITEM", *rawSignals.workItemIDs[0]} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log to contain %q, got %s", expected, output)
		}
	}
}

func TestProcessWithRetryReturnsErrorForInvalidSignalJSON(t *testing.T) {
	var logs bytes.Buffer
	worker := newTestWorker(&fakeConsumerQueue{}, &fakeRawSignalRepository{}, &fakeDebounceRepository{}, nil, &logs)

	err := worker.processWithRetry(context.Background(), 1, []byte(`not-json`))

	if err == nil {
		t.Fatal("expected invalid JSON to fail after retries")
	}
	if processed := worker.processedCount.Load(); processed != 0 {
		t.Fatalf("expected processed count 0, got %d", processed)
	}
}

func TestProcessReturnsErrorWhenContextExpired(t *testing.T) {
	var logs bytes.Buffer
	worker := newTestWorker(&fakeConsumerQueue{}, &fakeRawSignalRepository{}, &fakeDebounceRepository{}, nil, &logs)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := worker.process(ctx, []byte(`{
		"component_id": "CACHE_CLUSTER_01",
		"severity": "P2",
		"message": "High latency detected",
		"timestamp": "2026-05-03T10:15:00Z"
	}`), 1, 1)

	if err == nil {
		t.Fatal("expected expired context to fail processing")
	}
}

func TestConsumePushesFailedMessageToDLQAfterMaxRetries(t *testing.T) {
	var logs bytes.Buffer
	payload := []byte(`{
		"request_id": "req-123",
		"component_id": "CACHE_CLUSTER_01",
		"severity": "P2",
		"message": "High latency detected",
		"timestamp": "not-a-timestamp"
	}`)
	queue := &fakeConsumerQueue{
		payloads: [][]byte{payload},
	}
	worker := newTestWorker(queue, &fakeRawSignalRepository{}, &fakeDebounceRepository{}, nil, &logs)

	ctx, cancel := context.WithCancel(context.Background())
	queue.cancel = cancel

	worker.consume(ctx, 1)

	if len(queue.dlqPayloads) != 1 {
		t.Fatalf("expected 1 DLQ payload, got %d", len(queue.dlqPayloads))
	}
	if string(queue.dlqPayloads[0]) != string(payload) {
		t.Fatalf("expected original payload in DLQ, got %q", string(queue.dlqPayloads[0]))
	}

	output := logs.String()
	for _, expected := range []string{"signal_sent_to_dlq", "req-123", "CACHE_CLUSTER_01"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected DLQ log to contain %q, got %s", expected, output)
		}
	}
}

func TestDefaultWorkerCount(t *testing.T) {
	worker := NewSignalWorkerWithLogger(
		&fakeConsumerQueue{},
		&fakeRawSignalRepository{},
		services.NewDebounceService(&fakeDebounceRepository{}),
		nil,
		0,
		slog.Default(),
	)

	if worker.workerCount != defaultWorkerCount {
		t.Fatalf("expected default worker count %d, got %d", defaultWorkerCount, worker.workerCount)
	}
}

func TestProcessLogsDebouncedSignal(t *testing.T) {
	var logs bytes.Buffer
	rawSignals := &fakeRawSignalRepository{}
	debounceRepo := &fakeDebounceRepository{acquired: false, existingID: "work-item-123"}
	worker := newTestWorker(&fakeConsumerQueue{}, rawSignals, debounceRepo, nil, &logs)

	payload := []byte(`{
		"request_id": "req-123",
		"component_id": "CACHE_CLUSTER_01",
		"severity": "P2",
		"message": "High latency detected",
		"timestamp": "2026-05-03T10:15:00Z"
	}`)

	if err := worker.process(context.Background(), payload, 1, 1); err != nil {
		t.Fatalf("expected debounced signal to process, got %v", err)
	}

	if len(rawSignals.signals) != 1 {
		t.Fatalf("expected raw signal to be stored, got %d", len(rawSignals.signals))
	}
	if len(rawSignals.workItemIDs) != 1 || rawSignals.workItemIDs[0] == nil || *rawSignals.workItemIDs[0] != "work-item-123" {
		t.Fatalf("expected existing work item id in raw signal, got %#v", rawSignals.workItemIDs)
	}
	if !strings.Contains(logs.String(), "DEBOUNCED_SIGNAL") || !strings.Contains(logs.String(), "work-item-123") {
		t.Fatalf("expected debounced log, got %s", logs.String())
	}
}

func TestProcessRetriesWorkItemCreateWhenNewWorkItem(t *testing.T) {
	var logs bytes.Buffer
	rawSignals := &fakeRawSignalRepository{}
	debounceRepo := &fakeDebounceRepository{acquired: true}
	workItems := &fakeWorkItemCreator{err: errors.New("postgres unavailable")}
	worker := newTestWorker(&fakeConsumerQueue{}, rawSignals, debounceRepo, workItems, &logs)

	payload := []byte(`{
		"request_id": "req-123",
		"component_id": "CACHE_CLUSTER_01",
		"severity": "P2",
		"message": "High latency detected",
		"timestamp": "2026-05-03T10:15:00Z"
	}`)

	err := worker.process(context.Background(), payload, 1, 1)
	if err == nil {
		t.Fatal("expected work item create failure")
	}

	if workItems.calls != maxWorkItemCreateAttempts {
		t.Fatalf("expected %d work item create attempts, got %d", maxWorkItemCreateAttempts, workItems.calls)
	}

	output := logs.String()
	for _, expected := range []string{"work_item_create_attempt_failed", "work_item_create_failed", "req-123"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log to contain %q, got %s", expected, output)
		}
	}
}

func newTestWorker(
	queue *fakeConsumerQueue,
	rawSignals repositories.RawSignalRepository,
	debounceRepo repositories.DebounceRepository,
	workItems WorkItemCreator,
	logs *bytes.Buffer,
) *SignalWorker {
	return NewSignalWorkerWithLogger(
		queue,
		rawSignals,
		services.NewDebounceService(debounceRepo),
		workItems,
		1,
		slog.New(slog.NewJSONHandler(logs, nil)),
	)
}

package queues

import (
	"context"

	"ims/internal/domain"
)

type SignalQueue interface {
	Enqueue(ctx context.Context, signal domain.Signal) error
}

type SignalConsumerQueue interface {
	Dequeue(ctx context.Context) ([]byte, error)
	EnqueueRaw(ctx context.Context, payload []byte) error
}

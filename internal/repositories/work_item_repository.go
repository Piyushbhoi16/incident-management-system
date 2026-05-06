package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/domain"
)

var ErrWorkItemNotFound = errors.New("work item not found")

// WorkItemRepository persists incident work items. Implementations are storage-specific
// (PostgreSQL); the worker uses this only for NEW_WORK_ITEM (debounce leader) paths.
type WorkItemRepository interface {
	Create(ctx context.Context, item domain.WorkItem) error
	GetByID(ctx context.Context, id string) (domain.WorkItem, error)
	ListActive(ctx context.Context) ([]domain.WorkItem, error)
	UpdateStatus(ctx context.Context, id string, status domain.WorkItemStatus) (domain.WorkItem, error)
	// UpdateMTTR stores MTTR in nanoseconds (end_time - start_time from RCA).
	UpdateMTTR(ctx context.Context, id string, mttrNs int64) error
}

// PostgresWorkItemRepository maps domain.WorkItem to the work_items table.
type PostgresWorkItemRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresWorkItemRepository(pool *pgxpool.Pool) *PostgresWorkItemRepository {
	return &PostgresWorkItemRepository{pool: pool}
}

// EnsureSchema creates the work_items table for local/dev bootstrap. Production may
// replace this with migrations; it stays on the concrete type so the interface stays minimal.
func (r *PostgresWorkItemRepository) EnsureSchema(ctx context.Context) error {
	query := `
CREATE TABLE IF NOT EXISTS work_items (
	id UUID PRIMARY KEY,
	component_id TEXT NOT NULL,
	severity TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	if _, err := r.pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("ensure work_items table: %w", err)
	}
	// Idempotent column add for existing deployments.
	alter := `ALTER TABLE work_items ADD COLUMN IF NOT EXISTS mttr_ns BIGINT;`
	if _, err := r.pool.Exec(ctx, alter); err != nil {
		return fmt.Errorf("ensure work_items.mttr_ns: %w", err)
	}
	return nil
}

// Create inserts a work item. ON CONFLICT DO NOTHING makes retries safe if the same debounce id is processed twice.
func (r *PostgresWorkItemRepository) Create(ctx context.Context, item domain.WorkItem) error {
	query := `
INSERT INTO work_items (id, component_id, severity, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;`
	if _, err := r.pool.Exec(ctx, query, item.ID, item.ComponentID, item.Severity, item.Status); err != nil {
		return fmt.Errorf("create work item: %w", err)
	}
	return nil
}

func scanWorkItem(row interface {
	Scan(dest ...any) error
}) (domain.WorkItem, error) {
	var item domain.WorkItem
	var mttrNs *int64
	if err := row.Scan(
		&item.ID,
		&item.ComponentID,
		&item.Severity,
		&item.Status,
		&mttrNs,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return domain.WorkItem{}, err
	}
	if mttrNs != nil {
		d := time.Duration(*mttrNs)
		item.MTTR = &d
	}
	return item, nil
}

func (r *PostgresWorkItemRepository) GetByID(ctx context.Context, id string) (domain.WorkItem, error) {
	query := `
SELECT id::text, component_id, severity, status, mttr_ns, created_at, updated_at
FROM work_items
WHERE id = $1;`

	item, err := scanWorkItem(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WorkItem{}, ErrWorkItemNotFound
		}
		return domain.WorkItem{}, fmt.Errorf("get work item: %w", err)
	}

	return item, nil
}

func (r *PostgresWorkItemRepository) ListActive(ctx context.Context) ([]domain.WorkItem, error) {
	query := `
SELECT id::text, component_id, severity, status, mttr_ns, created_at, updated_at
FROM work_items
WHERE status <> $1
ORDER BY
	CASE severity
		WHEN 'P0' THEN 0
		WHEN 'P1' THEN 1
		WHEN 'P2' THEN 2
		ELSE 3
	END,
	created_at ASC;`

	rows, err := r.pool.Query(ctx, query, domain.StatusClosed)
	if err != nil {
		return nil, fmt.Errorf("list active work items: %w", err)
	}
	defer rows.Close()

	var items []domain.WorkItem
	for rows.Next() {
		item, err := scanWorkItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active work item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active work items: %w", err)
	}

	return items, nil
}

func (r *PostgresWorkItemRepository) UpdateStatus(ctx context.Context, id string, status domain.WorkItemStatus) (domain.WorkItem, error) {
	query := `
UPDATE work_items
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id::text, component_id, severity, status, mttr_ns, created_at, updated_at;`

	item, err := scanWorkItem(r.pool.QueryRow(ctx, query, id, status))
	if err != nil {
		return domain.WorkItem{}, fmt.Errorf("update work item status: %w", err)
	}

	return item, nil
}

// UpdateMTTR stores MTTR as nanoseconds in mttr_ns (derived from RCA end - start).
func (r *PostgresWorkItemRepository) UpdateMTTR(ctx context.Context, id string, mttrNs int64) error {
	query := `UPDATE work_items SET mttr_ns = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, id, mttrNs); err != nil {
		return fmt.Errorf("update work item mttr: %w", err)
	}
	return nil
}

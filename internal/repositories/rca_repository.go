package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/domain"
)

// RCAReader is the subset of persistence needed to enforce "no CLOSED without RCA".
type RCAReader interface {
	HasRCAForWorkItem(ctx context.Context, workItemID string) (bool, error)
}

// RCARepository persists root-cause analysis records.
type RCARepository interface {
	RCAReader
	EnsureSchema(ctx context.Context) error
	Create(ctx context.Context, rca domain.RCA) error
}

type PostgresRCARepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRCARepository(pool *pgxpool.Pool) *PostgresRCARepository {
	return &PostgresRCARepository{pool: pool}
}

// EnsureSchema creates the rca table. Must run after work_items exists (FK).
func (r *PostgresRCARepository) EnsureSchema(ctx context.Context) error {
	query := `
CREATE TABLE IF NOT EXISTS rca (
	work_item_id UUID PRIMARY KEY REFERENCES work_items(id) ON DELETE CASCADE,
	start_time TIMESTAMPTZ NOT NULL,
	end_time TIMESTAMPTZ NOT NULL,
	root_cause TEXT NOT NULL,
	fix TEXT NOT NULL,
	prevention TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	if _, err := r.pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("ensure rca table: %w", err)
	}
	return nil
}

func (r *PostgresRCARepository) Create(ctx context.Context, rca domain.RCA) error {
	query := `
INSERT INTO rca (work_item_id, start_time, end_time, root_cause, fix, prevention, created_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW());`
	_, err := r.pool.Exec(ctx, query,
		rca.WorkItemID,
		rca.StartTime,
		rca.EndTime,
		rca.RootCause,
		rca.Fix,
		rca.Prevention,
	)
	if err != nil {
		return fmt.Errorf("insert rca: %w", err)
	}
	return nil
}

func (r *PostgresRCARepository) HasRCAForWorkItem(ctx context.Context, workItemID string) (bool, error) {
	var one int
	err := r.pool.QueryRow(ctx, `SELECT 1 FROM rca WHERE work_item_id = $1 LIMIT 1`, workItemID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check rca: %w", err)
	}
	return true, nil
}

package repositories

import (
	"time"

	"ims/internal/config"
	"ims/internal/domain"
)

type HealthRepository interface {
	Get() domain.HealthStatus
}

type InMemoryHealthRepository struct {
	cfg config.Config
}

func NewInMemoryHealthRepository(cfg config.Config) *InMemoryHealthRepository {
	return &InMemoryHealthRepository{cfg: cfg}
}

func (r *InMemoryHealthRepository) Get() domain.HealthStatus {
	return domain.HealthStatus{
		Status:  "ok",
		Service: r.cfg.AppName,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
}

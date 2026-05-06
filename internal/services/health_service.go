package services

import (
	"ims/internal/domain"
	"ims/internal/repositories"
)

type HealthService struct {
	repo repositories.HealthRepository
}

func NewHealthService(repo repositories.HealthRepository) *HealthService {
	return &HealthService{repo: repo}
}

func (s *HealthService) GetHealth() domain.HealthStatus {
	return s.repo.Get()
}

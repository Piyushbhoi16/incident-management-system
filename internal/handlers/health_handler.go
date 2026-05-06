package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ims/internal/services"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Time    string `json:"time"`
}

type HealthHandler struct {
	service *services.HealthService
}

func NewHealthHandler(service *services.HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

func (h *HealthHandler) GetHealth(c *gin.Context) {
	health := h.service.GetHealth()

	c.JSON(http.StatusOK, HealthResponse{
		Status:  health.Status,
		Service: health.Service,
		Time:    health.Time,
	})
}

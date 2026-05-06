package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ims/internal/domain"
	"ims/internal/requestctx"
	"ims/internal/services"
)

type IngestSignalRequest struct {
	ComponentID string `json:"component_id"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Timestamp   string `json:"timestamp"`
}

type IngestSignalResponse struct {
	Status string `json:"status"`
}

type IngestionHandler struct {
	service *services.IngestionService
}

func NewIngestionHandler(service *services.IngestionService) *IngestionHandler {
	return &IngestionHandler{service: service}
}

func (h *IngestionHandler) Ingest(c *gin.Context) {
	var req IngestSignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON request body"})
		return
	}

	signal, ok := validateIngestSignalRequest(req)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "component_id, severity, message, and valid RFC3339 timestamp are required"})
		return
	}
	signal.RequestID = requestctx.RequestID(c.Request.Context())

	if err := h.service.Ingest(c.Request.Context(), signal); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "signal queue unavailable"})
		return
	}

	c.JSON(http.StatusAccepted, IngestSignalResponse{Status: "accepted"})
}

func validateIngestSignalRequest(req IngestSignalRequest) (domain.Signal, bool) {
	componentID := strings.TrimSpace(req.ComponentID)
	message := strings.TrimSpace(req.Message)
	severity := domain.Severity(strings.TrimSpace(req.Severity))

	timestamp, err := time.Parse(time.RFC3339, strings.TrimSpace(req.Timestamp))
	if err != nil {
		return domain.Signal{}, false
	}

	if componentID == "" || message == "" || !domain.IsValidSeverity(severity) {
		return domain.Signal{}, false
	}

	return domain.Signal{
		ComponentID: componentID,
		Severity:    severity,
		Message:     message,
		Timestamp:   timestamp.UTC(),
	}, true
}

package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ims/internal/domain"
	"ims/internal/services"
)

type CreateRCARequest struct {
	WorkItemID string `json:"work_item_id"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	RootCause  string `json:"root_cause"`
	Fix        string `json:"fix"`
	Prevention string `json:"prevention"`
}

type RCAHandler struct {
	service *services.RCAService
}

func NewRCAHandler(service *services.RCAService) *RCAHandler {
	return &RCAHandler{service: service}
}

func (h *RCAHandler) Create(c *gin.Context) {
	var req CreateRCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON request body"})
		return
	}
	startTime, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartTime))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_time must be RFC3339"})
		return
	}
	endTime, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndTime))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be RFC3339"})
		return
	}

	rca := domain.RCA{
		WorkItemID: strings.TrimSpace(req.WorkItemID),
		StartTime:  startTime,
		EndTime:    endTime,
		RootCause:  req.RootCause,
		Fix:        req.Fix,
		Prevention: req.Prevention,
	}

	if err := h.service.Create(c.Request.Context(), rca); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "validate RCA:") {
			c.JSON(http.StatusBadRequest, gin.H{"error": strings.TrimPrefix(msg, "validate RCA: ")})
			return
		}
		if strings.Contains(msg, "save RCA:") {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		if strings.Contains(msg, "RCA already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

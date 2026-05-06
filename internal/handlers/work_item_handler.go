package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ims/internal/domain"
	"ims/internal/repositories"
	"ims/internal/services"
)

type UpdateWorkItemStatusRequest struct {
	Status string `json:"status"`
}

type WorkItemResponse struct {
	ID          string `json:"id"`
	ComponentID string `json:"component_id"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ActiveWorkItemResponse struct {
	WorkItemID  string `json:"work_item_id"`
	ComponentID string `json:"component_id"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type ActiveWorkItemsResponse struct {
	Status string                   `json:"status"`
	Data   []ActiveWorkItemResponse `json:"data"`
}

type WorkItemDetailResponse struct {
	Status string           `json:"status"`
	Data   WorkItemResponse `json:"data"`
}

type WorkItemHandler struct {
	service *services.WorkItemService
}

func NewWorkItemHandler(service *services.WorkItemService) *WorkItemHandler {
	return &WorkItemHandler{service: service}
}

func (h *WorkItemHandler) ListActive(c *gin.Context) {
	items, err := h.service.ListActive(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list active work items"})
		return
	}

	response := make([]ActiveWorkItemResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toActiveWorkItemResponse(item))
	}

	c.JSON(http.StatusOK, ActiveWorkItemsResponse{
		Status: "success",
		Data:   response,
	})
}

func (h *WorkItemHandler) GetByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "work item id is required"})
		return
	}

	item, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repositories.ErrWorkItemNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "work item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get work item"})
		return
	}

	c.JSON(http.StatusOK, WorkItemDetailResponse{
		Status: "success",
		Data:   toWorkItemResponse(item),
	})
}

func (h *WorkItemHandler) UpdateStatus(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "work item id is required"})
		return
	}

	var req UpdateWorkItemStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON request body"})
		return
	}

	next := domain.WorkItemStatus(strings.TrimSpace(req.Status))
	item, err := h.service.Transition(c.Request.Context(), id, next)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, WorkItemDetailResponse{
		Status: "success",
		Data:   toWorkItemResponse(item),
	})
}

func toWorkItemResponse(item domain.WorkItem) WorkItemResponse {
	return WorkItemResponse{
		ID:          item.ID,
		ComponentID: item.ComponentID,
		Severity:    string(item.Severity),
		Status:      string(item.Status),
		CreatedAt:   item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toActiveWorkItemResponse(item domain.WorkItem) ActiveWorkItemResponse {
	return ActiveWorkItemResponse{
		WorkItemID:  item.ID,
		ComponentID: item.ComponentID,
		Severity:    string(item.Severity),
		Status:      string(item.Status),
		CreatedAt:   item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

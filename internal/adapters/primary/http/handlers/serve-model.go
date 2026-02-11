package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/dto"
	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

func (h *Handler) ListServeModels(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := output.ServeModelFilter{
		ProjectID: projectID,
		State:     c.Query("state"),
		SortBy:    c.Query("sort_by"),
		Order:     c.Query("order"),
		Limit:     limit,
		Offset:    offset,
	}

	// Optional filters
	if isvcID := c.Query("inference_service_id"); isvcID != "" {
		if id, err := uuid.Parse(isvcID); err == nil {
			filter.InferenceServiceID = &id
		}
	}
	if versionID := c.Query("model_version_id"); versionID != "" {
		if id, err := uuid.Parse(versionID); err == nil {
			filter.ModelVersionID = &id
		}
	}

	serveModels, total, err := h.serveModelSvc.List(c.Request.Context(), filter)
	if err != nil {
		log.WithError(err).Error("list serve models failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.ServeModelResponse, 0, len(serveModels))
	for _, sm := range serveModels {
		items = append(items, dto.ToServeModelResponse(sm))
	}

	c.JSON(http.StatusOK, dto.ListServeModelsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetServeModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	sm, err := h.serveModelSvc.Get(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToServeModelResponse(sm))
}

func (h *Handler) CreateServeModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateServeModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sm, err := h.serveModelSvc.Create(
		c.Request.Context(),
		projectID,
		req.InferenceServiceID,
		req.ModelVersionID,
	)
	if err != nil {
		log.WithError(err).Error("create serve model failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToServeModelResponse(sm))
}

func (h *Handler) DeleteServeModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.serveModelSvc.Delete(c.Request.Context(), projectID, id); err != nil {
		log.WithError(err).Error("delete serve model failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

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

func (h *Handler) ListInferenceServices(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := output.InferenceServiceFilter{
		ProjectID:    projectID,
		DesiredState: c.Query("desired_state"),
		CurrentState: c.Query("current_state"),
		SortBy:       c.Query("sort_by"),
		Order:        c.Query("order"),
		Limit:        limit,
		Offset:       offset,
	}

	// Optional filters
	if envID := c.Query("serving_environment_id"); envID != "" {
		if id, err := uuid.Parse(envID); err == nil {
			filter.ServingEnvironmentID = &id
		}
	}
	if modelID := c.Query("registered_model_id"); modelID != "" {
		if id, err := uuid.Parse(modelID); err == nil {
			filter.RegisteredModelID = &id
		}
	}

	isvcs, total, err := h.isvcSvc.List(c.Request.Context(), filter)
	if err != nil {
		log.WithError(err).Error("list inference services failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.InferenceServiceResponse, 0, len(isvcs))
	for _, isvc := range isvcs {
		items = append(items, dto.ToInferenceServiceResponse(isvc))
	}

	c.JSON(http.StatusOK, dto.ListInferenceServicesResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetInferenceService(c *gin.Context) {
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

	isvc, err := h.isvcSvc.Get(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToInferenceServiceResponse(isvc))
}

func (h *Handler) FindInferenceService(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	externalID := c.Query("externalId")
	if externalID != "" {
		isvc, err := h.isvcSvc.GetByExternalID(c.Request.Context(), projectID, externalID)
		if err != nil {
			mapDomainError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToInferenceServiceResponse(isvc))
		return
	}

	// Find by name requires environment ID
	name := c.Query("name")
	envIDStr := c.Query("serving_environment_id")
	if name != "" && envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid serving_environment_id"})
			return
		}
		isvc, err := h.isvcSvc.GetByName(c.Request.Context(), projectID, envID, name)
		if err != nil {
			mapDomainError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ToInferenceServiceResponse(isvc))
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "either externalId or (name + serving_environment_id) is required"})
}

func (h *Handler) CreateInferenceService(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateInferenceServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isvc, err := h.isvcSvc.Create(
		c.Request.Context(),
		projectID,
		req.Name,
		req.ServingEnvironmentID,
		req.RegisteredModelID,
		req.Runtime,
		req.Labels,
	)
	if err != nil {
		log.WithError(err).Error("create inference service failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToInferenceServiceResponse(isvc))
}

func (h *Handler) UpdateInferenceService(c *gin.Context) {
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

	var req dto.UpdateInferenceServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.ExternalID != nil {
		updates["external_id"] = *req.ExternalID
	}
	if req.DesiredState != nil {
		updates["desired_state"] = *req.DesiredState
	}
	if req.CurrentState != nil {
		updates["current_state"] = *req.CurrentState
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.LastError != nil {
		updates["last_error"] = *req.LastError
	}
	if req.Labels != nil {
		updates["labels"] = req.Labels
	}

	isvc, err := h.isvcSvc.Update(c.Request.Context(), projectID, id, updates)
	if err != nil {
		log.WithError(err).Error("update inference service failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToInferenceServiceResponse(isvc))
}

func (h *Handler) DeleteInferenceService(c *gin.Context) {
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

	if err := h.isvcSvc.Delete(c.Request.Context(), projectID, id); err != nil {
		log.WithError(err).Error("delete inference service failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

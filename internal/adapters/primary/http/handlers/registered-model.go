package handlers

import (
	"net/http"
	"strconv"

	"model-registry-service/internal/adapters/primary/http/dto"
	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) ListModels(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := ports.ListFilter{
		ProjectID: projectID,
		State:     c.Query("state"),
		ModelType: c.Query("model_type"),
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		Order:     c.Query("order"),
		Limit:     limit,
		Offset:    offset,
	}

	models, total, err := h.modelSvc.List(c.Request.Context(), filter)
	if err != nil {
		log.WithError(err).Error("list models failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.RegisteredModelResponse, 0, len(models))
	for _, m := range models {
		items = append(items, dto.ToRegisteredModelResponse(m))
	}

	c.JSON(http.StatusOK, dto.ListRegisteredModelsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	model, err := h.modelSvc.Get(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToRegisteredModelResponse(model))
}

func (h *Handler) GetModelByParams(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	name := c.Query("name")
	externalID := c.Query("externalId")

	model, err := h.modelSvc.GetByParams(c.Request.Context(), projectID, name, externalID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToRegisteredModelResponse(model))
}

func (h *Handler) CreateModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateRegisteredModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tags := domain.Tags{}
	if req.Tags != nil {
		tags = domain.Tags{
			Frameworks:    req.Tags.Frameworks,
			Architectures: req.Tags.Architectures,
			Tasks:         req.Tags.Tasks,
			Subjects:      req.Tags.Subjects,
		}
	}

	model, err := h.modelSvc.Create(
		c.Request.Context(), projectID, nil,
		req.Name, req.Description,
		req.OwnerEmail, req.ModelType, tags, req.Labels, req.ParentModelID,
	)
	if err != nil {
		log.WithError(err).Error("create model failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToRegisteredModelResponse(model))
}

func (h *Handler) UpdateModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	var req dto.UpdateRegisteredModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ModelType != nil {
		updates["model_type"] = *req.ModelType
	}
	if req.ModelSize != nil {
		updates["model_size"] = *req.ModelSize
	}
	if req.State != nil {
		updates["state"] = *req.State
	}
	if req.DeploymentStatus != nil {
		updates["deployment_status"] = *req.DeploymentStatus
	}
	if req.Tags != nil {
		updates["tags"] = domain.Tags{
			Frameworks:    req.Tags.Frameworks,
			Architectures: req.Tags.Architectures,
			Tasks:         req.Tags.Tasks,
			Subjects:      req.Tags.Subjects,
		}
	}
	if req.Labels != nil {
		updates["labels"] = req.Labels
	}

	model, err := h.modelSvc.Update(c.Request.Context(), projectID, id, updates)
	if err != nil {
		log.WithError(err).Error("update model failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToRegisteredModelResponse(model))
}

func (h *Handler) DeleteModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	if err := h.modelSvc.Delete(c.Request.Context(), projectID, id); err != nil {
		log.WithError(err).Error("delete model failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func getProjectID(c *gin.Context) (uuid.UUID, error) {
	header := c.GetHeader("Project-ID")
	if header == "" {
		return uuid.Nil, domain.ErrMissingProjectID
	}
	return uuid.Parse(header)
}

package handler

import (
	"net/http"
	"strconv"

	"model-registry-service/internal/domain"
	"model-registry-service/internal/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) ListModelVersions(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	modelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := domain.VersionListFilter{
		State:  c.Query("state"),
		Status: c.Query("status"),
		SortBy: c.Query("sort_by"),
		Order:  c.Query("order"),
		Limit:  limit,
		Offset: offset,
	}

	versions, total, err := h.versionUC.ListByModel(c.Request.Context(), projectID, modelID, filter)
	if err != nil {
		log.WithError(err).Error("list model versions failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.ModelVersionResponse, 0, len(versions))
	for _, v := range versions {
		items = append(items, dto.ToModelVersionResponse(v))
	}

	c.JSON(http.StatusOK, dto.ListModelVersionsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetModelVersion(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	modelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}
	versionID, err := uuid.Parse(c.Param("ver"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}

	version, err := h.versionUC.GetByModel(c.Request.Context(), projectID, modelID, versionID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelVersionResponse(version))
}

func (h *Handler) CreateModelVersion(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	modelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}

	var req dto.CreateModelVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isDefault := false
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}

	version, err := h.versionUC.Create(
		c.Request.Context(), projectID, modelID,
		req.Name, req.Description, isDefault,
		req.ArtifactType, req.ModelFramework, req.ModelFrameworkVersion,
		req.ContainerImage, req.ModelCatalogName,
		req.URI, req.AccessKey, req.SecretKey,
		req.Labels, req.PrebuiltContainerID, nil,
	)
	if err != nil {
		log.WithError(err).Error("create model version failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToModelVersionResponse(version))
}

func (h *Handler) UpdateModelVersion(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	_, err = uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model id"})
		return
	}
	versionID, err := uuid.Parse(c.Param("ver"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}

	var req dto.UpdateModelVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := buildVersionUpdates(req)

	version, err := h.versionUC.Update(c.Request.Context(), projectID, versionID, updates)
	if err != nil {
		log.WithError(err).Error("update model version failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelVersionResponse(version))
}

// Direct access routes

func (h *Handler) FindModelVersion(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	name := c.Query("name")
	externalID := c.Query("externalId")
	var modelID *uuid.UUID
	if mid := c.Query("parentResourceId"); mid != "" {
		parsed, err := uuid.Parse(mid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parentResourceId"})
			return
		}
		modelID = &parsed
	}

	version, err := h.versionUC.Find(c.Request.Context(), projectID, name, externalID, modelID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelVersionResponse(version))
}

func (h *Handler) ListAllModelVersions(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := domain.VersionListFilter{
		ProjectID: projectID,
		State:     c.Query("state"),
		Status:    c.Query("status"),
		SortBy:    c.Query("sort_by"),
		Order:     c.Query("order"),
		Limit:     limit,
		Offset:    offset,
	}

	versions, total, err := h.versionUC.List(c.Request.Context(), filter)
	if err != nil {
		log.WithError(err).Error("list all model versions failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.ModelVersionResponse, 0, len(versions))
	for _, v := range versions {
		items = append(items, dto.ToModelVersionResponse(v))
	}

	c.JSON(http.StatusOK, dto.ListModelVersionsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetModelVersionDirect(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}

	version, err := h.versionUC.Get(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelVersionResponse(version))
}

func (h *Handler) UpdateModelVersionDirect(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}

	var req dto.UpdateModelVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := buildVersionUpdates(req)

	version, err := h.versionUC.Update(c.Request.Context(), projectID, id, updates)
	if err != nil {
		log.WithError(err).Error("update model version direct failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelVersionResponse(version))
}

func buildVersionUpdates(req dto.UpdateModelVersionRequest) map[string]interface{} {
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}
	if req.State != nil {
		updates["state"] = *req.State
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.ArtifactType != nil {
		updates["artifact_type"] = *req.ArtifactType
	}
	if req.ModelFramework != nil {
		updates["model_framework"] = *req.ModelFramework
	}
	if req.ModelFrameworkVersion != nil {
		updates["model_framework_version"] = *req.ModelFrameworkVersion
	}
	if req.ContainerImage != nil {
		updates["container_image"] = *req.ContainerImage
	}
	if req.URI != nil {
		updates["uri"] = *req.URI
	}
	if req.Labels != nil {
		updates["labels"] = req.Labels
	}
	return updates
}

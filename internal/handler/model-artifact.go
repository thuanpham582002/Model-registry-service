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

func (h *Handler) FindModelArtifact(c *gin.Context) {
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

	version, err := h.artifactUC.Find(c.Request.Context(), projectID, name, externalID, modelID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelArtifactResponse(version))
}

func (h *Handler) ListModelArtifacts(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
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

	versions, total, err := h.artifactUC.List(c.Request.Context(), projectID, filter)
	if err != nil {
		log.WithError(err).Error("list model artifacts failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.ModelArtifactResponse, 0, len(versions))
	for _, v := range versions {
		items = append(items, dto.ToModelArtifactResponse(v))
	}

	c.JSON(http.StatusOK, dto.ListModelArtifactsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetModelArtifact(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid artifact id"})
		return
	}

	version, err := h.artifactUC.Get(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelArtifactResponse(version))
}

func (h *Handler) CreateModelArtifact(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateModelArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	version, err := h.artifactUC.Create(
		c.Request.Context(), projectID, req.RegisteredModelID,
		req.Name, req.Description, req.ArtifactType,
		req.ModelFramework, req.ModelFrameworkVersion,
		req.ContainerImage, req.URI, req.AccessKey, req.SecretKey,
		req.Labels, req.PrebuiltContainerID,
	)
	if err != nil {
		log.WithError(err).Error("create model artifact failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToModelArtifactResponse(version))
}

func (h *Handler) UpdateModelArtifact(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid artifact id"})
		return
	}

	var req dto.UpdateModelArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
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

	version, err := h.artifactUC.Update(c.Request.Context(), projectID, id, updates)
	if err != nil {
		log.WithError(err).Error("update model artifact failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToModelArtifactResponse(version))
}

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

func (h *Handler) ListServingEnvironments(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := output.ServingEnvironmentFilter{
		ProjectID: projectID,
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		Order:     c.Query("order"),
		Limit:     limit,
		Offset:    offset,
	}

	envs, total, err := h.servingEnvSvc.List(c.Request.Context(), filter)
	if err != nil {
		log.WithError(err).Error("list serving environments failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.ServingEnvironmentResponse, 0, len(envs))
	for _, env := range envs {
		items = append(items, dto.ToServingEnvironmentResponse(env))
	}

	c.JSON(http.StatusOK, dto.ListServingEnvironmentsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetServingEnvironment(c *gin.Context) {
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

	env, err := h.servingEnvSvc.Get(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) FindServingEnvironment(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name query parameter is required"})
		return
	}

	env, err := h.servingEnvSvc.GetByName(c.Request.Context(), projectID, name)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) CreateServingEnvironment(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateServingEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := h.servingEnvSvc.Create(c.Request.Context(), projectID, req.Name, req.Description, req.ExternalID)
	if err != nil {
		log.WithError(err).Error("create serving environment failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) UpdateServingEnvironment(c *gin.Context) {
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

	var req dto.UpdateServingEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := h.servingEnvSvc.Update(c.Request.Context(), projectID, id, req.Name, req.Description, req.ExternalID)
	if err != nil {
		log.WithError(err).Error("update serving environment failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) DeleteServingEnvironment(c *gin.Context) {
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

	if err := h.servingEnvSvc.Delete(c.Request.Context(), projectID, id); err != nil {
		log.WithError(err).Error("delete serving environment failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

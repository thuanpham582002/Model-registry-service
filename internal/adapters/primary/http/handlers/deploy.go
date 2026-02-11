package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/dto"
	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/services"
)

func (h *Handler) DeployModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.DeployModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.deploySvc.Deploy(c.Request.Context(), services.DeployRequest{
		ProjectID:            projectID,
		RegisteredModelID:    req.RegisteredModelID,
		ModelVersionID:       req.ModelVersionID,
		ServingEnvironmentID: req.ServingEnvironmentID,
		Name:                 req.Name,
		Labels:               req.Labels,
	})
	if err != nil {
		log.WithError(err).Error("deploy model failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, dto.DeployModelResponse{
		InferenceService: dto.ToInferenceServiceResponse(result.InferenceService),
		Status:           result.Status,
		Message:          result.Message,
	})
}

func (h *Handler) UndeployModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	isvcID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inference service id"})
		return
	}

	if err := h.deploySvc.Undeploy(c.Request.Context(), projectID, isvcID); err != nil {
		log.WithError(err).Error("undeploy model failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) SyncDeploymentStatus(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	isvcID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inference service id"})
		return
	}

	isvc, err := h.deploySvc.SyncStatus(c.Request.Context(), projectID, isvcID)
	if err != nil {
		log.WithError(err).Error("sync deployment status failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToInferenceServiceResponse(isvc))
}

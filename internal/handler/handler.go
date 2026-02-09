package handler

import (
	"model-registry-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	modelUC    *usecase.RegisteredModelUseCase
	versionUC  *usecase.ModelVersionUseCase
	artifactUC *usecase.ModelArtifactUseCase
}

func New(modelUC *usecase.RegisteredModelUseCase, versionUC *usecase.ModelVersionUseCase, artifactUC *usecase.ModelArtifactUseCase) *Handler {
	return &Handler{
		modelUC:    modelUC,
		versionUC:  versionUC,
		artifactUC: artifactUC,
	}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// Registered Models
	r.GET("/models", h.ListModels)
	r.GET("/models/:id", h.GetModel)
	r.GET("/model", h.GetModelByParams)
	r.POST("/models", h.CreateModel)
	r.PATCH("/models/:id", h.UpdateModel)
	r.DELETE("/models/:id", h.DeleteModel)

	// Model Versions (nested under model)
	r.GET("/models/:id/versions", h.ListModelVersions)
	r.GET("/models/:id/versions/:ver", h.GetModelVersion)
	r.POST("/models/:id/versions", h.CreateModelVersion)
	r.PATCH("/models/:id/versions/:ver", h.UpdateModelVersion)

	// Model Versions (direct access)
	r.GET("/model_version", h.FindModelVersion)
	r.GET("/model_versions", h.ListAllModelVersions)
	r.GET("/model_versions/:id", h.GetModelVersionDirect)
	r.PATCH("/model_versions/:id", h.UpdateModelVersionDirect)

	// Model Artifacts
	r.GET("/model_artifact", h.FindModelArtifact)
	r.GET("/model_artifacts", h.ListModelArtifacts)
	r.GET("/model_artifacts/:id", h.GetModelArtifact)
	r.POST("/model_artifacts", h.CreateModelArtifact)
	r.PATCH("/model_artifacts/:id", h.UpdateModelArtifact)
}

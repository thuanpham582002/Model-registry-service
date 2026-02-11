package handlers

import (
	"model-registry-service/internal/core/services"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	modelSvc        *services.RegisteredModelService
	versionSvc      *services.ModelVersionService
	artifactSvc     *services.ModelArtifactService
	servingEnvSvc   *services.ServingEnvironmentService
	isvcSvc         *services.InferenceServiceService
	serveModelSvc   *services.ServeModelService
	deploySvc       *services.DeployService
	trafficSvc      *services.TrafficService
	virtualModelSvc *services.VirtualModelService
	metricsSvc      *services.MetricsService
}

func New(
	modelSvc *services.RegisteredModelService,
	versionSvc *services.ModelVersionService,
	artifactSvc *services.ModelArtifactService,
	servingEnvSvc *services.ServingEnvironmentService,
	isvcSvc *services.InferenceServiceService,
	serveModelSvc *services.ServeModelService,
	deploySvc *services.DeployService,
	trafficSvc *services.TrafficService,
	virtualModelSvc *services.VirtualModelService,
	metricsSvc *services.MetricsService,
) *Handler {
	return &Handler{
		modelSvc:        modelSvc,
		versionSvc:      versionSvc,
		artifactSvc:     artifactSvc,
		servingEnvSvc:   servingEnvSvc,
		isvcSvc:         isvcSvc,
		serveModelSvc:   serveModelSvc,
		deploySvc:       deploySvc,
		trafficSvc:      trafficSvc,
		virtualModelSvc: virtualModelSvc,
		metricsSvc:      metricsSvc,
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

	// Serving Environments
	r.GET("/serving_environments", h.ListServingEnvironments)
	r.GET("/serving_environments/:id", h.GetServingEnvironment)
	r.GET("/serving_environment", h.FindServingEnvironment)
	r.POST("/serving_environments", h.CreateServingEnvironment)
	r.PATCH("/serving_environments/:id", h.UpdateServingEnvironment)
	r.DELETE("/serving_environments/:id", h.DeleteServingEnvironment)

	// Inference Services
	r.GET("/inference_services", h.ListInferenceServices)
	r.GET("/inference_services/:id", h.GetInferenceService)
	r.GET("/inference_service", h.FindInferenceService)
	r.POST("/inference_services", h.CreateInferenceService)
	r.PATCH("/inference_services/:id", h.UpdateInferenceService)
	r.DELETE("/inference_services/:id", h.DeleteInferenceService)

	// Serve Models
	r.GET("/serve_models", h.ListServeModels)
	r.GET("/serve_models/:id", h.GetServeModel)
	r.POST("/serve_models", h.CreateServeModel)
	r.DELETE("/serve_models/:id", h.DeleteServeModel)

	// Deploy Actions (Click-to-Deploy)
	r.POST("/deploy", h.DeployModel)
	r.DELETE("/inference_services/:id/undeploy", h.UndeployModel)
	r.POST("/inference_services/:id/sync", h.SyncDeploymentStatus)

	// Traffic Configs
	r.GET("/traffic_configs", h.ListTrafficConfigs)
	r.GET("/traffic_configs/:id", h.GetTrafficConfig)
	r.POST("/traffic_configs", h.CreateTrafficConfig)
	r.DELETE("/traffic_configs/:id", h.DeleteTrafficConfig)

	// Traffic Variants
	r.GET("/traffic_configs/:id/variants", h.ListVariants)
	r.GET("/traffic_configs/:id/variants/:name", h.GetVariant)
	r.POST("/traffic_configs/:id/variants", h.AddVariant)
	r.PATCH("/traffic_configs/:id/variants/:name", h.UpdateVariant)
	r.DELETE("/traffic_configs/:id/variants/:name", h.DeleteVariant)

	// Traffic Bulk Operations
	r.PATCH("/traffic_configs/:id/weights", h.BulkUpdateWeights)
	r.POST("/traffic_configs/:id/promote/:variant_name", h.PromoteVariant)
	r.POST("/traffic_configs/:id/rollback", h.Rollback)

	// Canary Operations (convenience endpoints)
	r.POST("/traffic_configs/:id/canary", h.StartCanary)
	r.PATCH("/traffic_configs/:id/canary/weight", h.UpdateCanaryWeight)
	r.POST("/traffic_configs/:id/canary/promote", h.PromoteCanary)

	// Virtual Models
	r.GET("/virtual_models", h.ListVirtualModels)
	r.GET("/virtual_models/:name", h.GetVirtualModel)
	r.POST("/virtual_models", h.CreateVirtualModel)
	r.DELETE("/virtual_models/:name", h.DeleteVirtualModel)

	// Virtual Model Backends
	r.GET("/virtual_models/:name/backends", h.ListVirtualModelBackends)
	r.POST("/virtual_models/:name/backends", h.AddVirtualModelBackend)
	r.PATCH("/virtual_models/:name/backends/:backend_id", h.UpdateVirtualModelBackend)
	r.DELETE("/virtual_models/:name/backends/:backend_id", h.DeleteVirtualModelBackend)

	// Metrics
	r.GET("/metrics/deployments/:isvc_name", h.GetDeploymentMetrics)
	r.GET("/metrics/compare/:isvc_name", h.CompareVariants)
	r.GET("/metrics/tokens", h.GetTokenUsageMetrics)
}

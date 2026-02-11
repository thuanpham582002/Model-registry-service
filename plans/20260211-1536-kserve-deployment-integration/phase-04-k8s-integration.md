# Phase 4: Kubernetes / KServe Integration

## Objective
Integrate with Kubernetes to create/delete KServe InferenceService CRs.

## Tasks

### 4.1 Add K8s Configuration

**File**: `internal/config/config.go` (append)

```go
type KubernetesConfig struct {
	Enabled        bool   `mapstructure:"KSERVE_ENABLED"`
	InCluster      bool   `mapstructure:"KUBERNETES_IN_CLUSTER"`
	KubeConfigPath string `mapstructure:"KUBERNETES_KUBECONFIG"`
	DefaultNS      string `mapstructure:"KUBERNETES_DEFAULT_NAMESPACE"`
}

// In Load() function, add:
func Load() (*Config, error) {
	// ... existing code ...

	cfg := &Config{
		// ... existing fields ...
		Kubernetes: KubernetesConfig{
			Enabled:        v.GetBool("KSERVE_ENABLED"),
			InCluster:      v.GetBool("KUBERNETES_IN_CLUSTER"),
			KubeConfigPath: v.GetString("KUBERNETES_KUBECONFIG"),
			DefaultNS:      v.GetString("KUBERNETES_DEFAULT_NAMESPACE"),
		},
	}

	// Set defaults
	if cfg.Kubernetes.DefaultNS == "" {
		cfg.Kubernetes.DefaultNS = "model-serving"
	}

	return cfg, nil
}
```

### 4.2 Add Dependencies

**File**: `go.mod` (add)

```go
require (
	github.com/kserve/kserve v0.13.0
	k8s.io/api v0.29.0
	k8s.io/apimachinery v0.29.0
	k8s.io/client-go v0.29.0
)
```

### 4.3 Deploy Service

**File**: `internal/service/kserve_client.go`

```go
package service

import (
	"context"
	"fmt"

	kservev1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"model-registry-service/internal/config"
	"model-registry-service/internal/domain"
)

var inferenceServiceGVR = schema.GroupVersionResource{
	Group:    "serving.kserve.io",
	Version:  "v1beta1",
	Resource: "inferenceservices",
}

type KServeClient struct {
	dynamicClient dynamic.Interface
	defaultNS     string
}

func NewKServeClient(cfg *config.KubernetesConfig) (*KServeClient, error) {
	if !cfg.Enabled {
		return nil, nil // KServe disabled
	}

	var restCfg *rest.Config
	var err error

	if cfg.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else if cfg.KubeConfigPath != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfigPath)
	} else {
		// Try default kubeconfig location
		restCfg, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	}
	if err != nil {
		return nil, fmt.Errorf("build k8s config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	return &KServeClient{
		dynamicClient: dynamicClient,
		defaultNS:     cfg.DefaultNS,
	}, nil
}

func (c *KServeClient) CreateInferenceService(ctx context.Context, namespace string, isvc *domain.InferenceService, version *domain.ModelVersion) (string, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	// Build the InferenceService CR
	obj := c.buildInferenceServiceCR(isvc, version)

	// Create in K8s
	created, err := c.dynamicClient.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create kserve inferenceservice: %w", err)
	}

	return string(created.GetUID()), nil
}

func (c *KServeClient) DeleteInferenceService(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		namespace = c.defaultNS
	}

	err := c.dynamicClient.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete kserve inferenceservice: %w", err)
	}

	return nil
}

func (c *KServeClient) GetInferenceService(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj, err := c.dynamicClient.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *KServeClient) buildInferenceServiceCR(isvc *domain.InferenceService, version *domain.ModelVersion) *unstructured.Unstructured {
	labels := map[string]interface{}{
		"modelregistry.ai-platform/inference-service-id": isvc.ID.String(),
		"modelregistry.ai-platform/registered-model-id":  isvc.RegisteredModelID.String(),
	}
	if isvc.ModelVersionID != nil {
		labels["modelregistry.ai-platform/model-version-id"] = isvc.ModelVersionID.String()
	}

	// Merge user labels
	for k, v := range isvc.Labels {
		labels[k] = v
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":   isvc.Name,
				"labels": labels,
			},
			"spec": map[string]interface{}{
				"predictor": map[string]interface{}{
					"model": map[string]interface{}{
						"modelFormat": map[string]interface{}{
							"name": version.ModelFramework,
						},
						"storageUri": version.URI,
					},
				},
			},
		},
	}

	// Add container image if specified
	if version.ContainerImage != "" {
		predictor := obj.Object["spec"].(map[string]interface{})["predictor"].(map[string]interface{})
		model := predictor["model"].(map[string]interface{})
		model["runtime"] = version.ContainerImage
	}

	return obj
}

// ParseInferenceServiceStatus extracts status from KServe CR
func (c *KServeClient) ParseInferenceServiceStatus(obj *unstructured.Unstructured) (url string, ready bool, err error) {
	status, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return "", false, nil
	}

	// Get URL
	urlVal, _, _ := unstructured.NestedString(status, "url")

	// Check conditions for ready state
	conditions, found, _ := unstructured.NestedSlice(status, "conditions")
	if found {
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			if condMap["type"] == "Ready" && condMap["status"] == "True" {
				ready = true
				break
			}
		}
	}

	return urlVal, ready, nil
}
```

### 4.4 Deploy Usecase

**File**: `internal/usecase/deploy.go`

```go
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/domain"
	"model-registry-service/internal/service"
)

type DeployUseCase struct {
	isvcRepo    domain.InferenceServiceRepository
	envRepo     domain.ServingEnvironmentRepository
	modelRepo   domain.RegisteredModelRepository
	versionRepo domain.ModelVersionRepository
	kserve      *service.KServeClient
}

func NewDeployUseCase(
	isvcRepo domain.InferenceServiceRepository,
	envRepo domain.ServingEnvironmentRepository,
	modelRepo domain.RegisteredModelRepository,
	versionRepo domain.ModelVersionRepository,
	kserve *service.KServeClient,
) *DeployUseCase {
	return &DeployUseCase{
		isvcRepo:    isvcRepo,
		envRepo:     envRepo,
		modelRepo:   modelRepo,
		versionRepo: versionRepo,
		kserve:      kserve,
	}
}

type DeployRequest struct {
	RegisteredModelID    uuid.UUID
	ModelVersionID       *uuid.UUID
	ServingEnvironmentID uuid.UUID
	Name                 string
}

type DeployResult struct {
	InferenceServiceID uuid.UUID
	Status             string
	Message            string
}

func (uc *DeployUseCase) Deploy(ctx context.Context, projectID uuid.UUID, req DeployRequest) (*DeployResult, error) {
	// 1. Validate environment
	env, err := uc.envRepo.GetByID(ctx, projectID, req.ServingEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}

	// 2. Get model
	model, err := uc.modelRepo.GetByID(ctx, projectID, req.RegisteredModelID)
	if err != nil {
		return nil, fmt.Errorf("get model: %w", err)
	}

	// 3. Get version (default if not specified)
	var version *domain.ModelVersion
	if req.ModelVersionID != nil {
		version, err = uc.versionRepo.GetByID(ctx, projectID, *req.ModelVersionID)
		if err != nil {
			return nil, fmt.Errorf("get version: %w", err)
		}
	} else if model.DefaultVersion != nil {
		version = model.DefaultVersion
	} else if model.LatestVersion != nil {
		version = model.LatestVersion
	}
	if version == nil {
		return nil, fmt.Errorf("no model version available")
	}

	// 4. Generate name
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s-%s", model.Slug, version.ID.String()[:8])
	}

	// 5. Create InferenceService record
	isvc := &domain.InferenceService{
		ID:                   uuid.New(),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		ProjectID:            projectID,
		Name:                 name,
		ServingEnvironmentID: env.ID,
		RegisteredModelID:    model.ID,
		ModelVersionID:       &version.ID,
		DesiredState:         domain.ISStateDeployed,
		CurrentState:         domain.ISStateUndeployed,
		Runtime:              "kserve",
	}

	if err := uc.isvcRepo.Create(ctx, isvc); err != nil {
		return nil, fmt.Errorf("create inference service: %w", err)
	}

	// 6. Create KServe CR (if client available)
	if uc.kserve != nil {
		externalID, err := uc.kserve.CreateInferenceService(ctx, env.Name, isvc, version)
		if err != nil {
			// Update with error
			isvc.LastError = err.Error()
			isvc.CurrentState = domain.ISStateUndeployed
			uc.isvcRepo.Update(ctx, projectID, isvc)
			return &DeployResult{
				InferenceServiceID: isvc.ID,
				Status:             "FAILED",
				Message:            err.Error(),
			}, nil
		}

		isvc.ExternalID = externalID
		uc.isvcRepo.Update(ctx, projectID, isvc)
	}

	return &DeployResult{
		InferenceServiceID: isvc.ID,
		Status:             "PENDING",
		Message:            "Deployment initiated",
	}, nil
}

func (uc *DeployUseCase) Undeploy(ctx context.Context, projectID, isvcID uuid.UUID) error {
	// 1. Get inference service
	isvc, err := uc.isvcRepo.GetByID(ctx, projectID, isvcID)
	if err != nil {
		return err
	}

	// 2. Get environment
	env, err := uc.envRepo.GetByID(ctx, projectID, isvc.ServingEnvironmentID)
	if err != nil {
		return err
	}

	// 3. Delete from K8s
	if uc.kserve != nil {
		if err := uc.kserve.DeleteInferenceService(ctx, env.Name, isvc.Name); err != nil {
			// Log but continue - might already be deleted
		}
	}

	// 4. Update state
	isvc.DesiredState = domain.ISStateUndeployed
	isvc.CurrentState = domain.ISStateUndeployed
	return uc.isvcRepo.Update(ctx, projectID, isvc)
}

func (uc *DeployUseCase) SyncStatus(ctx context.Context, projectID, isvcID uuid.UUID) error {
	if uc.kserve == nil {
		return nil
	}

	isvc, err := uc.isvcRepo.GetByID(ctx, projectID, isvcID)
	if err != nil {
		return err
	}

	env, err := uc.envRepo.GetByID(ctx, projectID, isvc.ServingEnvironmentID)
	if err != nil {
		return err
	}

	obj, err := uc.kserve.GetInferenceService(ctx, env.Name, isvc.Name)
	if err != nil {
		return err
	}

	url, ready, _ := uc.kserve.ParseInferenceServiceStatus(obj)
	if ready {
		isvc.CurrentState = domain.ISStateDeployed
	}
	isvc.URL = url

	return uc.isvcRepo.Update(ctx, projectID, isvc)
}
```

### 4.5 Deploy Handler

**File**: `internal/handler/deploy.go`

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"model-registry-service/internal/usecase"
)

type DeployModelRequest struct {
	RegisteredModelID    uuid.UUID  `json:"registered_model_id" binding:"required"`
	ModelVersionID       *uuid.UUID `json:"model_version_id"`
	ServingEnvironmentID uuid.UUID  `json:"serving_environment_id" binding:"required"`
	Name                 string     `json:"name"`
}

func (h *Handler) DeployModel(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
		return
	}

	var req DeployModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.deployUC.Deploy(c.Request.Context(), projectID, usecase.DeployRequest{
		RegisteredModelID:    req.RegisteredModelID,
		ModelVersionID:       req.ModelVersionID,
		ServingEnvironmentID: req.ServingEnvironmentID,
		Name:                 req.Name,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, result)
}

func (h *Handler) UndeployModel(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
		return
	}

	isvcID, err := uuid.Parse(c.Param("isvc_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid isvc_id"})
		return
	}

	if err := h.deployUC.Undeploy(c.Request.Context(), projectID, isvcID); err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
```

### 4.6 Wire Everything in main.go

**File**: `cmd/server/main.go` (update)

```go
func main() {
	// ... existing code ...

	// Create KServe client (if enabled)
	var kserveClient *service.KServeClient
	if cfg.Kubernetes.Enabled {
		var err error
		kserveClient, err = service.NewKServeClient(&cfg.Kubernetes)
		if err != nil {
			log.Warnf("KServe client init failed: %v", err)
		} else {
			log.Info("KServe client initialized")
		}
	}

	// Create repositories
	modelRepo := repository.NewRegisteredModelRepository(pool)
	versionRepo := repository.NewModelVersionRepository(pool)
	envRepo := repository.NewServingEnvironmentRepository(pool)
	isvcRepo := repository.NewInferenceServiceRepository(pool)

	// Create usecases
	modelUC := usecase.NewRegisteredModelUseCase(modelRepo)
	versionUC := usecase.NewModelVersionUseCase(versionRepo, modelRepo)
	artifactUC := usecase.NewModelArtifactUseCase(versionRepo, modelRepo)
	servingEnvUC := usecase.NewServingEnvironmentUseCase(envRepo)
	isvcUC := usecase.NewInferenceServiceUseCase(isvcRepo, envRepo, modelRepo, versionRepo)
	deployUC := usecase.NewDeployUseCase(isvcRepo, envRepo, modelRepo, versionRepo, kserveClient)

	h := handler.New(modelUC, versionUC, artifactUC, servingEnvUC, isvcUC, deployUC)

	// ... rest of existing code ...
}
```

## Environment Variables

```bash
# Enable KServe integration
KSERVE_ENABLED=true

# K8s config (one of these)
KUBERNETES_IN_CLUSTER=true              # Use in-cluster config
KUBERNETES_KUBECONFIG=~/.kube/config    # Use local kubeconfig

# Default namespace for deployments
KUBERNETES_DEFAULT_NAMESPACE=model-serving
```

## RBAC Requirements

The service needs K8s RBAC permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: model-registry-kserve
rules:
  - apiGroups: ["serving.kserve.io"]
    resources: ["inferenceservices"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

## Checklist

- [ ] Add KubernetesConfig to config
- [ ] Add K8s dependencies to go.mod
- [ ] Create `internal/service/kserve_client.go`
- [ ] Create `internal/usecase/deploy.go`
- [ ] Create `internal/handler/deploy.go`
- [ ] Update handler struct with deployUC
- [ ] Add routes: POST /deploy, DELETE /undeploy/:isvc_id
- [ ] Wire in main.go
- [ ] Test with minikube + KServe
- [ ] Create K8s RBAC manifests

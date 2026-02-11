# KServe Deployment Integration Plan

**Date**: 2026-02-11
**Status**: Revised (v4 - Hexagonal + API Preservation)
**Author**: Claude

## Executive Summary

Port Kubeflow Model Registry's click-to-deployment functionality using **Hexagonal Architecture** (Ports & Adapters).

### Key Constraints

1. **Existing API unchanged** - All 19 routes keep exact paths, request/response formats
2. **Internal restructure only** - Move existing code to Hexagonal layout
3. **New serving endpoints follow same API style** - Consistent naming conventions

---

## Architecture: Hexagonal (Ports & Adapters)

```
                    ┌─────────────────────────────────────┐
                    │           PRIMARY ADAPTERS          │
                    │  (HTTP Handlers, gRPC, CLI, etc.)   │
                    └──────────────┬──────────────────────┘
                                   │
                                   ▼
┌──────────────────────────────────────────────────────────────────┐
│                         INPUT PORTS                               │
│              (Service Interfaces - what app CAN do)               │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│    ┌─────────────────────────────────────────────────────────┐   │
│    │                    CORE DOMAIN                          │   │
│    │  • Entities (RegisteredModel, InferenceService, etc.)   │   │
│    │  • Value Objects (ModelState, DeploymentStatus)         │   │
│    │  • Domain Services (DeploymentService)                  │   │
│    │  • Domain Errors                                        │   │
│    │  • NO EXTERNAL DEPENDENCIES                             │   │
│    └─────────────────────────────────────────────────────────┘   │
│                                                                   │
├──────────────────────────────────────────────────────────────────┤
│                        OUTPUT PORTS                               │
│           (Repository Interfaces - what app NEEDS)                │
└──────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────────────────┐
                    │         SECONDARY ADAPTERS          │
                    │  (PostgreSQL, KServe, S3, etc.)     │
                    └─────────────────────────────────────┘
```

### Key Principles

1. **Domain has ZERO external dependencies** - only stdlib
2. **Ports define contracts** - interfaces owned by the core
3. **Adapters implement ports** - can be swapped (PostgreSQL → MySQL)
4. **Dependency flows inward** - outer layers depend on inner
5. **Testability** - mock adapters for unit testing core logic

---

## Migration Strategy

**Approach**: Build new serving features in Hexagonal, migrate existing code gradually.

```
CURRENT                          →  HEXAGONAL
─────────────────────────────────────────────────────────
internal/domain/                 →  internal/core/domain/        (move)
internal/domain/interfaces       →  internal/core/ports/output/  (move)
internal/usecase/                →  internal/core/services/      (move)
internal/repository/             →  internal/adapters/secondary/postgres/ (move)
internal/handler/                →  internal/adapters/primary/http/handlers/ (move)
internal/dto/                    →  internal/adapters/primary/http/dto/ (move)
internal/middleware/             →  internal/adapters/primary/http/middleware/ (move)
internal/config/                 →  internal/config/             (keep)

NEW (serving features):
- internal/core/domain/serving.go
- internal/core/ports/output/kserve_client.go
- internal/core/services/serving_service.go
- internal/adapters/secondary/kserve/
- internal/adapters/primary/http/handlers/serving_handler.go
```

---

## Final Project Structure

```
internal/
├── core/                           # 🎯 BUSINESS LOGIC (no external deps)
│   ├── domain/                     # Entities & Value Objects
│   │   ├── model.go                # (moved) RegisteredModel, ModelVersion, ModelArtifact
│   │   ├── serving.go              # (NEW) ServingEnvironment, InferenceService, ServeModel
│   │   └── errors.go               # (moved + extended) Domain errors
│   │
│   ├── ports/                      # Interfaces (contracts)
│   │   └── output/                 # What the app NEEDS (repositories, clients)
│   │       ├── model_repository.go     # (moved from domain/interfaces)
│   │       ├── version_repository.go   # (moved from domain/interfaces)
│   │       ├── serving_repository.go   # (NEW)
│   │       └── kserve_client.go        # (NEW)
│   │
│   └── services/                   # Application services (use cases)
│       ├── model_service.go            # (moved from usecase/)
│       ├── version_service.go          # (moved from usecase/)
│       ├── artifact_service.go         # (moved from usecase/)
│       ├── serving_service.go          # (NEW)
│       └── deploy_service.go           # (NEW)
│
├── adapters/                       # 🔌 INFRASTRUCTURE
│   ├── primary/                    # Incoming adapters
│   │   └── http/                   # REST API
│   │       ├── router.go               # (NEW) centralized routing
│   │       ├── middleware/             # (moved from middleware/)
│   │       │   ├── logging.go
│   │       │   └── request_id.go
│   │       ├── handlers/               # (moved from handler/)
│   │       │   ├── model_handler.go    # (moved, unchanged logic)
│   │       │   ├── version_handler.go  # (moved, unchanged logic)
│   │       │   ├── artifact_handler.go # (moved, unchanged logic)
│   │       │   ├── serving_handler.go  # (NEW)
│   │       │   └── deploy_handler.go   # (NEW)
│   │       ├── dto/                    # (moved from dto/)
│   │       │   ├── model.go            # UNCHANGED - same request/response
│   │       │   ├── version.go          # UNCHANGED - same request/response
│   │       │   ├── artifact.go         # UNCHANGED - same request/response
│   │       │   ├── serving.go          # (NEW)
│   │       │   └── common.go           # (moved, pagination etc)
│   │       └── mapper/
│   │           └── mapper.go           # (moved from dto/)
│   │
│   └── secondary/                  # Outgoing adapters
│       ├── postgres/               # (moved from repository/)
│       │   ├── model_repository.go
│       │   ├── version_repository.go
│       │   └── serving_repository.go   # (NEW)
│       │
│       └── kserve/                 # (NEW) KServe K8s client
│           └── client.go
│
├── config/                         # (keep as-is)
│   └── config.go
│
└── testutil/                       # (keep as-is)
    └── mock_repository.go

cmd/
└── server/
    └── main.go                     # Updated wiring
```

---

## API Routes (PRESERVED + NEW)

### Existing Routes (UNCHANGED)

| Method | Path | Status |
|--------|------|--------|
| POST | `/api/v1/model-registry/models` | ✅ Keep |
| GET | `/api/v1/model-registry/models` | ✅ Keep |
| GET | `/api/v1/model-registry/models/:id` | ✅ Keep |
| GET | `/api/v1/model-registry/model` | ✅ Keep |
| PATCH | `/api/v1/model-registry/models/:id` | ✅ Keep |
| DELETE | `/api/v1/model-registry/models/:id` | ✅ Keep |
| POST | `/api/v1/model-registry/models/:id/versions` | ✅ Keep |
| GET | `/api/v1/model-registry/models/:id/versions` | ✅ Keep |
| GET | `/api/v1/model-registry/models/:id/versions/:ver` | ✅ Keep |
| PATCH | `/api/v1/model-registry/models/:id/versions/:ver` | ✅ Keep |
| GET | `/api/v1/model-registry/model_versions` | ✅ Keep |
| GET | `/api/v1/model-registry/model_versions/:id` | ✅ Keep |
| GET | `/api/v1/model-registry/model_version` | ✅ Keep |
| PATCH | `/api/v1/model-registry/model_versions/:id` | ✅ Keep |
| POST | `/api/v1/model-registry/model_artifacts` | ✅ Keep |
| GET | `/api/v1/model-registry/model_artifacts` | ✅ Keep |
| GET | `/api/v1/model-registry/model_artifacts/:id` | ✅ Keep |
| GET | `/api/v1/model-registry/model_artifact` | ✅ Keep |
| PATCH | `/api/v1/model-registry/model_artifacts/:id` | ✅ Keep |

### New Serving Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/model-registry/serving_environments` | Create environment |
| GET | `/api/v1/model-registry/serving_environments` | List environments |
| GET | `/api/v1/model-registry/serving_environments/:id` | Get environment |
| PATCH | `/api/v1/model-registry/serving_environments/:id` | Update environment |
| DELETE | `/api/v1/model-registry/serving_environments/:id` | Delete environment |
| POST | `/api/v1/model-registry/inference_services` | Create inference svc |
| GET | `/api/v1/model-registry/inference_services` | List inference svcs |
| GET | `/api/v1/model-registry/inference_services/:id` | Get inference svc |
| PATCH | `/api/v1/model-registry/inference_services/:id` | Update inference svc |
| DELETE | `/api/v1/model-registry/inference_services/:id` | Delete inference svc |
| POST | `/api/v1/model-registry/serve_models` | Create serve model |
| GET | `/api/v1/model-registry/serve_models` | List serve models |
| GET | `/api/v1/model-registry/serve_models/:id` | Get serve model |
| DELETE | `/api/v1/model-registry/serve_models/:id` | Delete serve model |

---

## Phase 1: Core Domain Layer

### 1.1 Domain Entities

**File**: `internal/core/domain/serving.go`

```go
package domain

import (
	"time"
	"github.com/google/uuid"
)

// Value Objects
type InferenceServiceState string

const (
	ISStateDeployed   InferenceServiceState = "DEPLOYED"
	ISStateUndeployed InferenceServiceState = "UNDEPLOYED"
)

func (s InferenceServiceState) IsValid() bool {
	return s == ISStateDeployed || s == ISStateUndeployed
}

type ServeModelState string

const (
	ServeStatePending ServeModelState = "PENDING"
	ServeStateRunning ServeModelState = "RUNNING"
	ServeStateFailed  ServeModelState = "FAILED"
)

// Entities

type ServingEnvironment struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ProjectID   uuid.UUID
	Name        string
	Description string
	ExternalID  string
}

// NewServingEnvironment creates a new ServingEnvironment with validation
func NewServingEnvironment(projectID uuid.UUID, name, description string) (*ServingEnvironment, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if projectID == uuid.Nil {
		return nil, ErrInvalidProjectID
	}
	return &ServingEnvironment{
		ID:          uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		ProjectID:   projectID,
		Name:        name,
		Description: description,
	}, nil
}

type InferenceService struct {
	ID                   uuid.UUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ProjectID            uuid.UUID
	Name                 string
	ExternalID           string // K8s resource UID
	ServingEnvironmentID uuid.UUID
	RegisteredModelID    uuid.UUID
	ModelVersionID       *uuid.UUID
	DesiredState         InferenceServiceState
	CurrentState         InferenceServiceState
	Runtime              string
	URL                  string
	LastError            string
	Labels               map[string]string
}

// NewInferenceService creates a new InferenceService with validation
func NewInferenceService(
	projectID uuid.UUID,
	name string,
	envID uuid.UUID,
	modelID uuid.UUID,
	versionID *uuid.UUID,
) (*InferenceService, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if projectID == uuid.Nil || envID == uuid.Nil || modelID == uuid.Nil {
		return nil, ErrInvalidID
	}
	return &InferenceService{
		ID:                   uuid.New(),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		ProjectID:            projectID,
		Name:                 name,
		ServingEnvironmentID: envID,
		RegisteredModelID:    modelID,
		ModelVersionID:       versionID,
		DesiredState:         ISStateDeployed,
		CurrentState:         ISStateUndeployed,
		Runtime:              "kserve",
		Labels:               make(map[string]string),
	}, nil
}

// Deploy marks the service for deployment
func (is *InferenceService) Deploy() {
	is.DesiredState = ISStateDeployed
	is.UpdatedAt = time.Now()
}

// Undeploy marks the service for undeployment
func (is *InferenceService) Undeploy() {
	is.DesiredState = ISStateUndeployed
	is.UpdatedAt = time.Now()
}

// MarkDeployed updates current state to deployed
func (is *InferenceService) MarkDeployed(url string) {
	is.CurrentState = ISStateDeployed
	is.URL = url
	is.LastError = ""
	is.UpdatedAt = time.Now()
}

// MarkFailed records deployment failure
func (is *InferenceService) MarkFailed(err string) {
	is.LastError = err
	is.UpdatedAt = time.Now()
}

// IsDeployed returns true if currently deployed
func (is *InferenceService) IsDeployed() bool {
	return is.CurrentState == ISStateDeployed
}

type ServeModel struct {
	ID                 uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ProjectID          uuid.UUID
	InferenceServiceID uuid.UUID
	ModelVersionID     uuid.UUID
	LastKnownState     ServeModelState
}
```

### 1.2 Domain Errors

**File**: `internal/core/domain/errors.go`

```go
package domain

import "errors"

// Validation errors
var (
	ErrInvalidName      = errors.New("invalid name: cannot be empty")
	ErrInvalidID        = errors.New("invalid id: cannot be nil")
	ErrInvalidProjectID = errors.New("invalid project id")
	ErrInvalidState     = errors.New("invalid state")
)

// Not found errors
var (
	ErrModelNotFound           = errors.New("registered model not found")
	ErrVersionNotFound         = errors.New("model version not found")
	ErrServingEnvNotFound      = errors.New("serving environment not found")
	ErrInferenceServiceNotFound = errors.New("inference service not found")
	ErrServeModelNotFound      = errors.New("serve model not found")
)

// Conflict errors
var (
	ErrModelNameConflict           = errors.New("model name already exists")
	ErrServingEnvNameConflict      = errors.New("serving environment name already exists")
	ErrInferenceServiceNameConflict = errors.New("inference service name already exists")
)

// Business rule errors
var (
	ErrModelHasActiveDeployments = errors.New("model has active deployments")
	ErrCannotDeleteDeployed      = errors.New("cannot delete deployed inference service")
	ErrVersionNotReady           = errors.New("model version is not ready for deployment")
)
```

---

## Phase 2: Ports (Interfaces)

### 2.1 Output Ports (Repositories)

**File**: `internal/core/ports/output/serving_repository.go`

```go
package output

import (
	"context"
	"github.com/google/uuid"
	"model-registry-service/internal/core/domain"
)

// ServingEnvironmentRepository defines the contract for serving environment persistence
type ServingEnvironmentRepository interface {
	Save(ctx context.Context, env *domain.ServingEnvironment) error
	FindByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServingEnvironment, error)
	FindByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.ServingEnvironment, error)
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	FindAll(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*domain.ServingEnvironment, int, error)
}

// InferenceServiceRepository defines the contract for inference service persistence
type InferenceServiceRepository interface {
	Save(ctx context.Context, isvc *domain.InferenceService) error
	FindByID(ctx context.Context, projectID, id uuid.UUID) (*domain.InferenceService, error)
	FindByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*domain.InferenceService, error)
	FindByName(ctx context.Context, projectID, envID uuid.UUID, name string) (*domain.InferenceService, error)
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	FindAll(ctx context.Context, filter InferenceServiceFilter) ([]*domain.InferenceService, int, error)
	CountByModel(ctx context.Context, projectID, modelID uuid.UUID) (int, error)
}

type InferenceServiceFilter struct {
	ProjectID            uuid.UUID
	ServingEnvironmentID *uuid.UUID
	RegisteredModelID    *uuid.UUID
	ModelVersionID       *uuid.UUID
	State                string
	SortBy               string
	Order                string
	Limit                int
	Offset               int
}

// ServeModelRepository defines the contract for serve model persistence
type ServeModelRepository interface {
	Save(ctx context.Context, sm *domain.ServeModel) error
	FindByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServeModel, error)
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	FindByInferenceService(ctx context.Context, projectID, isvcID uuid.UUID) ([]*domain.ServeModel, error)
}
```

**File**: `internal/core/ports/output/kserve_client.go`

```go
package output

import (
	"context"
	"model-registry-service/internal/core/domain"
)

// KServeDeployment represents the result of a KServe deployment
type KServeDeployment struct {
	ExternalID string // K8s resource UID
	URL        string // Inference endpoint URL
}

// KServeClient defines the contract for KServe/K8s operations
type KServeClient interface {
	// Deploy creates a KServe InferenceService CR
	Deploy(ctx context.Context, namespace string, isvc *domain.InferenceService, version *domain.ModelVersion) (*KServeDeployment, error)

	// Undeploy deletes the KServe InferenceService CR
	Undeploy(ctx context.Context, namespace, name string) error

	// GetStatus retrieves current deployment status
	GetStatus(ctx context.Context, namespace, name string) (url string, ready bool, err error)

	// IsAvailable checks if KServe integration is enabled
	IsAvailable() bool
}
```

### 2.2 Input Ports (Services)

**File**: `internal/core/ports/input/deploy_service.go`

```go
package input

import (
	"context"
	"github.com/google/uuid"
	"model-registry-service/internal/core/domain"
)

// DeployRequest contains deployment parameters
type DeployRequest struct {
	ProjectID            uuid.UUID
	RegisteredModelID    uuid.UUID
	ModelVersionID       *uuid.UUID // nil = use default version
	ServingEnvironmentID uuid.UUID
	Name                 string // optional, auto-generated if empty
}

// DeployResult contains deployment outcome
type DeployResult struct {
	InferenceService *domain.InferenceService
	Status           string // PENDING, DEPLOYED, FAILED
	Message          string
}

// DeployService defines the contract for model deployment operations
type DeployService interface {
	// Deploy deploys a model version to KServe
	Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error)

	// Undeploy removes a deployment
	Undeploy(ctx context.Context, projectID, inferenceServiceID uuid.UUID) error

	// SyncStatus synchronizes status from K8s
	SyncStatus(ctx context.Context, projectID, inferenceServiceID uuid.UUID) error
}

// ServingEnvironmentService defines operations for serving environments
type ServingEnvironmentService interface {
	Create(ctx context.Context, projectID uuid.UUID, name, description string) (*domain.ServingEnvironment, error)
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServingEnvironment, error)
	GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.ServingEnvironment, error)
	Update(ctx context.Context, projectID, id uuid.UUID, name, description *string) (*domain.ServingEnvironment, error)
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	List(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*domain.ServingEnvironment, int, error)
}

// InferenceServiceService defines operations for inference services
type InferenceServiceService interface {
	Create(ctx context.Context, projectID uuid.UUID, req CreateInferenceServiceRequest) (*domain.InferenceService, error)
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.InferenceService, error)
	Update(ctx context.Context, projectID, id uuid.UUID, req UpdateInferenceServiceRequest) (*domain.InferenceService, error)
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	List(ctx context.Context, filter InferenceServiceListFilter) ([]*domain.InferenceService, int, error)
}

type CreateInferenceServiceRequest struct {
	Name                 string
	ServingEnvironmentID uuid.UUID
	RegisteredModelID    uuid.UUID
	ModelVersionID       *uuid.UUID
	Runtime              string
	Labels               map[string]string
}

type UpdateInferenceServiceRequest struct {
	Name           *string
	DesiredState   *domain.InferenceServiceState
	ModelVersionID *uuid.UUID
	Labels         map[string]string
}

type InferenceServiceListFilter struct {
	ProjectID            uuid.UUID
	ServingEnvironmentID *uuid.UUID
	RegisteredModelID    *uuid.UUID
	State                string
	Limit                int
	Offset               int
}
```

---

## Phase 3: Application Services

**File**: `internal/core/services/deploy_service.go`

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/input"
	"model-registry-service/internal/core/ports/output"
)

// deployService implements input.DeployService
type deployService struct {
	envRepo     output.ServingEnvironmentRepository
	isvcRepo    output.InferenceServiceRepository
	modelRepo   output.ModelRepository
	versionRepo output.ModelVersionRepository
	kserve      output.KServeClient
}

// NewDeployService creates a new deploy service
func NewDeployService(
	envRepo output.ServingEnvironmentRepository,
	isvcRepo output.InferenceServiceRepository,
	modelRepo output.ModelRepository,
	versionRepo output.ModelVersionRepository,
	kserve output.KServeClient,
) input.DeployService {
	return &deployService{
		envRepo:     envRepo,
		isvcRepo:    isvcRepo,
		modelRepo:   modelRepo,
		versionRepo: versionRepo,
		kserve:      kserve,
	}
}

func (s *deployService) Deploy(ctx context.Context, req input.DeployRequest) (*input.DeployResult, error) {
	// 1. Validate serving environment exists
	env, err := s.envRepo.FindByID(ctx, req.ProjectID, req.ServingEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("find serving environment: %w", err)
	}

	// 2. Get model
	model, err := s.modelRepo.FindByID(ctx, req.ProjectID, req.RegisteredModelID)
	if err != nil {
		return nil, fmt.Errorf("find model: %w", err)
	}

	// 3. Get version (default if not specified)
	version, err := s.resolveVersion(ctx, req.ProjectID, req.RegisteredModelID, req.ModelVersionID)
	if err != nil {
		return nil, err
	}

	// 4. Validate version is ready
	if version.Status != domain.VersionStatusReady {
		return nil, domain.ErrVersionNotReady
	}

	// 5. Create InferenceService entity
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s-%s", model.Slug, version.ID.String()[:8])
	}

	isvc, err := domain.NewInferenceService(req.ProjectID, name, env.ID, model.ID, &version.ID)
	if err != nil {
		return nil, err
	}

	// 6. Save to database
	if err := s.isvcRepo.Save(ctx, isvc); err != nil {
		return nil, fmt.Errorf("save inference service: %w", err)
	}

	// 7. Deploy to KServe (if available)
	if s.kserve.IsAvailable() {
		deployment, err := s.kserve.Deploy(ctx, env.Name, isvc, version)
		if err != nil {
			isvc.MarkFailed(err.Error())
			s.isvcRepo.Save(ctx, isvc) // Update with error
			return &input.DeployResult{
				InferenceService: isvc,
				Status:           "FAILED",
				Message:          err.Error(),
			}, nil
		}
		isvc.ExternalID = deployment.ExternalID
		s.isvcRepo.Save(ctx, isvc)
	}

	return &input.DeployResult{
		InferenceService: isvc,
		Status:           "PENDING",
		Message:          "Deployment initiated",
	}, nil
}

func (s *deployService) Undeploy(ctx context.Context, projectID, isvcID uuid.UUID) error {
	// 1. Get inference service
	isvc, err := s.isvcRepo.FindByID(ctx, projectID, isvcID)
	if err != nil {
		return err
	}

	// 2. Get environment for namespace
	env, err := s.envRepo.FindByID(ctx, projectID, isvc.ServingEnvironmentID)
	if err != nil {
		return err
	}

	// 3. Delete from KServe
	if s.kserve.IsAvailable() {
		if err := s.kserve.Undeploy(ctx, env.Name, isvc.Name); err != nil {
			// Log but continue - might already be deleted
		}
	}

	// 4. Update state
	isvc.Undeploy()
	isvc.CurrentState = domain.ISStateUndeployed
	return s.isvcRepo.Save(ctx, isvc)
}

func (s *deployService) SyncStatus(ctx context.Context, projectID, isvcID uuid.UUID) error {
	if !s.kserve.IsAvailable() {
		return nil
	}

	isvc, err := s.isvcRepo.FindByID(ctx, projectID, isvcID)
	if err != nil {
		return err
	}

	env, err := s.envRepo.FindByID(ctx, projectID, isvc.ServingEnvironmentID)
	if err != nil {
		return err
	}

	url, ready, err := s.kserve.GetStatus(ctx, env.Name, isvc.Name)
	if err != nil {
		return err
	}

	if ready {
		isvc.MarkDeployed(url)
	}

	return s.isvcRepo.Save(ctx, isvc)
}

func (s *deployService) resolveVersion(ctx context.Context, projectID, modelID uuid.UUID, versionID *uuid.UUID) (*domain.ModelVersion, error) {
	if versionID != nil {
		return s.versionRepo.FindByID(ctx, projectID, *versionID)
	}
	// Get default version
	return s.versionRepo.FindDefaultByModel(ctx, projectID, modelID)
}

// Ensure interface compliance
var _ input.DeployService = (*deployService)(nil)
```

---

## Phase 4: Adapters

### 4.1 Primary Adapter (HTTP)

**File**: `internal/adapters/primary/http/handlers/deploy_handler.go`

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"model-registry-service/internal/adapters/primary/http/dto"
	"model-registry-service/internal/core/ports/input"
)

type DeployHandler struct {
	deployService input.DeployService
}

func NewDeployHandler(svc input.DeployService) *DeployHandler {
	return &DeployHandler{deployService: svc}
}

func (h *DeployHandler) Deploy(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		return
	}

	var req dto.DeployModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	result, err := h.deployService.Deploy(c.Request.Context(), input.DeployRequest{
		ProjectID:            projectID,
		RegisteredModelID:    req.RegisteredModelID,
		ModelVersionID:       req.ModelVersionID,
		ServingEnvironmentID: req.ServingEnvironmentID,
		Name:                 req.Name,
	})
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, dto.ToDeployResponse(result))
}

func (h *DeployHandler) Undeploy(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		return
	}

	isvcID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id"})
		return
	}

	if err := h.deployService.Undeploy(c.Request.Context(), projectID, isvcID); err != nil {
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
```

### 4.2 Secondary Adapter (PostgreSQL)

**File**: `internal/adapters/secondary/postgres/inference_service_repository.go`

```go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"
)

type inferenceServiceRepository struct {
	pool *pgxpool.Pool
}

func NewInferenceServiceRepository(pool *pgxpool.Pool) output.InferenceServiceRepository {
	return &inferenceServiceRepository{pool: pool}
}

func (r *inferenceServiceRepository) Save(ctx context.Context, isvc *domain.InferenceService) error {
	labelsJSON, _ := json.Marshal(isvc.Labels)

	query := `
		INSERT INTO inference_service
			(id, project_id, name, external_id, serving_environment_id,
			 registered_model_id, model_version_id, desired_state, current_state,
			 runtime, url, last_error, labels, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			external_id = EXCLUDED.external_id,
			model_version_id = EXCLUDED.model_version_id,
			desired_state = EXCLUDED.desired_state,
			current_state = EXCLUDED.current_state,
			runtime = EXCLUDED.runtime,
			url = EXCLUDED.url,
			last_error = EXCLUDED.last_error,
			labels = EXCLUDED.labels,
			updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query,
		isvc.ID, isvc.ProjectID, isvc.Name, isvc.ExternalID,
		isvc.ServingEnvironmentID, isvc.RegisteredModelID, isvc.ModelVersionID,
		string(isvc.DesiredState), string(isvc.CurrentState),
		isvc.Runtime, isvc.URL, isvc.LastError, labelsJSON,
		isvc.CreatedAt, isvc.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrInferenceServiceNameConflict
		}
		return fmt.Errorf("save inference service: %w", err)
	}
	return nil
}

// ... other methods follow same pattern
```

### 4.3 Secondary Adapter (KServe)

**File**: `internal/adapters/secondary/kserve/client.go`

```go
package kserve

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"
)

var inferenceServiceGVR = schema.GroupVersionResource{
	Group:    "serving.kserve.io",
	Version:  "v1beta1",
	Resource: "inferenceservices",
}

type kserveClient struct {
	client    dynamic.Interface
	enabled   bool
	defaultNS string
}

func NewKServeClient(kubeconfig string, inCluster bool, enabled bool, defaultNS string) (output.KServeClient, error) {
	if !enabled {
		return &kserveClient{enabled: false}, nil
	}

	var cfg *rest.Config
	var err error

	if inCluster {
		cfg, err = rest.InClusterConfig()
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("build k8s config: %w", err)
	}

	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	return &kserveClient{
		client:    client,
		enabled:   true,
		defaultNS: defaultNS,
	}, nil
}

func (c *kserveClient) IsAvailable() bool {
	return c.enabled
}

func (c *kserveClient) Deploy(ctx context.Context, namespace string, isvc *domain.InferenceService, version *domain.ModelVersion) (*output.KServeDeployment, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj := c.buildInferenceServiceCR(isvc, version)

	created, err := c.client.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create kserve inferenceservice: %w", err)
	}

	return &output.KServeDeployment{
		ExternalID: string(created.GetUID()),
	}, nil
}

func (c *kserveClient) Undeploy(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		namespace = c.defaultNS
	}
	return c.client.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *kserveClient) GetStatus(ctx context.Context, namespace, name string) (string, bool, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj, err := c.client.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", false, err
	}

	return c.parseStatus(obj)
}

func (c *kserveClient) buildInferenceServiceCR(isvc *domain.InferenceService, version *domain.ModelVersion) *unstructured.Unstructured {
	labels := map[string]interface{}{
		"modelregistry.ai-platform/inference-service-id": isvc.ID.String(),
		"modelregistry.ai-platform/registered-model-id":  isvc.RegisteredModelID.String(),
	}
	if isvc.ModelVersionID != nil {
		labels["modelregistry.ai-platform/model-version-id"] = isvc.ModelVersionID.String()
	}

	return &unstructured.Unstructured{
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
}

func (c *kserveClient) parseStatus(obj *unstructured.Unstructured) (string, bool, error) {
	status, found, _ := unstructured.NestedMap(obj.Object, "status")
	if !found {
		return "", false, nil
	}

	url, _, _ := unstructured.NestedString(status, "url")

	conditions, found, _ := unstructured.NestedSlice(status, "conditions")
	if found {
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if ok && condMap["type"] == "Ready" && condMap["status"] == "True" {
				return url, true, nil
			}
		}
	}

	return url, false, nil
}

var _ output.KServeClient = (*kserveClient)(nil)
```

---

## Phase 5: Wiring (Dependency Injection)

**File**: `cmd/server/main.go`

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/handlers"
	"model-registry-service/internal/adapters/primary/http/middleware"
	"model-registry-service/internal/adapters/secondary/kserve"
	"model-registry-service/internal/adapters/secondary/postgres"
	"model-registry-service/internal/config"
	"model-registry-service/internal/core/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// === DATABASE ===
	pool, err := createDBPool(cfg)
	if err != nil {
		log.Fatalf("create db pool: %v", err)
	}
	defer pool.Close()

	// === SECONDARY ADAPTERS (Output Ports) ===
	modelRepo := postgres.NewModelRepository(pool)
	versionRepo := postgres.NewModelVersionRepository(pool)
	envRepo := postgres.NewServingEnvironmentRepository(pool)
	isvcRepo := postgres.NewInferenceServiceRepository(pool)
	serveModelRepo := postgres.NewServeModelRepository(pool)

	kserveClient, err := kserve.NewKServeClient(
		cfg.Kubernetes.KubeConfigPath,
		cfg.Kubernetes.InCluster,
		cfg.Kubernetes.Enabled,
		cfg.Kubernetes.DefaultNS,
	)
	if err != nil {
		log.Warnf("KServe client init failed: %v", err)
	}

	// === CORE SERVICES (Input Ports) ===
	modelService := services.NewModelService(modelRepo, versionRepo, isvcRepo)
	versionService := services.NewVersionService(versionRepo, modelRepo)
	envService := services.NewServingEnvironmentService(envRepo)
	isvcService := services.NewInferenceServiceService(isvcRepo, envRepo, modelRepo, versionRepo)
	deployService := services.NewDeployService(envRepo, isvcRepo, modelRepo, versionRepo, kserveClient)

	// === PRIMARY ADAPTERS (HTTP Handlers) ===
	modelHandler := handlers.NewModelHandler(modelService)
	versionHandler := handlers.NewVersionHandler(versionService)
	envHandler := handlers.NewServingEnvironmentHandler(envService)
	isvcHandler := handlers.NewInferenceServiceHandler(isvcService)
	deployHandler := handlers.NewDeployHandler(deployService)

	// === ROUTER ===
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Logging(), gin.Recovery())

	api := router.Group("/api/v1/model-registry")
	{
		// Models
		api.GET("/models", modelHandler.List)
		api.POST("/models", modelHandler.Create)
		api.GET("/models/:id", modelHandler.Get)
		api.PATCH("/models/:id", modelHandler.Update)
		api.DELETE("/models/:id", modelHandler.Delete)

		// Versions
		api.GET("/models/:id/versions", versionHandler.ListByModel)
		api.POST("/models/:id/versions", versionHandler.Create)

		// Serving Environments
		api.GET("/serving_environments", envHandler.List)
		api.POST("/serving_environments", envHandler.Create)
		api.GET("/serving_environments/:id", envHandler.Get)
		api.PATCH("/serving_environments/:id", envHandler.Update)
		api.DELETE("/serving_environments/:id", envHandler.Delete)

		// Inference Services
		api.GET("/inference_services", isvcHandler.List)
		api.POST("/inference_services", isvcHandler.Create)
		api.GET("/inference_services/:id", isvcHandler.Get)
		api.PATCH("/inference_services/:id", isvcHandler.Update)
		api.DELETE("/inference_services/:id", isvcHandler.Delete)

		// Deploy Actions
		api.POST("/deploy", deployHandler.Deploy)
		api.DELETE("/inference_services/:id/undeploy", deployHandler.Undeploy)
	}

	// Health check
	router.GET("/healthz", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Start server with graceful shutdown
	startServer(router, cfg)
}

func createDBPool(cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}
	log.Info("Database connected")
	return pool, nil
}

func startServer(router *gin.Engine, cfg *config.Config) {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Infof("Server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Info("Server stopped")
}
```

---

## Summary: Hexagonal vs Current

| Aspect | Current | Hexagonal |
|--------|---------|-----------|
| Domain deps | Has external deps | Zero external deps |
| Testability | Mock repos only | Mock any port |
| Swap DB | Hard | Easy (implement port) |
| Swap KServe | Hard | Easy (implement port) |
| Business rules | Mixed in usecase | Pure domain |
| Layer coupling | Tight | Loose via interfaces |

---

## Implementation Order

### Phase 1: Restructure Existing Code (2 days)
- Move files to Hexagonal layout (no logic changes)
- Update imports
- Verify all tests pass
- **Zero API changes**

### Phase 2: Add Serving Domain (1 day)
- `internal/core/domain/serving.go` - new entities
- `internal/core/ports/output/serving_repository.go` - interfaces
- `internal/core/domain/errors.go` - extend with serving errors

### Phase 3: PostgreSQL Serving Adapter (1.5 days)
- Migrations for `serving_environment`, `inference_service`, `serve_model`
- Repository implementations

### Phase 4: Serving Services + HTTP (1.5 days)
- `internal/core/services/serving_service.go`
- `internal/adapters/primary/http/handlers/serving_handler.go`
- DTOs and mappers

### Phase 5: KServe Adapter (1.5 days)
- K8s dynamic client
- CR builder + status parser
- Deploy service integration

### Phase 6: Wiring + E2E Testing (1 day)
- Update `cmd/server/main.go`
- E2E tests for new endpoints

**Total**: ~8 days

---

## Resolved Questions

1. ~~Should we refactor existing code to Hexagonal first, or build new features in Hexagonal and migrate later?~~
   → **Gradual migration**: Move existing code to new structure, add new features in Hexagonal

2. ~~Do you want event-driven architecture (domain events) for async operations?~~
   → **No** - Keep it simple for now, can add later if needed

3. ~~Should we add a Unit of Work pattern for transactional consistency?~~
   → **No** - PostgreSQL transactions in repository layer sufficient

4. ~~Change the API?~~
   → **No** - All 19 existing routes preserved, only internal restructure + new serving routes

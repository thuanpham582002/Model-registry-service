# Phase 3: API Layer (DTOs, Usecases, Handlers)

## Objective
Implement REST API endpoints for serving entities.

## Tasks

### 3.1 DTOs

**File**: `internal/dto/serving.go`

```go
package dto

import (
	"time"
	"github.com/google/uuid"
	"model-registry-service/internal/domain"
)

// --- Serving Environment ---

type CreateServingEnvironmentRequest struct {
	Name        string `json:"name" binding:"required,max=255"`
	Description string `json:"description"`
	ExternalID  string `json:"external_id"`
}

type UpdateServingEnvironmentRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ExternalID  *string `json:"external_id"`
}

type ServingEnvironmentResponse struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ExternalID  string    `json:"external_id,omitempty"`
}

func ToServingEnvironmentResponse(env *domain.ServingEnvironment) ServingEnvironmentResponse {
	return ServingEnvironmentResponse{
		ID:          env.ID,
		CreatedAt:   env.CreatedAt,
		UpdatedAt:   env.UpdatedAt,
		Name:        env.Name,
		Description: env.Description,
		ExternalID:  env.ExternalID,
	}
}

// --- Inference Service ---

type CreateInferenceServiceRequest struct {
	Name                 string            `json:"name" binding:"required"`
	ServingEnvironmentID uuid.UUID         `json:"serving_environment_id" binding:"required"`
	RegisteredModelID    uuid.UUID         `json:"registered_model_id" binding:"required"`
	ModelVersionID       *uuid.UUID        `json:"model_version_id"`
	Runtime              string            `json:"runtime"`
	Labels               map[string]string `json:"labels"`
}

type UpdateInferenceServiceRequest struct {
	Name           *string           `json:"name"`
	DesiredState   *string           `json:"desired_state"`
	ModelVersionID *uuid.UUID        `json:"model_version_id"`
	URL            *string           `json:"url"`
	CurrentState   *string           `json:"current_state"`
	LastError      *string           `json:"last_error"`
	Labels         map[string]string `json:"labels"`
}

type InferenceServiceResponse struct {
	ID                   uuid.UUID         `json:"id"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
	Name                 string            `json:"name"`
	ExternalID           string            `json:"external_id,omitempty"`
	ServingEnvironmentID uuid.UUID         `json:"serving_environment_id"`
	RegisteredModelID    uuid.UUID         `json:"registered_model_id"`
	ModelVersionID       *uuid.UUID        `json:"model_version_id,omitempty"`
	DesiredState         string            `json:"desired_state"`
	CurrentState         string            `json:"current_state"`
	Runtime              string            `json:"runtime"`
	URL                  string            `json:"url,omitempty"`
	LastError            string            `json:"last_error,omitempty"`
	Labels               map[string]string `json:"labels"`
}

func ToInferenceServiceResponse(isvc *domain.InferenceService) InferenceServiceResponse {
	return InferenceServiceResponse{
		ID:                   isvc.ID,
		CreatedAt:            isvc.CreatedAt,
		UpdatedAt:            isvc.UpdatedAt,
		Name:                 isvc.Name,
		ExternalID:           isvc.ExternalID,
		ServingEnvironmentID: isvc.ServingEnvironmentID,
		RegisteredModelID:    isvc.RegisteredModelID,
		ModelVersionID:       isvc.ModelVersionID,
		DesiredState:         string(isvc.DesiredState),
		CurrentState:         string(isvc.CurrentState),
		Runtime:              isvc.Runtime,
		URL:                  isvc.URL,
		LastError:            isvc.LastError,
		Labels:               isvc.Labels,
	}
}

// --- Serve Model ---

type CreateServeModelRequest struct {
	ModelVersionID uuid.UUID `json:"model_version_id" binding:"required"`
}

type ServeModelResponse struct {
	ID                 uuid.UUID `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	InferenceServiceID uuid.UUID `json:"inference_service_id"`
	ModelVersionID     uuid.UUID `json:"model_version_id"`
	LastKnownState     string    `json:"last_known_state"`
}

func ToServeModelResponse(sm *domain.ServeModel) ServeModelResponse {
	return ServeModelResponse{
		ID:                 sm.ID,
		CreatedAt:          sm.CreatedAt,
		UpdatedAt:          sm.UpdatedAt,
		InferenceServiceID: sm.InferenceServiceID,
		ModelVersionID:     sm.ModelVersionID,
		LastKnownState:     string(sm.LastKnownState),
	}
}
```

### 3.2 Usecases

**File**: `internal/usecase/serving-environment.go`

```go
package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"model-registry-service/internal/domain"
	"model-registry-service/internal/dto"
)

type ServingEnvironmentUseCase struct {
	repo domain.ServingEnvironmentRepository
}

func NewServingEnvironmentUseCase(repo domain.ServingEnvironmentRepository) *ServingEnvironmentUseCase {
	return &ServingEnvironmentUseCase{repo: repo}
}

func (uc *ServingEnvironmentUseCase) Create(ctx context.Context, projectID uuid.UUID, req dto.CreateServingEnvironmentRequest) (*domain.ServingEnvironment, error) {
	env := &domain.ServingEnvironment{
		ID:          uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		ExternalID:  req.ExternalID,
	}
	if err := uc.repo.Create(ctx, env); err != nil {
		return nil, err
	}
	return env, nil
}

func (uc *ServingEnvironmentUseCase) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServingEnvironment, error) {
	return uc.repo.GetByID(ctx, projectID, id)
}

func (uc *ServingEnvironmentUseCase) GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.ServingEnvironment, error) {
	return uc.repo.GetByName(ctx, projectID, name)
}

func (uc *ServingEnvironmentUseCase) Update(ctx context.Context, projectID, id uuid.UUID, req dto.UpdateServingEnvironmentRequest) (*domain.ServingEnvironment, error) {
	env, err := uc.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		env.Name = *req.Name
	}
	if req.Description != nil {
		env.Description = *req.Description
	}
	if req.ExternalID != nil {
		env.ExternalID = *req.ExternalID
	}

	if err := uc.repo.Update(ctx, projectID, env); err != nil {
		return nil, err
	}
	return env, nil
}

func (uc *ServingEnvironmentUseCase) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	return uc.repo.Delete(ctx, projectID, id)
}

func (uc *ServingEnvironmentUseCase) List(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*domain.ServingEnvironment, int, error) {
	return uc.repo.List(ctx, projectID, limit, offset)
}
```

**File**: `internal/usecase/inference-service.go`

```go
package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"model-registry-service/internal/domain"
	"model-registry-service/internal/dto"
)

type InferenceServiceUseCase struct {
	repo        domain.InferenceServiceRepository
	envRepo     domain.ServingEnvironmentRepository
	modelRepo   domain.RegisteredModelRepository
	versionRepo domain.ModelVersionRepository
}

func NewInferenceServiceUseCase(
	repo domain.InferenceServiceRepository,
	envRepo domain.ServingEnvironmentRepository,
	modelRepo domain.RegisteredModelRepository,
	versionRepo domain.ModelVersionRepository,
) *InferenceServiceUseCase {
	return &InferenceServiceUseCase{
		repo:        repo,
		envRepo:     envRepo,
		modelRepo:   modelRepo,
		versionRepo: versionRepo,
	}
}

func (uc *InferenceServiceUseCase) Create(ctx context.Context, projectID uuid.UUID, req dto.CreateInferenceServiceRequest) (*domain.InferenceService, error) {
	// Validate serving environment exists
	if _, err := uc.envRepo.GetByID(ctx, projectID, req.ServingEnvironmentID); err != nil {
		return nil, err
	}

	// Validate model exists
	if _, err := uc.modelRepo.GetByID(ctx, projectID, req.RegisteredModelID); err != nil {
		return nil, err
	}

	// Validate version if provided
	if req.ModelVersionID != nil {
		if _, err := uc.versionRepo.GetByID(ctx, projectID, *req.ModelVersionID); err != nil {
			return nil, err
		}
	}

	runtime := req.Runtime
	if runtime == "" {
		runtime = "kserve"
	}

	isvc := &domain.InferenceService{
		ID:                   uuid.New(),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		ProjectID:            projectID,
		Name:                 req.Name,
		ServingEnvironmentID: req.ServingEnvironmentID,
		RegisteredModelID:    req.RegisteredModelID,
		ModelVersionID:       req.ModelVersionID,
		DesiredState:         domain.ISStateDeployed,
		CurrentState:         domain.ISStateUndeployed,
		Runtime:              runtime,
		Labels:               req.Labels,
	}

	if err := uc.repo.Create(ctx, isvc); err != nil {
		return nil, err
	}
	return isvc, nil
}

func (uc *InferenceServiceUseCase) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.InferenceService, error) {
	return uc.repo.GetByID(ctx, projectID, id)
}

func (uc *InferenceServiceUseCase) Update(ctx context.Context, projectID, id uuid.UUID, req dto.UpdateInferenceServiceRequest) (*domain.InferenceService, error) {
	isvc, err := uc.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		isvc.Name = *req.Name
	}
	if req.DesiredState != nil {
		isvc.DesiredState = domain.InferenceServiceState(*req.DesiredState)
	}
	if req.CurrentState != nil {
		isvc.CurrentState = domain.InferenceServiceState(*req.CurrentState)
	}
	if req.ModelVersionID != nil {
		isvc.ModelVersionID = req.ModelVersionID
	}
	if req.URL != nil {
		isvc.URL = *req.URL
	}
	if req.LastError != nil {
		isvc.LastError = *req.LastError
	}
	if req.Labels != nil {
		isvc.Labels = req.Labels
	}

	if err := uc.repo.Update(ctx, projectID, isvc); err != nil {
		return nil, err
	}
	return isvc, nil
}

func (uc *InferenceServiceUseCase) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	return uc.repo.Delete(ctx, projectID, id)
}

func (uc *InferenceServiceUseCase) List(ctx context.Context, filter domain.InferenceServiceFilter) ([]*domain.InferenceService, int, error) {
	return uc.repo.List(ctx, filter)
}

func (uc *InferenceServiceUseCase) CountDeploymentsByModel(ctx context.Context, projectID, modelID uuid.UUID) (int, error) {
	return uc.repo.CountByModel(ctx, projectID, modelID)
}
```

### 3.3 Handlers

**File**: `internal/handler/serving-environment.go`

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"model-registry-service/internal/dto"
)

func (h *Handler) ListServingEnvironments(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
		return
	}

	limit, offset := h.getPagination(c)
	envs, total, err := h.servingEnvUC.List(c.Request.Context(), projectID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]dto.ServingEnvironmentResponse, len(envs))
	for i, env := range envs {
		items[i] = dto.ToServingEnvironmentResponse(env)
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func (h *Handler) GetServingEnvironment(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	env, err := h.servingEnvUC.GetByID(c.Request.Context(), projectID, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) FindServingEnvironment(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
		return
	}

	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name query param required"})
		return
	}

	env, err := h.servingEnvUC.GetByName(c.Request.Context(), projectID, name)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) CreateServingEnvironment(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
		return
	}

	var req dto.CreateServingEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := h.servingEnvUC.Create(c.Request.Context(), projectID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) UpdateServingEnvironment(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
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

	env, err := h.servingEnvUC.Update(c.Request.Context(), projectID, id, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToServingEnvironmentResponse(env))
}

func (h *Handler) DeleteServingEnvironment(c *gin.Context) {
	projectID, err := h.getProjectID(c)
	if err != nil {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.servingEnvUC.Delete(c.Request.Context(), projectID, id); err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
```

**File**: `internal/handler/inference-service.go` (similar pattern)

### 3.4 Update Handler Struct

**File**: `internal/handler/handler.go` (modify)

```go
type Handler struct {
	modelUC       *usecase.RegisteredModelUseCase
	versionUC     *usecase.ModelVersionUseCase
	artifactUC    *usecase.ModelArtifactUseCase
	servingEnvUC  *usecase.ServingEnvironmentUseCase  // NEW
	isvcUC        *usecase.InferenceServiceUseCase    // NEW
}

func New(
	modelUC *usecase.RegisteredModelUseCase,
	versionUC *usecase.ModelVersionUseCase,
	artifactUC *usecase.ModelArtifactUseCase,
	servingEnvUC *usecase.ServingEnvironmentUseCase,
	isvcUC *usecase.InferenceServiceUseCase,
) *Handler {
	return &Handler{
		modelUC:       modelUC,
		versionUC:     versionUC,
		artifactUC:    artifactUC,
		servingEnvUC:  servingEnvUC,
		isvcUC:        isvcUC,
	}
}
```

### 3.5 Update Error Mapper

**File**: `internal/handler/error-mapper.go` (append)

```go
func init() {
	errorMap[domain.ErrServingEnvNotFound] = http.StatusNotFound
	errorMap[domain.ErrServingEnvNameConflict] = http.StatusConflict
	errorMap[domain.ErrInferenceServiceNotFound] = http.StatusNotFound
	errorMap[domain.ErrInferenceServiceNameConflict] = http.StatusConflict
}
```

## Checklist

- [ ] Create `internal/dto/serving.go`
- [ ] Create `internal/usecase/serving-environment.go`
- [ ] Create `internal/usecase/inference-service.go`
- [ ] Create `internal/handler/serving-environment.go`
- [ ] Create `internal/handler/inference-service.go`
- [ ] Update `internal/handler/handler.go` with new usecases
- [ ] Add routes in `RegisterRoutes()`
- [ ] Update error mapper
- [ ] Wire dependencies in `cmd/server/main.go`
- [ ] Write handler tests

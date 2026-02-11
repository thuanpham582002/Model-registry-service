# Phase 3: Traffic Service

## Objective

Implement traffic management service for canary deployments, A/B testing, and traffic splitting with promotion/rollback capabilities.

---

## 3.1 Repository Interfaces

**File**: `internal/core/ports/output/traffic_repository.go`

```go
package ports

import (
	"context"
	"github.com/google/uuid"
	"model-registry-service/internal/core/domain"
)

// TrafficConfigRepository defines traffic config persistence
type TrafficConfigRepository interface {
	Create(ctx context.Context, config *domain.TrafficConfig) error
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.TrafficConfig, error)
	GetByISVC(ctx context.Context, projectID, isvcID uuid.UUID) (*domain.TrafficConfig, error)
	Update(ctx context.Context, projectID uuid.UUID, config *domain.TrafficConfig) error
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	List(ctx context.Context, filter TrafficConfigFilter) ([]*domain.TrafficConfig, int, error)
}

type TrafficConfigFilter struct {
	ProjectID           uuid.UUID
	InferenceServiceID  *uuid.UUID
	Strategy            string
	Status              string
	Limit               int
	Offset              int
}

// TrafficVariantRepository defines traffic variant persistence
type TrafficVariantRepository interface {
	Create(ctx context.Context, variant *domain.TrafficVariant) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.TrafficVariant, error)
	GetByName(ctx context.Context, configID uuid.UUID, name string) (*domain.TrafficVariant, error)
	Update(ctx context.Context, variant *domain.TrafficVariant) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByConfig(ctx context.Context, configID uuid.UUID) ([]*domain.TrafficVariant, error)
	DeleteByConfig(ctx context.Context, configID uuid.UUID) error
}
```

---

## 3.2 Traffic Service

**File**: `internal/core/services/traffic.go`

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type TrafficService struct {
	configRepo   output.TrafficConfigRepository
	variantRepo  output.TrafficVariantRepository
	isvcRepo     output.InferenceServiceRepository
	versionRepo  output.ModelVersionRepository
	envRepo      output.ServingEnvironmentRepository
	kserve       output.KServeClient
	aiGateway    output.AIGatewayClient
}

func NewTrafficService(
	configRepo output.TrafficConfigRepository,
	variantRepo output.TrafficVariantRepository,
	isvcRepo output.InferenceServiceRepository,
	versionRepo output.ModelVersionRepository,
	envRepo output.ServingEnvironmentRepository,
	kserve output.KServeClient,
	aiGateway output.AIGatewayClient,
) *TrafficService {
	return &TrafficService{
		configRepo:  configRepo,
		variantRepo: variantRepo,
		isvcRepo:    isvcRepo,
		versionRepo: versionRepo,
		envRepo:     envRepo,
		kserve:      kserve,
		aiGateway:   aiGateway,
	}
}

// ============================================================================
// Traffic Config CRUD
// ============================================================================

type CreateTrafficConfigRequest struct {
	ProjectID          uuid.UUID
	InferenceServiceID uuid.UUID
	Strategy           domain.TrafficStrategy
	StableVersionID    uuid.UUID
}

func (s *TrafficService) CreateConfig(ctx context.Context, req CreateTrafficConfigRequest) (*domain.TrafficConfig, error) {
	// Validate ISVC exists
	isvc, err := s.isvcRepo.GetByID(ctx, req.ProjectID, req.InferenceServiceID)
	if err != nil {
		return nil, err
	}

	// Validate version exists
	version, err := s.versionRepo.GetByID(ctx, req.ProjectID, req.StableVersionID)
	if err != nil {
		return nil, err
	}

	// Create config
	config, err := domain.NewTrafficConfig(req.ProjectID, req.InferenceServiceID, req.Strategy)
	if err != nil {
		return nil, err
	}

	if err := s.configRepo.Create(ctx, config); err != nil {
		return nil, err
	}

	// Create stable variant (100% traffic)
	stable, err := domain.NewTrafficVariant(config.ID, req.StableVersionID, "stable", 100)
	if err != nil {
		return nil, err
	}
	stable.KServeISVCName = isvc.Name
	stable.Activate()

	if err := s.variantRepo.Create(ctx, stable); err != nil {
		return nil, err
	}

	// Create AI Gateway route
	if s.aiGateway != nil && s.aiGateway.IsAvailable() {
		env, _ := s.envRepo.GetByID(ctx, req.ProjectID, isvc.ServingEnvironmentID)
		namespace := env.Name
		if env.ExternalID != "" {
			namespace = env.ExternalID
		}

		route := &output.AIGatewayRoute{
			Name:      fmt.Sprintf("traffic-%s", config.ID.String()[:8]),
			Namespace: namespace,
			ModelName: isvc.Name,
			Backends: []output.WeightedBackend{
				{Name: isvc.Name, Weight: 100, VariantTag: "stable"},
			},
			Labels: map[string]string{
				"traffic-config-id": config.ID.String(),
				"model-name":        isvc.RegisteredModelName,
			},
		}

		if err := s.aiGateway.CreateRoute(ctx, route); err != nil {
			// Log but don't fail
		} else {
			config.AIGatewayRouteName = route.Name
			s.configRepo.Update(ctx, req.ProjectID, config)
		}
	}

	return s.configRepo.GetByID(ctx, req.ProjectID, config.ID)
}

func (s *TrafficService) GetConfig(ctx context.Context, projectID, id uuid.UUID) (*domain.TrafficConfig, error) {
	config, err := s.configRepo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	// Load variants
	variants, err := s.variantRepo.ListByConfig(ctx, config.ID)
	if err != nil {
		return nil, err
	}
	config.Variants = variants

	return config, nil
}

func (s *TrafficService) ListConfigs(ctx context.Context, filter output.TrafficConfigFilter) ([]*domain.TrafficConfig, int, error) {
	return s.configRepo.List(ctx, filter)
}

func (s *TrafficService) DeleteConfig(ctx context.Context, projectID, id uuid.UUID) error {
	config, err := s.configRepo.GetByID(ctx, projectID, id)
	if err != nil {
		return err
	}

	// Delete AI Gateway route
	if config.AIGatewayRouteName != "" && s.aiGateway != nil && s.aiGateway.IsAvailable() {
		isvc, _ := s.isvcRepo.GetByID(ctx, projectID, config.InferenceServiceID)
		if isvc != nil {
			env, _ := s.envRepo.GetByID(ctx, projectID, isvc.ServingEnvironmentID)
			namespace := env.Name
			if env.ExternalID != "" {
				namespace = env.ExternalID
			}
			s.aiGateway.DeleteRoute(ctx, namespace, config.AIGatewayRouteName)
		}
	}

	// Delete variants (cascade)
	if err := s.variantRepo.DeleteByConfig(ctx, id); err != nil {
		return err
	}

	return s.configRepo.Delete(ctx, projectID, id)
}

// ============================================================================
// Canary Operations
// ============================================================================

type StartCanaryRequest struct {
	ProjectID      uuid.UUID
	ConfigID       uuid.UUID
	ModelVersionID uuid.UUID
	InitialWeight  int // 5-50, default 10
}

func (s *TrafficService) StartCanary(ctx context.Context, req StartCanaryRequest) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, req.ProjectID, req.ConfigID)
	if err != nil {
		return nil, err
	}

	// Check no existing canary
	for _, v := range config.Variants {
		if v.VariantName == "canary" && v.Status == domain.VariantStatusActive {
			return nil, domain.ErrCanaryAlreadyExists
		}
	}

	// Validate version
	version, err := s.versionRepo.GetByID(ctx, req.ProjectID, req.ModelVersionID)
	if err != nil {
		return nil, err
	}

	// Get ISVC for deployment
	isvc, err := s.isvcRepo.GetByID(ctx, req.ProjectID, config.InferenceServiceID)
	if err != nil {
		return nil, err
	}

	// Set initial weight
	weight := req.InitialWeight
	if weight <= 0 {
		weight = 10
	}
	if weight > 50 {
		weight = 50
	}

	// Create canary KServe ISVC
	canaryISVCName := fmt.Sprintf("%s-canary", isvc.Name)
	env, _ := s.envRepo.GetByID(ctx, req.ProjectID, isvc.ServingEnvironmentID)

	if s.kserve != nil && s.kserve.IsAvailable() {
		canaryISVC := &domain.InferenceService{
			ID:                   uuid.New(),
			Name:                 canaryISVCName,
			ProjectID:            req.ProjectID,
			ServingEnvironmentID: isvc.ServingEnvironmentID,
			RegisteredModelID:    isvc.RegisteredModelID,
			ModelVersionID:       &req.ModelVersionID,
			DesiredState:         domain.ISStateDeployed,
			CurrentState:         domain.ISStateUndeployed,
			Runtime:              isvc.Runtime,
		}

		namespace := env.Name
		if env.ExternalID != "" {
			namespace = env.ExternalID
		}

		_, err := s.kserve.Deploy(ctx, namespace, canaryISVC, version)
		if err != nil {
			return nil, fmt.Errorf("deploy canary: %w", err)
		}
	}

	// Create canary variant
	canary, err := domain.NewTrafficVariant(config.ID, req.ModelVersionID, "canary", weight)
	if err != nil {
		return nil, err
	}
	canary.KServeISVCName = canaryISVCName
	canary.Activate()

	if err := s.variantRepo.Create(ctx, canary); err != nil {
		return nil, err
	}

	// Update stable weight
	for _, v := range config.Variants {
		if v.VariantName == "stable" {
			v.SetWeight(100 - weight)
			s.variantRepo.Update(ctx, v)
			break
		}
	}

	// Update AI Gateway route
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, req.ProjectID, req.ConfigID)
}

func (s *TrafficService) UpdateCanaryWeight(ctx context.Context, projectID, configID uuid.UUID, weight int) (*domain.TrafficConfig, error) {
	if weight < 0 || weight > 100 {
		return nil, domain.ErrInvalidTrafficWeight
	}

	config, err := s.GetConfig(ctx, projectID, configID)
	if err != nil {
		return nil, err
	}

	var canary, stable *domain.TrafficVariant
	for _, v := range config.Variants {
		if v.VariantName == "canary" {
			canary = v
		}
		if v.VariantName == "stable" {
			stable = v
		}
	}

	if canary == nil {
		return nil, domain.ErrTrafficVariantNotFound
	}

	// Update weights
	canary.SetWeight(weight)
	stable.SetWeight(100 - weight)

	if err := s.variantRepo.Update(ctx, canary); err != nil {
		return nil, err
	}
	if err := s.variantRepo.Update(ctx, stable); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, projectID, configID)
}

func (s *TrafficService) PromoteCanary(ctx context.Context, projectID, configID uuid.UUID) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, projectID, configID)
	if err != nil {
		return nil, err
	}

	var canary, stable *domain.TrafficVariant
	for _, v := range config.Variants {
		if v.VariantName == "canary" {
			canary = v
		}
		if v.VariantName == "stable" {
			stable = v
		}
	}

	if canary == nil || canary.Status != domain.VariantStatusActive {
		return nil, domain.ErrCannotPromoteInactive
	}

	// Swap: canary becomes stable, old stable becomes inactive
	canary.VariantName = "stable"
	canary.SetWeight(100)
	canary.UpdatedAt = time.Now()

	stable.VariantName = "old-stable"
	stable.Deactivate()

	if err := s.variantRepo.Update(ctx, canary); err != nil {
		return nil, err
	}
	if err := s.variantRepo.Update(ctx, stable); err != nil {
		return nil, err
	}

	// Delete old stable ISVC from K8s (async/background)
	// TODO: implement cleanup job

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, projectID, configID)
}

func (s *TrafficService) Rollback(ctx context.Context, projectID, configID uuid.UUID) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, projectID, configID)
	if err != nil {
		return nil, err
	}

	var canary, stable *domain.TrafficVariant
	for _, v := range config.Variants {
		if v.VariantName == "canary" {
			canary = v
		}
		if v.VariantName == "stable" {
			stable = v
		}
	}

	if canary != nil {
		// Deactivate canary
		canary.Deactivate()
		s.variantRepo.Update(ctx, canary)

		// Delete canary ISVC from K8s
		if s.kserve != nil && s.kserve.IsAvailable() && canary.KServeISVCName != "" {
			isvc, _ := s.isvcRepo.GetByID(ctx, projectID, config.InferenceServiceID)
			if isvc != nil {
				env, _ := s.envRepo.GetByID(ctx, projectID, isvc.ServingEnvironmentID)
				namespace := env.Name
				if env.ExternalID != "" {
					namespace = env.ExternalID
				}
				s.kserve.Undeploy(ctx, namespace, canary.KServeISVCName)
			}
		}
	}

	// Set stable to 100%
	if stable != nil {
		stable.SetWeight(100)
		s.variantRepo.Update(ctx, stable)
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, projectID, configID)
}

// ============================================================================
// Multi-Variant Operations (A/B Testing, Blue/Green, etc.)
// ============================================================================

type AddVariantRequest struct {
	ProjectID      uuid.UUID
	ConfigID       uuid.UUID
	VariantName    string    // e.g., "variant_a", "variant_b", "shadow"
	ModelVersionID uuid.UUID
	Weight         int       // 0-100, default 0
}

func (s *TrafficService) AddVariant(ctx context.Context, req AddVariantRequest) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, req.ProjectID, req.ConfigID)
	if err != nil {
		return nil, err
	}

	// Check variant doesn't already exist
	if config.HasVariant(req.VariantName) {
		return nil, domain.ErrVariantAlreadyExists
	}

	// Validate version
	version, err := s.versionRepo.GetByID(ctx, req.ProjectID, req.ModelVersionID)
	if err != nil {
		return nil, err
	}

	// Validate total weight won't exceed 100
	if config.TotalWeight()+req.Weight > 100 {
		return nil, domain.ErrWeightSumExceeds100
	}

	// Get ISVC for deployment
	isvc, err := s.isvcRepo.GetByID(ctx, req.ProjectID, config.InferenceServiceID)
	if err != nil {
		return nil, err
	}

	// Create variant KServe ISVC
	variantISVCName := fmt.Sprintf("%s-%s", isvc.Name, req.VariantName)
	env, _ := s.envRepo.GetByID(ctx, req.ProjectID, isvc.ServingEnvironmentID)

	if s.kserve != nil && s.kserve.IsAvailable() {
		variantISVC := &domain.InferenceService{
			ID:                   uuid.New(),
			Name:                 variantISVCName,
			ProjectID:            req.ProjectID,
			ServingEnvironmentID: isvc.ServingEnvironmentID,
			RegisteredModelID:    isvc.RegisteredModelID,
			ModelVersionID:       &req.ModelVersionID,
			DesiredState:         domain.ISStateDeployed,
			CurrentState:         domain.ISStateUndeployed,
			Runtime:              isvc.Runtime,
		}

		namespace := env.Name
		if env.ExternalID != "" {
			namespace = env.ExternalID
		}

		_, err := s.kserve.Deploy(ctx, namespace, variantISVC, version)
		if err != nil {
			return nil, fmt.Errorf("deploy variant %s: %w", req.VariantName, err)
		}
	}

	// Create variant record
	variant, err := domain.NewTrafficVariant(config.ID, req.ModelVersionID, req.VariantName, req.Weight)
	if err != nil {
		return nil, err
	}
	variant.KServeISVCName = variantISVCName
	if req.Weight > 0 {
		variant.Activate()
	}

	if err := s.variantRepo.Create(ctx, variant); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, req.ProjectID, req.ConfigID)
}

func (s *TrafficService) UpdateVariant(ctx context.Context, projectID, configID uuid.UUID, variantName string, weight int) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, projectID, configID)
	if err != nil {
		return nil, err
	}

	variant := config.GetVariant(variantName)
	if variant == nil {
		return nil, domain.ErrTrafficVariantNotFound
	}

	// Calculate new total (excluding current variant)
	newTotal := config.TotalWeight() - variant.Weight + weight
	if newTotal > 100 {
		return nil, domain.ErrWeightSumExceeds100
	}

	if err := variant.SetWeight(weight); err != nil {
		return nil, err
	}
	if weight > 0 {
		variant.Activate()
	} else {
		variant.Status = domain.VariantStatusInactive
	}

	if err := s.variantRepo.Update(ctx, variant); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, projectID, configID)
}

func (s *TrafficService) DeleteVariant(ctx context.Context, projectID, configID uuid.UUID, variantName string) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, projectID, configID)
	if err != nil {
		return nil, err
	}

	// Cannot delete "stable" variant
	if variantName == "stable" {
		return nil, domain.ErrCannotDeleteStable
	}

	variant := config.GetVariant(variantName)
	if variant == nil {
		return nil, domain.ErrTrafficVariantNotFound
	}

	// Delete KServe ISVC
	if s.kserve != nil && s.kserve.IsAvailable() && variant.KServeISVCName != "" {
		isvc, _ := s.isvcRepo.GetByID(ctx, projectID, config.InferenceServiceID)
		if isvc != nil {
			env, _ := s.envRepo.GetByID(ctx, projectID, isvc.ServingEnvironmentID)
			namespace := env.Name
			if env.ExternalID != "" {
				namespace = env.ExternalID
			}
			s.kserve.Undeploy(ctx, namespace, variant.KServeISVCName)
		}
	}

	if err := s.variantRepo.Delete(ctx, variant.ID); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, projectID, configID)
}

type BulkUpdateWeightsRequest struct {
	ProjectID uuid.UUID
	ConfigID  uuid.UUID
	Weights   map[string]int // variant_name -> weight
}

func (s *TrafficService) BulkUpdateWeights(ctx context.Context, req BulkUpdateWeightsRequest) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, req.ProjectID, req.ConfigID)
	if err != nil {
		return nil, err
	}

	// Validate total weight
	total := 0
	for _, w := range req.Weights {
		if w < 0 || w > 100 {
			return nil, domain.ErrInvalidTrafficWeight
		}
		total += w
	}
	if total > 100 {
		return nil, domain.ErrWeightSumExceeds100
	}

	// Update each variant
	for name, weight := range req.Weights {
		variant := config.GetVariant(name)
		if variant == nil {
			return nil, fmt.Errorf("variant %s: %w", name, domain.ErrTrafficVariantNotFound)
		}

		if err := variant.SetWeight(weight); err != nil {
			return nil, err
		}
		if weight > 0 {
			variant.Activate()
		} else {
			variant.Status = domain.VariantStatusInactive
		}

		if err := s.variantRepo.Update(ctx, variant); err != nil {
			return nil, err
		}
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, req.ProjectID, req.ConfigID)
}

// PromoteVariant promotes any variant to stable (generic version of PromoteCanary)
func (s *TrafficService) PromoteVariant(ctx context.Context, projectID, configID uuid.UUID, variantName string) (*domain.TrafficConfig, error) {
	config, err := s.GetConfig(ctx, projectID, configID)
	if err != nil {
		return nil, err
	}

	if variantName == "stable" {
		return nil, domain.ErrCannotPromoteStable
	}

	variant := config.GetVariant(variantName)
	if variant == nil || variant.Status != domain.VariantStatusActive {
		return nil, domain.ErrCannotPromoteInactive
	}

	stable := config.GetVariant("stable")
	if stable == nil {
		return nil, domain.ErrNoStableVariant
	}

	// Swap: variant becomes stable, old stable becomes inactive
	oldStableName := fmt.Sprintf("old-stable-%d", time.Now().Unix())
	stable.VariantName = oldStableName
	stable.Deactivate()

	variant.VariantName = "stable"
	variant.SetWeight(100)

	// Deactivate all other variants
	for _, v := range config.Variants {
		if v.ID != variant.ID && v.ID != stable.ID {
			v.Deactivate()
			s.variantRepo.Update(ctx, v)
		}
	}

	if err := s.variantRepo.Update(ctx, stable); err != nil {
		return nil, err
	}
	if err := s.variantRepo.Update(ctx, variant); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		// Log but don't fail
	}

	return s.GetConfig(ctx, projectID, configID)
}

// ============================================================================
// Helpers
// ============================================================================

func (s *TrafficService) syncAIGatewayRoute(ctx context.Context, config *domain.TrafficConfig) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return nil
	}

	if config.AIGatewayRouteName == "" {
		return nil
	}

	// Reload variants
	variants, err := s.variantRepo.ListByConfig(ctx, config.ID)
	if err != nil {
		return err
	}

	// Build backends
	backends := make([]output.WeightedBackend, 0)
	for _, v := range variants {
		if v.Status == domain.VariantStatusActive && v.Weight > 0 {
			backends = append(backends, output.WeightedBackend{
				Name:       v.KServeISVCName,
				Weight:     v.Weight,
				VariantTag: v.VariantName,
			})
		}
	}

	// Get namespace
	isvc, _ := s.isvcRepo.GetByID(ctx, config.ProjectID, config.InferenceServiceID)
	if isvc == nil {
		return nil
	}
	env, _ := s.envRepo.GetByID(ctx, config.ProjectID, isvc.ServingEnvironmentID)
	namespace := env.Name
	if env.ExternalID != "" {
		namespace = env.ExternalID
	}

	return s.aiGateway.UpdateTrafficWeights(ctx, namespace, config.AIGatewayRouteName, backends)
}
```

---

## 3.3 DTOs

**File**: `internal/adapters/primary/http/dto/traffic.go`

```go
package dto

import (
	"time"
	"github.com/google/uuid"
	"model-registry-service/internal/core/domain"
)

type CreateTrafficConfigRequest struct {
	InferenceServiceID uuid.UUID `json:"inference_service_id" binding:"required"`
	Strategy           string    `json:"strategy"` // canary, ab_test
	StableVersionID    uuid.UUID `json:"stable_version_id" binding:"required"`
}

type StartCanaryRequest struct {
	ModelVersionID uuid.UUID `json:"model_version_id" binding:"required"`
	InitialWeight  int       `json:"initial_weight"` // 5-50, default 10
}

type UpdateCanaryWeightRequest struct {
	Weight int `json:"weight" binding:"required,min=0,max=100"`
}

// Multi-Variant DTOs
type AddVariantRequest struct {
	VariantName    string    `json:"variant_name" binding:"required"`    // e.g., "variant_a", "shadow"
	ModelVersionID uuid.UUID `json:"model_version_id" binding:"required"`
	Weight         int       `json:"weight"`                             // 0-100, default 0
}

type UpdateVariantRequest struct {
	Weight int `json:"weight" binding:"min=0,max=100"`
}

type BulkUpdateWeightsRequest struct {
	Weights map[string]int `json:"weights" binding:"required"` // variant_name -> weight
}

type ListVariantsResponse struct {
	ConfigID uuid.UUID                `json:"config_id"`
	Variants []TrafficVariantResponse `json:"variants"`
	Total    int                      `json:"total"`
}

type TrafficConfigResponse struct {
	ID                  uuid.UUID                `json:"id"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
	InferenceServiceID  uuid.UUID                `json:"inference_service_id"`
	InferenceServiceName string                  `json:"inference_service_name,omitempty"`
	Strategy            string                   `json:"strategy"`
	Status              string                   `json:"status"`
	AIGatewayRouteName  string                   `json:"ai_gateway_route_name,omitempty"`
	Variants            []TrafficVariantResponse `json:"variants"`
}

type TrafficVariantResponse struct {
	ID              uuid.UUID `json:"id"`
	VariantName     string    `json:"variant_name"`
	ModelVersionID  uuid.UUID `json:"model_version_id"`
	ModelVersionName string   `json:"model_version_name,omitempty"`
	Weight          int       `json:"weight"`
	Status          string    `json:"status"`
	KServeISVCName  string    `json:"kserve_isvc_name,omitempty"`
}

type ListTrafficConfigsResponse struct {
	Items      []TrafficConfigResponse `json:"items"`
	Total      int                     `json:"total"`
	PageSize   int                     `json:"page_size"`
	NextOffset int                     `json:"next_offset"`
}

func ToTrafficConfigResponse(config *domain.TrafficConfig) TrafficConfigResponse {
	resp := TrafficConfigResponse{
		ID:                   config.ID,
		CreatedAt:            config.CreatedAt,
		UpdatedAt:            config.UpdatedAt,
		InferenceServiceID:   config.InferenceServiceID,
		InferenceServiceName: config.InferenceServiceName,
		Strategy:             string(config.Strategy),
		Status:               config.Status,
		AIGatewayRouteName:   config.AIGatewayRouteName,
		Variants:             make([]TrafficVariantResponse, 0),
	}

	for _, v := range config.Variants {
		resp.Variants = append(resp.Variants, TrafficVariantResponse{
			ID:               v.ID,
			VariantName:      v.VariantName,
			ModelVersionID:   v.ModelVersionID,
			ModelVersionName: v.ModelVersionName,
			Weight:           v.Weight,
			Status:           string(v.Status),
			KServeISVCName:   v.KServeISVCName,
		})
	}

	return resp
}
```

---

## 3.4 Handlers

**File**: `internal/adapters/primary/http/handlers/traffic.go`

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/dto"
	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/services"
	output "model-registry-service/internal/core/ports/output"
)

// Traffic Config CRUD
func (h *Handler) ListTrafficConfigs(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := output.TrafficConfigFilter{
		ProjectID: projectID,
		Status:    c.Query("status"),
		Limit:     limit,
		Offset:    offset,
	}

	if isvcID := c.Query("inference_service_id"); isvcID != "" {
		if id, err := uuid.Parse(isvcID); err == nil {
			filter.InferenceServiceID = &id
		}
	}

	configs, total, err := h.trafficSvc.ListConfigs(c.Request.Context(), filter)
	if err != nil {
		log.WithError(err).Error("list traffic configs failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.TrafficConfigResponse, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, dto.ToTrafficConfigResponse(cfg))
	}

	c.JSON(http.StatusOK, dto.ListTrafficConfigsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetTrafficConfig(c *gin.Context) {
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

	config, err := h.trafficSvc.GetConfig(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) CreateTrafficConfig(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateTrafficConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	strategy := domain.TrafficStrategyCanary
	if req.Strategy != "" {
		strategy = domain.TrafficStrategy(req.Strategy)
	}

	config, err := h.trafficSvc.CreateConfig(c.Request.Context(), services.CreateTrafficConfigRequest{
		ProjectID:          projectID,
		InferenceServiceID: req.InferenceServiceID,
		Strategy:           strategy,
		StableVersionID:    req.StableVersionID,
	})
	if err != nil {
		log.WithError(err).Error("create traffic config failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) DeleteTrafficConfig(c *gin.Context) {
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

	if err := h.trafficSvc.DeleteConfig(c.Request.Context(), projectID, id); err != nil {
		log.WithError(err).Error("delete traffic config failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Canary Operations
func (h *Handler) StartCanary(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req dto.StartCanaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.StartCanary(c.Request.Context(), services.StartCanaryRequest{
		ProjectID:      projectID,
		ConfigID:       configID,
		ModelVersionID: req.ModelVersionID,
		InitialWeight:  req.InitialWeight,
	})
	if err != nil {
		log.WithError(err).Error("start canary failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) UpdateCanaryWeight(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req dto.UpdateCanaryWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.UpdateCanaryWeight(c.Request.Context(), projectID, configID, req.Weight)
	if err != nil {
		log.WithError(err).Error("update canary weight failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) PromoteCanary(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	config, err := h.trafficSvc.PromoteCanary(c.Request.Context(), projectID, configID)
	if err != nil {
		log.WithError(err).Error("promote canary failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) RollbackCanary(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	config, err := h.trafficSvc.Rollback(c.Request.Context(), projectID, configID)
	if err != nil {
		log.WithError(err).Error("rollback failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

// ============================================================================
// Multi-Variant Handlers
// ============================================================================

func (h *Handler) ListVariants(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	config, err := h.trafficSvc.GetConfig(c.Request.Context(), projectID, configID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	resp := dto.ListVariantsResponse{
		ConfigID: config.ID,
		Variants: make([]dto.TrafficVariantResponse, 0),
		Total:    len(config.Variants),
	}
	for _, v := range config.Variants {
		resp.Variants = append(resp.Variants, dto.TrafficVariantResponse{
			ID:               v.ID,
			VariantName:      v.VariantName,
			ModelVersionID:   v.ModelVersionID,
			ModelVersionName: v.ModelVersionName,
			Weight:           v.Weight,
			Status:           string(v.Status),
			KServeISVCName:   v.KServeISVCName,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) AddVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req dto.AddVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.AddVariant(c.Request.Context(), services.AddVariantRequest{
		ProjectID:      projectID,
		ConfigID:       configID,
		VariantName:    req.VariantName,
		ModelVersionID: req.ModelVersionID,
		Weight:         req.Weight,
	})
	if err != nil {
		log.WithError(err).Error("add variant failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) UpdateVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	variantName := c.Param("name")

	var req dto.UpdateVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.UpdateVariant(c.Request.Context(), projectID, configID, variantName, req.Weight)
	if err != nil {
		log.WithError(err).Error("update variant failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) DeleteVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	variantName := c.Param("name")

	config, err := h.trafficSvc.DeleteVariant(c.Request.Context(), projectID, configID, variantName)
	if err != nil {
		log.WithError(err).Error("delete variant failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) BulkUpdateWeights(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req dto.BulkUpdateWeightsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.BulkUpdateWeights(c.Request.Context(), services.BulkUpdateWeightsRequest{
		ProjectID: projectID,
		ConfigID:  configID,
		Weights:   req.Weights,
	})
	if err != nil {
		log.WithError(err).Error("bulk update weights failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) PromoteVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	variantName := c.Param("variant_name")

	config, err := h.trafficSvc.PromoteVariant(c.Request.Context(), projectID, configID, variantName)
	if err != nil {
		log.WithError(err).Error("promote variant failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}
```

---

## 3.5 Virtual Model Service

**File**: `internal/core/services/virtual_model.go`

```go
package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type VirtualModelService struct {
	vmRepo    output.VirtualModelRepository
	aiGateway output.AIGatewayClient
}

func NewVirtualModelService(
	vmRepo output.VirtualModelRepository,
	aiGateway output.AIGatewayClient,
) *VirtualModelService {
	return &VirtualModelService{
		vmRepo:    vmRepo,
		aiGateway: aiGateway,
	}
}

// ============================================================================
// Virtual Model CRUD
// ============================================================================

type CreateVirtualModelRequest struct {
	ProjectID   uuid.UUID
	Name        string
	Description string
}

func (s *VirtualModelService) Create(ctx context.Context, req CreateVirtualModelRequest) (*domain.VirtualModel, error) {
	vm, err := domain.NewVirtualModel(req.ProjectID, req.Name)
	if err != nil {
		return nil, err
	}
	vm.Description = req.Description

	if err := s.vmRepo.Create(ctx, vm); err != nil {
		return nil, err
	}

	// Create AIGatewayRoute for this virtual model
	if s.aiGateway != nil && s.aiGateway.IsAvailable() {
		route := &output.AIGatewayRoute{
			Name:      fmt.Sprintf("vm-%s", vm.ID.String()[:8]),
			Namespace: "model-serving", // TODO: configurable
			ModelName: vm.Name,
			Backends:  []output.WeightedBackend{},
			Labels: map[string]string{
				"virtual-model-id": vm.ID.String(),
			},
		}

		if err := s.aiGateway.CreateRoute(ctx, route); err != nil {
			// Log but don't fail
		} else {
			vm.AIGatewayRouteName = route.Name
			s.vmRepo.Update(ctx, req.ProjectID, vm)
		}
	}

	return s.vmRepo.GetByID(ctx, req.ProjectID, vm.ID)
}

func (s *VirtualModelService) Get(ctx context.Context, projectID uuid.UUID, name string) (*domain.VirtualModel, error) {
	return s.vmRepo.GetByName(ctx, projectID, name)
}

func (s *VirtualModelService) List(ctx context.Context, projectID uuid.UUID) ([]*domain.VirtualModel, error) {
	return s.vmRepo.List(ctx, projectID)
}

func (s *VirtualModelService) Delete(ctx context.Context, projectID uuid.UUID, name string) error {
	vm, err := s.vmRepo.GetByName(ctx, projectID, name)
	if err != nil {
		return err
	}

	// Delete AIGatewayRoute
	if vm.AIGatewayRouteName != "" && s.aiGateway != nil && s.aiGateway.IsAvailable() {
		s.aiGateway.DeleteRoute(ctx, "model-serving", vm.AIGatewayRouteName)
	}

	return s.vmRepo.Delete(ctx, projectID, vm.ID)
}

// ============================================================================
// Backend Mapping CRUD
// ============================================================================

type AddBackendRequest struct {
	ProjectID             uuid.UUID
	VirtualModelName      string
	AIServiceBackendName  string
	AIServiceBackendNS    string
	ModelNameOverride     *string
	Weight                int
	Priority              int
}

func (s *VirtualModelService) AddBackend(ctx context.Context, req AddBackendRequest) (*domain.VirtualModel, error) {
	vm, err := s.vmRepo.GetByName(ctx, req.ProjectID, req.VirtualModelName)
	if err != nil {
		return nil, err
	}

	// Check backend doesn't already exist
	for _, b := range vm.Backends {
		if b.AIServiceBackendName == req.AIServiceBackendName {
			return nil, domain.ErrBackendAlreadyExists
		}
	}

	backend, err := domain.NewVirtualModelBackend(
		vm.ID,
		req.AIServiceBackendName,
		req.ModelNameOverride,
		req.Weight,
		req.Priority,
	)
	if err != nil {
		return nil, err
	}
	backend.AIServiceBackendNamespace = req.AIServiceBackendNS

	if err := s.vmRepo.CreateBackend(ctx, backend); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, vm); err != nil {
		// Log but don't fail
	}

	return s.vmRepo.GetByName(ctx, req.ProjectID, req.VirtualModelName)
}

func (s *VirtualModelService) UpdateBackend(ctx context.Context, projectID uuid.UUID, vmName string, backendID uuid.UUID, weight int, priority int) (*domain.VirtualModel, error) {
	vm, err := s.vmRepo.GetByName(ctx, projectID, vmName)
	if err != nil {
		return nil, err
	}

	var backend *domain.VirtualModelBackend
	for _, b := range vm.Backends {
		if b.ID == backendID {
			backend = b
			break
		}
	}
	if backend == nil {
		return nil, domain.ErrBackendNotFound
	}

	backend.Weight = weight
	backend.Priority = priority

	if err := s.vmRepo.UpdateBackend(ctx, backend); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, vm); err != nil {
		// Log but don't fail
	}

	return s.vmRepo.GetByName(ctx, projectID, vmName)
}

func (s *VirtualModelService) DeleteBackend(ctx context.Context, projectID uuid.UUID, vmName string, backendID uuid.UUID) (*domain.VirtualModel, error) {
	vm, err := s.vmRepo.GetByName(ctx, projectID, vmName)
	if err != nil {
		return nil, err
	}

	if err := s.vmRepo.DeleteBackend(ctx, backendID); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, vm); err != nil {
		// Log but don't fail
	}

	return s.vmRepo.GetByName(ctx, projectID, vmName)
}

// ============================================================================
// Helpers
// ============================================================================

func (s *VirtualModelService) syncAIGatewayRoute(ctx context.Context, vm *domain.VirtualModel) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return nil
	}

	if vm.AIGatewayRouteName == "" {
		return nil
	}

	// Reload backends
	vm, err := s.vmRepo.GetByID(ctx, vm.ProjectID, vm.ID)
	if err != nil {
		return err
	}

	// Build weighted backends with modelNameOverride
	backends := make([]output.WeightedBackend, 0)
	for _, b := range vm.Backends {
		if b.Status == "active" {
			backends = append(backends, output.WeightedBackend{
				Name:              b.AIServiceBackendName,
				Namespace:         b.AIServiceBackendNamespace,
				Weight:            b.Weight,
				Priority:          b.Priority,
				ModelNameOverride: b.ModelNameOverride,
			})
		}
	}

	return s.aiGateway.UpdateTrafficWeights(ctx, "model-serving", vm.AIGatewayRouteName, backends)
}
```

---

## 3.6 Virtual Model DTOs

**File**: `internal/adapters/primary/http/dto/virtual_model.go`

```go
package dto

import (
	"time"
	"github.com/google/uuid"
	"model-registry-service/internal/core/domain"
)

type CreateVirtualModelRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type AddBackendRequest struct {
	AIServiceBackendName string  `json:"ai_service_backend_name" binding:"required"`
	AIServiceBackendNS   string  `json:"ai_service_backend_namespace"`
	ModelNameOverride    *string `json:"model_name_override"` // nil = use virtual model name
	Weight               int     `json:"weight"`              // default 1
	Priority             int     `json:"priority"`            // 0 = primary, 1+ = fallback
}

type UpdateBackendRequest struct {
	Weight   int `json:"weight"`
	Priority int `json:"priority"`
}

type VirtualModelResponse struct {
	ID                 uuid.UUID                      `json:"id"`
	CreatedAt          time.Time                      `json:"created_at"`
	UpdatedAt          time.Time                      `json:"updated_at"`
	Name               string                         `json:"name"`
	Description        string                         `json:"description,omitempty"`
	AIGatewayRouteName string                         `json:"ai_gateway_route_name,omitempty"`
	Status             string                         `json:"status"`
	Backends           []VirtualModelBackendResponse  `json:"backends"`
}

type VirtualModelBackendResponse struct {
	ID                    uuid.UUID `json:"id"`
	AIServiceBackendName  string    `json:"ai_service_backend_name"`
	AIServiceBackendNS    string    `json:"ai_service_backend_namespace,omitempty"`
	ModelNameOverride     *string   `json:"model_name_override,omitempty"`
	Weight                int       `json:"weight"`
	Priority              int       `json:"priority"`
	Status                string    `json:"status"`
}

type ListVirtualModelsResponse struct {
	Items []VirtualModelResponse `json:"items"`
	Total int                    `json:"total"`
}

func ToVirtualModelResponse(vm *domain.VirtualModel) VirtualModelResponse {
	resp := VirtualModelResponse{
		ID:                 vm.ID,
		CreatedAt:          vm.CreatedAt,
		UpdatedAt:          vm.UpdatedAt,
		Name:               vm.Name,
		Description:        vm.Description,
		AIGatewayRouteName: vm.AIGatewayRouteName,
		Status:             vm.Status,
		Backends:           make([]VirtualModelBackendResponse, 0),
	}

	for _, b := range vm.Backends {
		resp.Backends = append(resp.Backends, VirtualModelBackendResponse{
			ID:                   b.ID,
			AIServiceBackendName: b.AIServiceBackendName,
			AIServiceBackendNS:   b.AIServiceBackendNamespace,
			ModelNameOverride:    b.ModelNameOverride,
			Weight:               b.Weight,
			Priority:             b.Priority,
			Status:               b.Status,
		})
	}

	return resp
}
```

---

## 3.7 Virtual Model Handlers

**File**: `internal/adapters/primary/http/handlers/virtual_model.go`

```go
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

func (h *Handler) ListVirtualModels(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vms, err := h.virtualModelSvc.List(c.Request.Context(), projectID)
	if err != nil {
		log.WithError(err).Error("list virtual models failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.VirtualModelResponse, 0, len(vms))
	for _, vm := range vms {
		items = append(items, dto.ToVirtualModelResponse(vm))
	}

	c.JSON(http.StatusOK, dto.ListVirtualModelsResponse{
		Items: items,
		Total: len(items),
	})
}

func (h *Handler) GetVirtualModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	name := c.Param("name")

	vm, err := h.virtualModelSvc.Get(c.Request.Context(), projectID, name)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) CreateVirtualModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateVirtualModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.virtualModelSvc.Create(c.Request.Context(), services.CreateVirtualModelRequest{
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		log.WithError(err).Error("create virtual model failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) DeleteVirtualModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	name := c.Param("name")

	if err := h.virtualModelSvc.Delete(c.Request.Context(), projectID, name); err != nil {
		log.WithError(err).Error("delete virtual model failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) AddVirtualModelBackend(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vmName := c.Param("name")

	var req dto.AddBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	weight := req.Weight
	if weight == 0 {
		weight = 1
	}

	vm, err := h.virtualModelSvc.AddBackend(c.Request.Context(), services.AddBackendRequest{
		ProjectID:            projectID,
		VirtualModelName:     vmName,
		AIServiceBackendName: req.AIServiceBackendName,
		AIServiceBackendNS:   req.AIServiceBackendNS,
		ModelNameOverride:    req.ModelNameOverride,
		Weight:               weight,
		Priority:             req.Priority,
	})
	if err != nil {
		log.WithError(err).Error("add backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) UpdateVirtualModelBackend(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vmName := c.Param("name")
	backendID, err := uuid.Parse(c.Param("backend_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backend_id"})
		return
	}

	var req dto.UpdateBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.virtualModelSvc.UpdateBackend(c.Request.Context(), projectID, vmName, backendID, req.Weight, req.Priority)
	if err != nil {
		log.WithError(err).Error("update backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) DeleteVirtualModelBackend(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vmName := c.Param("name")
	backendID, err := uuid.Parse(c.Param("backend_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backend_id"})
		return
	}

	vm, err := h.virtualModelSvc.DeleteBackend(c.Request.Context(), projectID, vmName, backendID)
	if err != nil {
		log.WithError(err).Error("delete backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToVirtualModelResponse(vm))
}
```

---

## 3.8 Virtual Model Repository Interface

**File**: `internal/core/ports/output/virtual_model_repository.go`

```go
package ports

import (
	"context"
	"github.com/google/uuid"
	"model-registry-service/internal/core/domain"
)

// VirtualModelRepository defines virtual model persistence
type VirtualModelRepository interface {
	Create(ctx context.Context, vm *domain.VirtualModel) error
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.VirtualModel, error)
	GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.VirtualModel, error)
	Update(ctx context.Context, projectID uuid.UUID, vm *domain.VirtualModel) error
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	List(ctx context.Context, projectID uuid.UUID) ([]*domain.VirtualModel, error)

	// Backend operations
	CreateBackend(ctx context.Context, backend *domain.VirtualModelBackend) error
	UpdateBackend(ctx context.Context, backend *domain.VirtualModelBackend) error
	DeleteBackend(ctx context.Context, backendID uuid.UUID) error
	ListBackends(ctx context.Context, vmID uuid.UUID) ([]*domain.VirtualModelBackend, error)
}
```

---

## 3.9 Traffic Config Repository Implementation

**File**: `internal/adapters/secondary/postgres/traffic_config_repo.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type trafficConfigRepo struct {
	db *sqlx.DB
}

func NewTrafficConfigRepo(db *sqlx.DB) output.TrafficConfigRepository {
	return &trafficConfigRepo{db: db}
}

type trafficConfigRow struct {
	ID                 uuid.UUID      `db:"id"`
	CreatedAt          sql.NullTime   `db:"created_at"`
	UpdatedAt          sql.NullTime   `db:"updated_at"`
	ProjectID          uuid.UUID      `db:"project_id"`
	InferenceServiceID uuid.UUID      `db:"inference_service_id"`
	Strategy           string         `db:"strategy"`
	AIGatewayRouteName sql.NullString `db:"ai_gateway_route_name"`
	Status             string         `db:"status"`
}

func (r *trafficConfigRepo) Create(ctx context.Context, config *domain.TrafficConfig) error {
	query := `
		INSERT INTO traffic_config (id, project_id, inference_service_id, strategy, ai_gateway_route_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.ProjectID,
		config.InferenceServiceID,
		config.Strategy,
		nullString(config.AIGatewayRouteName),
		config.Status,
	)
	if err != nil {
		return fmt.Errorf("insert traffic_config: %w", err)
	}
	return nil
}

func (r *trafficConfigRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.TrafficConfig, error) {
	query := `
		SELECT tc.id, tc.created_at, tc.updated_at, tc.project_id, tc.inference_service_id,
		       tc.strategy, tc.ai_gateway_route_name, tc.status,
		       COALESCE(isvc.name, '') as inference_service_name
		FROM traffic_config tc
		LEFT JOIN inference_service isvc ON tc.inference_service_id = isvc.id
		WHERE tc.id = $1 AND tc.project_id = $2
	`
	row := r.db.QueryRowxContext(ctx, query, id, projectID)

	var cfg trafficConfigRow
	var isvcName string
	err := row.Scan(
		&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt, &cfg.ProjectID, &cfg.InferenceServiceID,
		&cfg.Strategy, &cfg.AIGatewayRouteName, &cfg.Status, &isvcName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTrafficConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan traffic_config: %w", err)
	}

	return r.toDomain(&cfg, isvcName), nil
}

func (r *trafficConfigRepo) GetByISVC(ctx context.Context, projectID, isvcID uuid.UUID) (*domain.TrafficConfig, error) {
	query := `
		SELECT tc.id, tc.created_at, tc.updated_at, tc.project_id, tc.inference_service_id,
		       tc.strategy, tc.ai_gateway_route_name, tc.status,
		       COALESCE(isvc.name, '') as inference_service_name
		FROM traffic_config tc
		LEFT JOIN inference_service isvc ON tc.inference_service_id = isvc.id
		WHERE tc.inference_service_id = $1 AND tc.project_id = $2
	`
	row := r.db.QueryRowxContext(ctx, query, isvcID, projectID)

	var cfg trafficConfigRow
	var isvcName string
	err := row.Scan(
		&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt, &cfg.ProjectID, &cfg.InferenceServiceID,
		&cfg.Strategy, &cfg.AIGatewayRouteName, &cfg.Status, &isvcName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTrafficConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan traffic_config: %w", err)
	}

	return r.toDomain(&cfg, isvcName), nil
}

func (r *trafficConfigRepo) Update(ctx context.Context, projectID uuid.UUID, config *domain.TrafficConfig) error {
	query := `
		UPDATE traffic_config
		SET strategy = $1, ai_gateway_route_name = $2, status = $3, updated_at = NOW()
		WHERE id = $4 AND project_id = $5
	`
	result, err := r.db.ExecContext(ctx, query,
		config.Strategy,
		nullString(config.AIGatewayRouteName),
		config.Status,
		config.ID,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("update traffic_config: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTrafficConfigNotFound
	}
	return nil
}

func (r *trafficConfigRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	query := `DELETE FROM traffic_config WHERE id = $1 AND project_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete traffic_config: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTrafficConfigNotFound
	}
	return nil
}

func (r *trafficConfigRepo) List(ctx context.Context, filter output.TrafficConfigFilter) ([]*domain.TrafficConfig, int, error) {
	query := `
		SELECT tc.id, tc.created_at, tc.updated_at, tc.project_id, tc.inference_service_id,
		       tc.strategy, tc.ai_gateway_route_name, tc.status,
		       COALESCE(isvc.name, '') as inference_service_name
		FROM traffic_config tc
		LEFT JOIN inference_service isvc ON tc.inference_service_id = isvc.id
		WHERE tc.project_id = $1
	`
	args := []interface{}{filter.ProjectID}
	argIdx := 2

	if filter.InferenceServiceID != nil {
		query += fmt.Sprintf(" AND tc.inference_service_id = $%d", argIdx)
		args = append(args, *filter.InferenceServiceID)
		argIdx++
	}
	if filter.Strategy != "" {
		query += fmt.Sprintf(" AND tc.strategy = $%d", argIdx)
		args = append(args, filter.Strategy)
		argIdx++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND tc.status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS sub"
	var total int
	r.db.GetContext(ctx, &total, countQuery, args...)

	// Add pagination
	query += " ORDER BY tc.created_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query traffic_configs: %w", err)
	}
	defer rows.Close()

	var configs []*domain.TrafficConfig
	for rows.Next() {
		var cfg trafficConfigRow
		var isvcName string
		if err := rows.Scan(
			&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt, &cfg.ProjectID, &cfg.InferenceServiceID,
			&cfg.Strategy, &cfg.AIGatewayRouteName, &cfg.Status, &isvcName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan row: %w", err)
		}
		configs = append(configs, r.toDomain(&cfg, isvcName))
	}

	return configs, total, nil
}

func (r *trafficConfigRepo) toDomain(row *trafficConfigRow, isvcName string) *domain.TrafficConfig {
	cfg := &domain.TrafficConfig{
		ID:                   row.ID,
		ProjectID:            row.ProjectID,
		InferenceServiceID:   row.InferenceServiceID,
		Strategy:             domain.TrafficStrategy(row.Strategy),
		Status:               row.Status,
		InferenceServiceName: isvcName,
	}
	if row.CreatedAt.Valid {
		cfg.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		cfg.UpdatedAt = row.UpdatedAt.Time
	}
	if row.AIGatewayRouteName.Valid {
		cfg.AIGatewayRouteName = row.AIGatewayRouteName.String
	}
	return cfg
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
```

---

## 3.10 Traffic Variant Repository Implementation

**File**: `internal/adapters/secondary/postgres/traffic_variant_repo.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type trafficVariantRepo struct {
	db *sqlx.DB
}

func NewTrafficVariantRepo(db *sqlx.DB) output.TrafficVariantRepository {
	return &trafficVariantRepo{db: db}
}

type trafficVariantRow struct {
	ID              uuid.UUID      `db:"id"`
	CreatedAt       sql.NullTime   `db:"created_at"`
	UpdatedAt       sql.NullTime   `db:"updated_at"`
	TrafficConfigID uuid.UUID      `db:"traffic_config_id"`
	VariantName     string         `db:"variant_name"`
	ModelVersionID  uuid.UUID      `db:"model_version_id"`
	Weight          int            `db:"weight"`
	KServeISVCName  sql.NullString `db:"kserve_isvc_name"`
	KServeRevision  sql.NullString `db:"kserve_revision"`
	Status          string         `db:"status"`
}

func (r *trafficVariantRepo) Create(ctx context.Context, variant *domain.TrafficVariant) error {
	query := `
		INSERT INTO traffic_variant (id, traffic_config_id, variant_name, model_version_id, weight, kserve_isvc_name, kserve_revision, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query,
		variant.ID,
		variant.TrafficConfigID,
		variant.VariantName,
		variant.ModelVersionID,
		variant.Weight,
		nullString(variant.KServeISVCName),
		nullString(variant.KServeRevision),
		variant.Status,
	)
	if err != nil {
		return fmt.Errorf("insert traffic_variant: %w", err)
	}
	return nil
}

func (r *trafficVariantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.TrafficVariant, error) {
	query := `
		SELECT tv.id, tv.created_at, tv.updated_at, tv.traffic_config_id, tv.variant_name,
		       tv.model_version_id, tv.weight, tv.kserve_isvc_name, tv.kserve_revision, tv.status,
		       COALESCE(mv.name, '') as model_version_name
		FROM traffic_variant tv
		LEFT JOIN model_version mv ON tv.model_version_id = mv.id
		WHERE tv.id = $1
	`
	row := r.db.QueryRowxContext(ctx, query, id)

	var v trafficVariantRow
	var mvName string
	err := row.Scan(
		&v.ID, &v.CreatedAt, &v.UpdatedAt, &v.TrafficConfigID, &v.VariantName,
		&v.ModelVersionID, &v.Weight, &v.KServeISVCName, &v.KServeRevision, &v.Status, &mvName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTrafficVariantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan traffic_variant: %w", err)
	}

	return r.toDomain(&v, mvName), nil
}

func (r *trafficVariantRepo) GetByName(ctx context.Context, configID uuid.UUID, name string) (*domain.TrafficVariant, error) {
	query := `
		SELECT tv.id, tv.created_at, tv.updated_at, tv.traffic_config_id, tv.variant_name,
		       tv.model_version_id, tv.weight, tv.kserve_isvc_name, tv.kserve_revision, tv.status,
		       COALESCE(mv.name, '') as model_version_name
		FROM traffic_variant tv
		LEFT JOIN model_version mv ON tv.model_version_id = mv.id
		WHERE tv.traffic_config_id = $1 AND tv.variant_name = $2
	`
	row := r.db.QueryRowxContext(ctx, query, configID, name)

	var v trafficVariantRow
	var mvName string
	err := row.Scan(
		&v.ID, &v.CreatedAt, &v.UpdatedAt, &v.TrafficConfigID, &v.VariantName,
		&v.ModelVersionID, &v.Weight, &v.KServeISVCName, &v.KServeRevision, &v.Status, &mvName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTrafficVariantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan traffic_variant: %w", err)
	}

	return r.toDomain(&v, mvName), nil
}

func (r *trafficVariantRepo) Update(ctx context.Context, variant *domain.TrafficVariant) error {
	query := `
		UPDATE traffic_variant
		SET variant_name = $1, weight = $2, kserve_isvc_name = $3, kserve_revision = $4, status = $5, updated_at = NOW()
		WHERE id = $6
	`
	result, err := r.db.ExecContext(ctx, query,
		variant.VariantName,
		variant.Weight,
		nullString(variant.KServeISVCName),
		nullString(variant.KServeRevision),
		variant.Status,
		variant.ID,
	)
	if err != nil {
		return fmt.Errorf("update traffic_variant: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTrafficVariantNotFound
	}
	return nil
}

func (r *trafficVariantRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM traffic_variant WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete traffic_variant: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTrafficVariantNotFound
	}
	return nil
}

func (r *trafficVariantRepo) ListByConfig(ctx context.Context, configID uuid.UUID) ([]*domain.TrafficVariant, error) {
	query := `
		SELECT tv.id, tv.created_at, tv.updated_at, tv.traffic_config_id, tv.variant_name,
		       tv.model_version_id, tv.weight, tv.kserve_isvc_name, tv.kserve_revision, tv.status,
		       COALESCE(mv.name, '') as model_version_name
		FROM traffic_variant tv
		LEFT JOIN model_version mv ON tv.model_version_id = mv.id
		WHERE tv.traffic_config_id = $1
		ORDER BY tv.variant_name
	`
	rows, err := r.db.QueryxContext(ctx, query, configID)
	if err != nil {
		return nil, fmt.Errorf("query traffic_variants: %w", err)
	}
	defer rows.Close()

	var variants []*domain.TrafficVariant
	for rows.Next() {
		var v trafficVariantRow
		var mvName string
		if err := rows.Scan(
			&v.ID, &v.CreatedAt, &v.UpdatedAt, &v.TrafficConfigID, &v.VariantName,
			&v.ModelVersionID, &v.Weight, &v.KServeISVCName, &v.KServeRevision, &v.Status, &mvName,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		variants = append(variants, r.toDomain(&v, mvName))
	}

	return variants, nil
}

func (r *trafficVariantRepo) DeleteByConfig(ctx context.Context, configID uuid.UUID) error {
	query := `DELETE FROM traffic_variant WHERE traffic_config_id = $1`
	_, err := r.db.ExecContext(ctx, query, configID)
	if err != nil {
		return fmt.Errorf("delete traffic_variants by config: %w", err)
	}
	return nil
}

func (r *trafficVariantRepo) toDomain(row *trafficVariantRow, mvName string) *domain.TrafficVariant {
	v := &domain.TrafficVariant{
		ID:               row.ID,
		TrafficConfigID:  row.TrafficConfigID,
		VariantName:      row.VariantName,
		ModelVersionID:   row.ModelVersionID,
		Weight:           row.Weight,
		Status:           domain.VariantStatus(row.Status),
		ModelVersionName: mvName,
	}
	if row.CreatedAt.Valid {
		v.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		v.UpdatedAt = row.UpdatedAt.Time
	}
	if row.KServeISVCName.Valid {
		v.KServeISVCName = row.KServeISVCName.String
	}
	if row.KServeRevision.Valid {
		v.KServeRevision = row.KServeRevision.String
	}
	return v
}
```

---

## 3.11 Virtual Model Repository Implementation

**File**: `internal/adapters/secondary/postgres/virtual_model_repo.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type virtualModelRepo struct {
	db *sqlx.DB
}

func NewVirtualModelRepo(db *sqlx.DB) output.VirtualModelRepository {
	return &virtualModelRepo{db: db}
}

type virtualModelRow struct {
	ID                 uuid.UUID      `db:"id"`
	CreatedAt          sql.NullTime   `db:"created_at"`
	UpdatedAt          sql.NullTime   `db:"updated_at"`
	ProjectID          uuid.UUID      `db:"project_id"`
	Name               string         `db:"name"`
	Description        sql.NullString `db:"description"`
	AIGatewayRouteName sql.NullString `db:"ai_gateway_route_name"`
	Status             string         `db:"status"`
}

type virtualModelBackendRow struct {
	ID                        uuid.UUID      `db:"id"`
	CreatedAt                 sql.NullTime   `db:"created_at"`
	UpdatedAt                 sql.NullTime   `db:"updated_at"`
	VirtualModelID            uuid.UUID      `db:"virtual_model_id"`
	AIServiceBackendName      string         `db:"ai_service_backend_name"`
	AIServiceBackendNamespace sql.NullString `db:"ai_service_backend_namespace"`
	ModelNameOverride         sql.NullString `db:"model_name_override"`
	Weight                    int            `db:"weight"`
	Priority                  int            `db:"priority"`
	Status                    string         `db:"status"`
}

func (r *virtualModelRepo) Create(ctx context.Context, vm *domain.VirtualModel) error {
	query := `
		INSERT INTO virtual_model (id, project_id, name, description, ai_gateway_route_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query,
		vm.ID,
		vm.ProjectID,
		vm.Name,
		nullString(vm.Description),
		nullString(vm.AIGatewayRouteName),
		vm.Status,
	)
	if err != nil {
		return fmt.Errorf("insert virtual_model: %w", err)
	}
	return nil
}

func (r *virtualModelRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.VirtualModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, ai_gateway_route_name, status
		FROM virtual_model
		WHERE id = $1 AND project_id = $2
	`
	row := r.db.QueryRowxContext(ctx, query, id, projectID)

	var vm virtualModelRow
	err := row.Scan(
		&vm.ID, &vm.CreatedAt, &vm.UpdatedAt, &vm.ProjectID, &vm.Name,
		&vm.Description, &vm.AIGatewayRouteName, &vm.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrVirtualModelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan virtual_model: %w", err)
	}

	result := r.toDomain(&vm)

	// Load backends
	backends, err := r.ListBackends(ctx, result.ID)
	if err != nil {
		return nil, err
	}
	result.Backends = backends

	return result, nil
}

func (r *virtualModelRepo) GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.VirtualModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, ai_gateway_route_name, status
		FROM virtual_model
		WHERE project_id = $1 AND name = $2
	`
	row := r.db.QueryRowxContext(ctx, query, projectID, name)

	var vm virtualModelRow
	err := row.Scan(
		&vm.ID, &vm.CreatedAt, &vm.UpdatedAt, &vm.ProjectID, &vm.Name,
		&vm.Description, &vm.AIGatewayRouteName, &vm.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrVirtualModelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan virtual_model: %w", err)
	}

	result := r.toDomain(&vm)

	// Load backends
	backends, err := r.ListBackends(ctx, result.ID)
	if err != nil {
		return nil, err
	}
	result.Backends = backends

	return result, nil
}

func (r *virtualModelRepo) Update(ctx context.Context, projectID uuid.UUID, vm *domain.VirtualModel) error {
	query := `
		UPDATE virtual_model
		SET name = $1, description = $2, ai_gateway_route_name = $3, status = $4, updated_at = NOW()
		WHERE id = $5 AND project_id = $6
	`
	result, err := r.db.ExecContext(ctx, query,
		vm.Name,
		nullString(vm.Description),
		nullString(vm.AIGatewayRouteName),
		vm.Status,
		vm.ID,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("update virtual_model: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrVirtualModelNotFound
	}
	return nil
}

func (r *virtualModelRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	query := `DELETE FROM virtual_model WHERE id = $1 AND project_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete virtual_model: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrVirtualModelNotFound
	}
	return nil
}

func (r *virtualModelRepo) List(ctx context.Context, projectID uuid.UUID) ([]*domain.VirtualModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, ai_gateway_route_name, status
		FROM virtual_model
		WHERE project_id = $1
		ORDER BY name
	`
	rows, err := r.db.QueryxContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("query virtual_models: %w", err)
	}
	defer rows.Close()

	var models []*domain.VirtualModel
	for rows.Next() {
		var vm virtualModelRow
		if err := rows.Scan(
			&vm.ID, &vm.CreatedAt, &vm.UpdatedAt, &vm.ProjectID, &vm.Name,
			&vm.Description, &vm.AIGatewayRouteName, &vm.Status,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		model := r.toDomain(&vm)

		// Load backends for each model
		backends, _ := r.ListBackends(ctx, model.ID)
		model.Backends = backends

		models = append(models, model)
	}

	return models, nil
}

// Backend operations

func (r *virtualModelRepo) CreateBackend(ctx context.Context, backend *domain.VirtualModelBackend) error {
	query := `
		INSERT INTO virtual_model_backend (id, virtual_model_id, ai_service_backend_name, ai_service_backend_namespace, model_name_override, weight, priority, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	var modelOverride sql.NullString
	if backend.ModelNameOverride != nil {
		modelOverride = sql.NullString{String: *backend.ModelNameOverride, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		backend.ID,
		backend.VirtualModelID,
		backend.AIServiceBackendName,
		nullString(backend.AIServiceBackendNamespace),
		modelOverride,
		backend.Weight,
		backend.Priority,
		backend.Status,
	)
	if err != nil {
		return fmt.Errorf("insert virtual_model_backend: %w", err)
	}
	return nil
}

func (r *virtualModelRepo) UpdateBackend(ctx context.Context, backend *domain.VirtualModelBackend) error {
	query := `
		UPDATE virtual_model_backend
		SET weight = $1, priority = $2, status = $3, updated_at = NOW()
		WHERE id = $4
	`
	result, err := r.db.ExecContext(ctx, query,
		backend.Weight,
		backend.Priority,
		backend.Status,
		backend.ID,
	)
	if err != nil {
		return fmt.Errorf("update virtual_model_backend: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrBackendNotFound
	}
	return nil
}

func (r *virtualModelRepo) DeleteBackend(ctx context.Context, backendID uuid.UUID) error {
	query := `DELETE FROM virtual_model_backend WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, backendID)
	if err != nil {
		return fmt.Errorf("delete virtual_model_backend: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrBackendNotFound
	}
	return nil
}

func (r *virtualModelRepo) ListBackends(ctx context.Context, vmID uuid.UUID) ([]*domain.VirtualModelBackend, error) {
	query := `
		SELECT id, created_at, updated_at, virtual_model_id, ai_service_backend_name,
		       ai_service_backend_namespace, model_name_override, weight, priority, status
		FROM virtual_model_backend
		WHERE virtual_model_id = $1
		ORDER BY priority, weight DESC
	`
	rows, err := r.db.QueryxContext(ctx, query, vmID)
	if err != nil {
		return nil, fmt.Errorf("query virtual_model_backends: %w", err)
	}
	defer rows.Close()

	var backends []*domain.VirtualModelBackend
	for rows.Next() {
		var b virtualModelBackendRow
		if err := rows.Scan(
			&b.ID, &b.CreatedAt, &b.UpdatedAt, &b.VirtualModelID, &b.AIServiceBackendName,
			&b.AIServiceBackendNamespace, &b.ModelNameOverride, &b.Weight, &b.Priority, &b.Status,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		backends = append(backends, r.backendToDomain(&b))
	}

	return backends, nil
}

func (r *virtualModelRepo) toDomain(row *virtualModelRow) *domain.VirtualModel {
	vm := &domain.VirtualModel{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		Name:      row.Name,
		Status:    row.Status,
	}
	if row.CreatedAt.Valid {
		vm.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		vm.UpdatedAt = row.UpdatedAt.Time
	}
	if row.Description.Valid {
		vm.Description = row.Description.String
	}
	if row.AIGatewayRouteName.Valid {
		vm.AIGatewayRouteName = row.AIGatewayRouteName.String
	}
	return vm
}

func (r *virtualModelRepo) backendToDomain(row *virtualModelBackendRow) *domain.VirtualModelBackend {
	b := &domain.VirtualModelBackend{
		ID:                   row.ID,
		VirtualModelID:       row.VirtualModelID,
		AIServiceBackendName: row.AIServiceBackendName,
		Weight:               row.Weight,
		Priority:             row.Priority,
		Status:               row.Status,
	}
	if row.CreatedAt.Valid {
		b.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		b.UpdatedAt = row.UpdatedAt.Time
	}
	if row.AIServiceBackendNamespace.Valid {
		b.AIServiceBackendNamespace = row.AIServiceBackendNamespace.String
	}
	if row.ModelNameOverride.Valid {
		override := row.ModelNameOverride.String
		b.ModelNameOverride = &override
	}
	return b
}
```

---

## Checklist

### Traffic Management
- [ ] Create `internal/core/ports/output/traffic_repository.go`
- [ ] Create `internal/adapters/secondary/postgres/traffic_config_repo.go`
- [ ] Create `internal/adapters/secondary/postgres/traffic_variant_repo.go`
- [ ] Create `internal/core/services/traffic.go`
- [ ] Create `internal/adapters/primary/http/dto/traffic.go`
- [ ] Create `internal/adapters/primary/http/handlers/traffic.go`
- [ ] Test canary flow end-to-end
- [ ] Test multi-variant A/B testing flow
- [ ] Test bulk weight update
- [ ] Test variant promotion

### Virtual Model (Model Name Virtualization)
- [ ] Create `internal/core/ports/output/virtual_model_repository.go`
- [ ] Create `internal/adapters/secondary/postgres/virtual_model_repo.go`
- [ ] Create `internal/core/services/virtual_model.go`
- [ ] Create `internal/adapters/primary/http/dto/virtual_model.go`
- [ ] Create `internal/adapters/primary/http/handlers/virtual_model.go`
- [ ] Test virtual model with multi-provider routing
- [ ] Test virtual model with fallback (priority)
- [ ] Test modelNameOverride injection into AIGatewayRoute

### Integration
- [ ] Update handler.go with traffic + virtual model services
- [ ] Add routes to RegisterRoutes()
- [ ] Update error mapper with new errors
- [ ] Wire in main.go

---

## 3.12 Integration: Handler Struct Updates

**File**: `internal/adapters/primary/http/handlers/handler.go` (modify)

```go
type Handler struct {
	// ... existing fields ...
	trafficSvc      *services.TrafficService
	virtualModelSvc *services.VirtualModelService
	metricsSvc      *services.MetricsService
}

func NewHandler(
	// ... existing params ...
	trafficSvc *services.TrafficService,
	virtualModelSvc *services.VirtualModelService,
	metricsSvc *services.MetricsService,
) *Handler {
	return &Handler{
		// ... existing assignments ...
		trafficSvc:      trafficSvc,
		virtualModelSvc: virtualModelSvc,
		metricsSvc:      metricsSvc,
	}
}
```

---

## 3.13 Integration: Route Registration

**File**: `cmd/server/main.go` (add to router setup)

```go
// Traffic management routes
trafficGroup := api.Group("/traffic_configs")
{
	trafficGroup.POST("", handler.CreateTrafficConfig)
	trafficGroup.GET("", handler.ListTrafficConfigs)
	trafficGroup.GET("/:id", handler.GetTrafficConfig)
	trafficGroup.DELETE("/:id", handler.DeleteTrafficConfig)

	// Canary operations
	trafficGroup.POST("/:id/canary", handler.StartCanary)
	trafficGroup.PATCH("/:id/canary/weight", handler.UpdateCanaryWeight)
	trafficGroup.POST("/:id/canary/promote", handler.PromoteCanary)
	trafficGroup.POST("/:id/rollback", handler.RollbackCanary)

	// Multi-variant operations
	trafficGroup.GET("/:id/variants", handler.ListVariants)
	trafficGroup.POST("/:id/variants", handler.AddVariant)
	trafficGroup.PATCH("/:id/variants/:name", handler.UpdateVariant)
	trafficGroup.DELETE("/:id/variants/:name", handler.DeleteVariant)
	trafficGroup.PATCH("/:id/weights", handler.BulkUpdateWeights)
	trafficGroup.POST("/:id/promote/:variant_name", handler.PromoteVariant)
}

// Virtual model routes
vmGroup := api.Group("/virtual_models")
{
	vmGroup.POST("", handler.CreateVirtualModel)
	vmGroup.GET("", handler.ListVirtualModels)
	vmGroup.GET("/:name", handler.GetVirtualModel)
	vmGroup.DELETE("/:name", handler.DeleteVirtualModel)

	vmGroup.POST("/:name/backends", handler.AddVirtualModelBackend)
	vmGroup.PATCH("/:name/backends/:backend_id", handler.UpdateVirtualModelBackend)
	vmGroup.DELETE("/:name/backends/:backend_id", handler.DeleteVirtualModelBackend)
}

// Metrics routes
metricsGroup := api.Group("/metrics")
{
	metricsGroup.GET("/deployments/:isvc_name", handler.GetDeploymentMetrics)
	metricsGroup.GET("/compare/:isvc_name", handler.CompareVariants)
	metricsGroup.GET("/tokens", handler.GetTokenUsageMetrics)
}
```

---

## 3.14 Integration: Service Wiring in main.go

**File**: `cmd/server/main.go` (add service initialization)

```go
// AI Gateway client
aiGatewayClient, err := aigateway.NewAIGatewayClient(&cfg.AIGateway)
if err != nil {
	log.WithError(err).Warn("AI Gateway client init failed, traffic features disabled")
}

// Prometheus client
var prometheusClient output.PrometheusClient
if cfg.Prometheus.Enabled {
	prometheusClient = prometheus.NewPrometheusClient(cfg.Prometheus.URL)
}

// Traffic repositories
trafficConfigRepo := postgres.NewTrafficConfigRepo(db)
trafficVariantRepo := postgres.NewTrafficVariantRepo(db)

// Virtual model repository
virtualModelRepo := postgres.NewVirtualModelRepo(db)

// Traffic service
trafficSvc := services.NewTrafficService(
	trafficConfigRepo,
	trafficVariantRepo,
	isvcRepo,       // existing
	versionRepo,    // existing
	envRepo,        // existing
	kserveClient,   // existing
	aiGatewayClient,
)

// Virtual model service
virtualModelSvc := services.NewVirtualModelService(
	virtualModelRepo,
	aiGatewayClient,
)

// Metrics service
metricsSvc := services.NewMetricsService(
	prometheusClient,
	isvcRepo, // existing
)

// Update handler creation
handler := handlers.NewHandler(
	// ... existing services ...
	trafficSvc,
	virtualModelSvc,
	metricsSvc,
)
```

---

## 3.15 Integration: Error Mapper Updates

**File**: `internal/adapters/primary/http/handlers/error-mapper.go` (append)

```go
func mapDomainError(c *gin.Context, err error) {
	switch {
	// ... existing cases ...

	// Traffic errors
	case errors.Is(err, domain.ErrTrafficConfigNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrTrafficVariantNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidVariantName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidTrafficWeight):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrWeightSumExceeds100):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrVariantAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrCanaryAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrNoStableVariant):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrCannotPromoteInactive):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrCannotPromoteStable):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrCannotDeleteStable):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	// Virtual model errors
	case errors.Is(err, domain.ErrVirtualModelNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidVirtualModelName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrVirtualModelExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrBackendNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidBackendName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidPriority):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrBackendAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
```

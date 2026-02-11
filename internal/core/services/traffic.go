package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

// TrafficService handles traffic management operations
type TrafficService struct {
	configRepo  output.TrafficConfigRepository
	variantRepo output.TrafficVariantRepository
	isvcRepo    output.InferenceServiceRepository
	versionRepo output.ModelVersionRepository
	envRepo     output.ServingEnvironmentRepository
	kserve      output.KServeClient
	aiGateway   output.AIGatewayClient
}

// NewTrafficService creates a new traffic service
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

// CreateTrafficConfigRequest contains parameters for creating a traffic config
type CreateTrafficConfigRequest struct {
	ProjectID          uuid.UUID
	InferenceServiceID uuid.UUID
	Strategy           domain.TrafficStrategy
	StableVersionID    uuid.UUID
}

// CreateConfig creates a new traffic configuration
func (s *TrafficService) CreateConfig(ctx context.Context, req CreateTrafficConfigRequest) (*domain.TrafficConfig, error) {
	// Validate ISVC exists
	isvc, err := s.isvcRepo.GetByID(ctx, req.ProjectID, req.InferenceServiceID)
	if err != nil {
		return nil, err
	}

	// Validate version exists
	_, err = s.versionRepo.GetByID(ctx, req.ProjectID, req.StableVersionID)
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

	// Create AI Gateway route if available
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
			},
		}

		if err := s.aiGateway.CreateRoute(ctx, route); err != nil {
			log.WithError(err).Warn("failed to create AI Gateway route")
		} else {
			config.AIGatewayRouteName = route.Name
			s.configRepo.Update(ctx, req.ProjectID, config)
		}
	}

	return s.GetConfig(ctx, req.ProjectID, config.ID)
}

// GetConfig retrieves a traffic config with variants
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

// ListConfigs lists traffic configs with filtering
func (s *TrafficService) ListConfigs(ctx context.Context, filter output.TrafficConfigFilter) ([]*domain.TrafficConfig, int, error) {
	return s.configRepo.List(ctx, filter)
}

// DeleteConfig deletes a traffic config and its variants
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
			if err := s.aiGateway.DeleteRoute(ctx, namespace, config.AIGatewayRouteName); err != nil {
				log.WithError(err).Warn("failed to delete AI Gateway route")
			}
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

// StartCanaryRequest contains parameters for starting a canary deployment
type StartCanaryRequest struct {
	ProjectID      uuid.UUID
	ConfigID       uuid.UUID
	ModelVersionID uuid.UUID
	InitialWeight  int // 5-50, default 10
}

// StartCanary starts a canary deployment
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, req.ProjectID, req.ConfigID)
}

// UpdateCanaryWeight updates the canary traffic weight
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, projectID, configID)
}

// PromoteCanary promotes the canary to stable
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

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, projectID, configID)
}

// Rollback rolls back to stable, removing canary
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
				if err := s.kserve.Undeploy(ctx, namespace, canary.KServeISVCName); err != nil {
					log.WithError(err).Warn("failed to undeploy canary ISVC")
				}
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, projectID, configID)
}

// ============================================================================
// Multi-Variant Operations
// ============================================================================

// AddVariantRequest contains parameters for adding a variant
type AddVariantRequest struct {
	ProjectID      uuid.UUID
	ConfigID       uuid.UUID
	VariantName    string
	ModelVersionID uuid.UUID
	Weight         int
}

// AddVariant adds a new variant to the config
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, req.ProjectID, req.ConfigID)
}

// UpdateVariant updates a variant's weight
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, projectID, configID)
}

// DeleteVariant removes a variant
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
			if err := s.kserve.Undeploy(ctx, namespace, variant.KServeISVCName); err != nil {
				log.WithError(err).Warn("failed to undeploy variant ISVC")
			}
		}
	}

	if err := s.variantRepo.Delete(ctx, variant.ID); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, config); err != nil {
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, projectID, configID)
}

// BulkUpdateWeightsRequest contains parameters for bulk weight update
type BulkUpdateWeightsRequest struct {
	ProjectID uuid.UUID
	ConfigID  uuid.UUID
	Weights   map[string]int // variant_name -> weight
}

// BulkUpdateWeights updates multiple variant weights at once
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.GetConfig(ctx, req.ProjectID, req.ConfigID)
}

// PromoteVariant promotes any variant to stable
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
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

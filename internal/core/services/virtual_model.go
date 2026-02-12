package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

// VirtualModelService handles virtual model operations
type VirtualModelService struct {
	vmRepo    output.VirtualModelRepository
	aiGateway output.AIGatewayClient
}

// NewVirtualModelService creates a new virtual model service
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

// CreateVirtualModelRequest contains parameters for creating a virtual model
type CreateVirtualModelRequest struct {
	ProjectID   uuid.UUID
	Name        string
	Description string
}

// Create creates a new virtual model
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
			Namespace: "model-serving", // TODO: make configurable
			ModelName: vm.Name,
			Backends:  []output.WeightedBackend{},
			Labels: map[string]string{
				"virtual-model-id": vm.ID.String(),
			},
		}

		if err := s.aiGateway.CreateRoute(ctx, route); err != nil {
			log.WithError(err).Warn("failed to create AI Gateway route for virtual model")
		} else {
			vm.AIGatewayRouteName = route.Name
			s.vmRepo.Update(ctx, req.ProjectID, vm)
		}
	}

	return s.vmRepo.GetByID(ctx, req.ProjectID, vm.ID)
}

// Get retrieves a virtual model by name
func (s *VirtualModelService) Get(ctx context.Context, projectID uuid.UUID, name string) (*domain.VirtualModel, error) {
	return s.vmRepo.GetByName(ctx, projectID, name)
}

// GetByID retrieves a virtual model by ID
func (s *VirtualModelService) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.VirtualModel, error) {
	return s.vmRepo.GetByID(ctx, projectID, id)
}

// List lists all virtual models for a project
func (s *VirtualModelService) List(ctx context.Context, projectID uuid.UUID) ([]*domain.VirtualModel, error) {
	return s.vmRepo.List(ctx, projectID)
}

// Delete deletes a virtual model
func (s *VirtualModelService) Delete(ctx context.Context, projectID uuid.UUID, name string) error {
	vm, err := s.vmRepo.GetByName(ctx, projectID, name)
	if err != nil {
		return err
	}

	// Delete AIGatewayRoute
	if vm.AIGatewayRouteName != "" && s.aiGateway != nil && s.aiGateway.IsAvailable() {
		if err := s.aiGateway.DeleteRoute(ctx, "model-serving", vm.AIGatewayRouteName); err != nil {
			log.WithError(err).Warn("failed to delete AI Gateway route for virtual model")
		}
	}

	return s.vmRepo.Delete(ctx, projectID, vm.ID)
}

// ============================================================================
// Backend Mapping CRUD
// ============================================================================

// AddBackendRequest contains parameters for adding a backend
type AddBackendRequest struct {
	ProjectID            uuid.UUID
	VirtualModelName     string
	AIServiceBackendName string
	AIServiceBackendNS   string
	ModelNameOverride    *string
	Weight               int
	Priority             int
}

// AddBackend adds a backend mapping to a virtual model
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

	// Default namespace if not provided
	namespace := req.AIServiceBackendNS
	if namespace == "" {
		namespace = "model-serving"
	}

	// Validate AIServiceBackend exists in K8s
	if s.aiGateway != nil && s.aiGateway.IsAvailable() {
		_, err := s.aiGateway.GetServiceBackend(ctx, namespace, req.AIServiceBackendName)
		if err != nil {
			return nil, fmt.Errorf("backend %s/%s not found: %w", namespace, req.AIServiceBackendName, domain.ErrBackendNotFound)
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
	backend.AIServiceBackendNamespace = namespace

	if err := s.vmRepo.CreateBackend(ctx, backend); err != nil {
		return nil, err
	}

	// Sync to AI Gateway
	if err := s.syncAIGatewayRoute(ctx, vm); err != nil {
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.vmRepo.GetByName(ctx, req.ProjectID, req.VirtualModelName)
}

// UpdateBackend updates a backend's weight and priority
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
	}

	return s.vmRepo.GetByName(ctx, projectID, vmName)
}

// DeleteBackend removes a backend mapping
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
		log.WithError(err).Warn("failed to sync AI Gateway route")
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

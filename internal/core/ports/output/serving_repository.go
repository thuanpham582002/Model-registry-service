package ports

import (
	"context"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
)

// ============================================================================
// Serving Environment Repository
// ============================================================================

// ServingEnvironmentRepository defines the contract for serving environment persistence
type ServingEnvironmentRepository interface {
	// Create creates a new serving environment
	Create(ctx context.Context, env *domain.ServingEnvironment) error

	// GetByID retrieves a serving environment by ID
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServingEnvironment, error)

	// GetByName retrieves a serving environment by name
	GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.ServingEnvironment, error)

	// Update updates a serving environment
	Update(ctx context.Context, projectID uuid.UUID, env *domain.ServingEnvironment) error

	// Delete deletes a serving environment
	Delete(ctx context.Context, projectID, id uuid.UUID) error

	// List lists serving environments with filtering
	List(ctx context.Context, filter ServingEnvironmentFilter) ([]*domain.ServingEnvironment, int, error)
}

// ServingEnvironmentFilter defines filters for listing serving environments
type ServingEnvironmentFilter struct {
	ProjectID uuid.UUID
	Search    string
	SortBy    string
	Order     string
	Limit     int
	Offset    int
}

// ============================================================================
// Inference Service Repository
// ============================================================================

// InferenceServiceRepository defines the contract for inference service persistence
type InferenceServiceRepository interface {
	// Create creates a new inference service
	Create(ctx context.Context, isvc *domain.InferenceService) error

	// GetByID retrieves an inference service by ID
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.InferenceService, error)

	// GetByExternalID retrieves an inference service by K8s resource UID
	GetByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*domain.InferenceService, error)

	// GetByName retrieves an inference service by name within an environment
	GetByName(ctx context.Context, projectID, envID uuid.UUID, name string) (*domain.InferenceService, error)

	// Update updates an inference service
	Update(ctx context.Context, projectID uuid.UUID, isvc *domain.InferenceService) error

	// Delete deletes an inference service
	Delete(ctx context.Context, projectID, id uuid.UUID) error

	// List lists inference services with filtering
	List(ctx context.Context, filter InferenceServiceFilter) ([]*domain.InferenceService, int, error)

	// CountByModel counts inference services for a model
	CountByModel(ctx context.Context, projectID, modelID uuid.UUID) (int, error)

	// CountByEnvironment counts inference services in an environment
	CountByEnvironment(ctx context.Context, projectID, envID uuid.UUID) (int, error)
}

// InferenceServiceFilter defines filters for listing inference services
type InferenceServiceFilter struct {
	ProjectID            uuid.UUID
	ServingEnvironmentID *uuid.UUID
	RegisteredModelID    *uuid.UUID
	DesiredState         string
	CurrentState         string
	SortBy               string
	Order                string
	Limit                int
	Offset               int
}

// ============================================================================
// Serve Model Repository
// ============================================================================

// ServeModelRepository defines the contract for serve model persistence
type ServeModelRepository interface {
	// Create creates a new serve model
	Create(ctx context.Context, sm *domain.ServeModel) error

	// GetByID retrieves a serve model by ID
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServeModel, error)

	// Update updates a serve model
	Update(ctx context.Context, projectID uuid.UUID, sm *domain.ServeModel) error

	// Delete deletes a serve model
	Delete(ctx context.Context, projectID, id uuid.UUID) error

	// List lists serve models with filtering
	List(ctx context.Context, filter ServeModelFilter) ([]*domain.ServeModel, int, error)

	// FindByInferenceService finds serve models by inference service ID
	FindByInferenceService(ctx context.Context, projectID, isvcID uuid.UUID) ([]*domain.ServeModel, error)

	// FindByModelVersion finds serve models by model version ID
	FindByModelVersion(ctx context.Context, projectID, versionID uuid.UUID) ([]*domain.ServeModel, error)
}

// ServeModelFilter defines filters for listing serve models
type ServeModelFilter struct {
	ProjectID          uuid.UUID
	InferenceServiceID *uuid.UUID
	ModelVersionID     *uuid.UUID
	State              string
	SortBy             string
	Order              string
	Limit              int
	Offset             int
}

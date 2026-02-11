package ports

import (
	"context"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
)

// ============================================================================
// Traffic Config Repository
// ============================================================================

// TrafficConfigFilter defines filter options for listing traffic configs
type TrafficConfigFilter struct {
	ProjectID          uuid.UUID
	InferenceServiceID *uuid.UUID
	Strategy           string
	Status             string
	Limit              int
	Offset             int
}

// TrafficConfigRepository defines traffic config persistence operations
type TrafficConfigRepository interface {
	Create(ctx context.Context, config *domain.TrafficConfig) error
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.TrafficConfig, error)
	GetByISVC(ctx context.Context, projectID, isvcID uuid.UUID) (*domain.TrafficConfig, error)
	Update(ctx context.Context, projectID uuid.UUID, config *domain.TrafficConfig) error
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	List(ctx context.Context, filter TrafficConfigFilter) ([]*domain.TrafficConfig, int, error)
}

// ============================================================================
// Traffic Variant Repository
// ============================================================================

// TrafficVariantRepository defines traffic variant persistence operations
type TrafficVariantRepository interface {
	Create(ctx context.Context, variant *domain.TrafficVariant) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.TrafficVariant, error)
	GetByName(ctx context.Context, configID uuid.UUID, name string) (*domain.TrafficVariant, error)
	Update(ctx context.Context, variant *domain.TrafficVariant) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByConfig(ctx context.Context, configID uuid.UUID) ([]*domain.TrafficVariant, error)
	DeleteByConfig(ctx context.Context, configID uuid.UUID) error
}

package ports

import (
	"context"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
)

// VirtualModelRepository defines virtual model persistence operations
type VirtualModelRepository interface {
	// Virtual model CRUD
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

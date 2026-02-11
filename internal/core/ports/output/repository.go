package ports

import (
	"context"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
)

type ListFilter struct {
	ProjectID uuid.UUID
	State     string
	ModelType string
	Search    string
	SortBy    string
	Order     string
	Limit     int
	Offset    int
}

type VersionListFilter struct {
	ProjectID         uuid.UUID
	RegisteredModelID uuid.UUID
	State             string
	Status            string
	SortBy            string
	Order             string
	Limit             int
	Offset            int
}

type RegisteredModelRepository interface {
	Create(ctx context.Context, model *domain.RegisteredModel) error
	GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.RegisteredModel, error)
	GetByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string) (*domain.RegisteredModel, error)
	Update(ctx context.Context, projectID uuid.UUID, model *domain.RegisteredModel) error
	Delete(ctx context.Context, projectID uuid.UUID, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*domain.RegisteredModel, int, error)
}

type ModelVersionRepository interface {
	Create(ctx context.Context, version *domain.ModelVersion) error
	GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.ModelVersion, error)
	GetByModelAndVersion(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, versionID uuid.UUID) (*domain.ModelVersion, error)
	Update(ctx context.Context, projectID uuid.UUID, version *domain.ModelVersion) error
	List(ctx context.Context, filter VersionListFilter) ([]*domain.ModelVersion, int, error)
	ListByModel(ctx context.Context, modelID uuid.UUID, filter VersionListFilter) ([]*domain.ModelVersion, int, error)
	FindByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string, modelID *uuid.UUID) (*domain.ModelVersion, error)
}

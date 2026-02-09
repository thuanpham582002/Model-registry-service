package domain

import (
	"context"

	"github.com/google/uuid"
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
	Create(ctx context.Context, model *RegisteredModel) error
	GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*RegisteredModel, error)
	GetByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string) (*RegisteredModel, error)
	Update(ctx context.Context, projectID uuid.UUID, model *RegisteredModel) error
	Delete(ctx context.Context, projectID uuid.UUID, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*RegisteredModel, int, error)
}

type ModelVersionRepository interface {
	Create(ctx context.Context, version *ModelVersion) error
	GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*ModelVersion, error)
	GetByModelAndVersion(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, versionID uuid.UUID) (*ModelVersion, error)
	Update(ctx context.Context, projectID uuid.UUID, version *ModelVersion) error
	List(ctx context.Context, filter VersionListFilter) ([]*ModelVersion, int, error)
	ListByModel(ctx context.Context, modelID uuid.UUID, filter VersionListFilter) ([]*ModelVersion, int, error)
	FindByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string, modelID *uuid.UUID) (*ModelVersion, error)
}

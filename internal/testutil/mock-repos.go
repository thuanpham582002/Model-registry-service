package testutil

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"
)

// MockRegisteredModelRepo is a mock of RegisteredModelRepository.
type MockRegisteredModelRepo struct {
	mock.Mock
}

func (m *MockRegisteredModelRepo) Create(ctx context.Context, model *domain.RegisteredModel) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

func (m *MockRegisteredModelRepo) GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.RegisteredModel, error) {
	args := m.Called(ctx, projectID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RegisteredModel), args.Error(1)
}

func (m *MockRegisteredModelRepo) GetByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string) (*domain.RegisteredModel, error) {
	args := m.Called(ctx, projectID, name, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RegisteredModel), args.Error(1)
}

func (m *MockRegisteredModelRepo) Update(ctx context.Context, projectID uuid.UUID, model *domain.RegisteredModel) error {
	args := m.Called(ctx, projectID, model)
	return args.Error(0)
}

func (m *MockRegisteredModelRepo) Delete(ctx context.Context, projectID uuid.UUID, id uuid.UUID) error {
	args := m.Called(ctx, projectID, id)
	return args.Error(0)
}

func (m *MockRegisteredModelRepo) List(ctx context.Context, filter ports.ListFilter) ([]*domain.RegisteredModel, int, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.RegisteredModel), args.Int(1), args.Error(2)
}

// MockModelVersionRepo is a mock of ModelVersionRepository.
type MockModelVersionRepo struct {
	mock.Mock
}

func (m *MockModelVersionRepo) Create(ctx context.Context, version *domain.ModelVersion) error {
	args := m.Called(ctx, version)
	return args.Error(0)
}

func (m *MockModelVersionRepo) GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.ModelVersion, error) {
	args := m.Called(ctx, projectID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ModelVersion), args.Error(1)
}

func (m *MockModelVersionRepo) GetByModelAndVersion(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, versionID uuid.UUID) (*domain.ModelVersion, error) {
	args := m.Called(ctx, projectID, modelID, versionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ModelVersion), args.Error(1)
}

func (m *MockModelVersionRepo) Update(ctx context.Context, projectID uuid.UUID, version *domain.ModelVersion) error {
	args := m.Called(ctx, projectID, version)
	return args.Error(0)
}

func (m *MockModelVersionRepo) List(ctx context.Context, filter ports.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.ModelVersion), args.Int(1), args.Error(2)
}

func (m *MockModelVersionRepo) ListByModel(ctx context.Context, modelID uuid.UUID, filter ports.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	args := m.Called(ctx, modelID, filter)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.ModelVersion), args.Int(1), args.Error(2)
}

func (m *MockModelVersionRepo) FindByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string, modelID *uuid.UUID) (*domain.ModelVersion, error) {
	args := m.Called(ctx, projectID, name, externalID, modelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ModelVersion), args.Error(1)
}

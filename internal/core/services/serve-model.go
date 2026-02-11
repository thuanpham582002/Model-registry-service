package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type ServeModelService struct {
	repo        output.ServeModelRepository
	isvcRepo    output.InferenceServiceRepository
	versionRepo output.ModelVersionRepository
}

func NewServeModelService(
	repo output.ServeModelRepository,
	isvcRepo output.InferenceServiceRepository,
	versionRepo output.ModelVersionRepository,
) *ServeModelService {
	return &ServeModelService{
		repo:        repo,
		isvcRepo:    isvcRepo,
		versionRepo: versionRepo,
	}
}

func (s *ServeModelService) Create(
	ctx context.Context,
	projectID uuid.UUID,
	inferenceServiceID uuid.UUID,
	modelVersionID uuid.UUID,
) (*domain.ServeModel, error) {
	// Validate inference service exists
	if _, err := s.isvcRepo.GetByID(ctx, projectID, inferenceServiceID); err != nil {
		return nil, err
	}

	// Validate model version exists
	if _, err := s.versionRepo.GetByID(ctx, projectID, modelVersionID); err != nil {
		return nil, err
	}

	sm, err := domain.NewServeModel(projectID, inferenceServiceID, modelVersionID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, sm); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, sm.ID)
}

func (s *ServeModelService) Get(ctx context.Context, projectID, id uuid.UUID) (*domain.ServeModel, error) {
	return s.repo.GetByID(ctx, projectID, id)
}

func (s *ServeModelService) List(ctx context.Context, filter output.ServeModelFilter) ([]*domain.ServeModel, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.List(ctx, filter)
}

func (s *ServeModelService) UpdateState(
	ctx context.Context,
	projectID, id uuid.UUID,
	state domain.ServeModelState,
) (*domain.ServeModel, error) {
	sm, err := s.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	sm.LastKnownState = state
	sm.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, projectID, sm); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, id)
}

func (s *ServeModelService) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	return s.repo.Delete(ctx, projectID, id)
}

func (s *ServeModelService) FindByInferenceService(ctx context.Context, projectID, isvcID uuid.UUID) ([]*domain.ServeModel, error) {
	return s.repo.FindByInferenceService(ctx, projectID, isvcID)
}

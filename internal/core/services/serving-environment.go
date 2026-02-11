package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type ServingEnvironmentService struct {
	repo   output.ServingEnvironmentRepository
	isvcRepo output.InferenceServiceRepository
}

func NewServingEnvironmentService(
	repo output.ServingEnvironmentRepository,
	isvcRepo output.InferenceServiceRepository,
) *ServingEnvironmentService {
	return &ServingEnvironmentService{
		repo:   repo,
		isvcRepo: isvcRepo,
	}
}

func (s *ServingEnvironmentService) Create(
	ctx context.Context,
	projectID uuid.UUID,
	name, description, externalID string,
) (*domain.ServingEnvironment, error) {
	env, err := domain.NewServingEnvironment(projectID, name, description)
	if err != nil {
		return nil, err
	}
	env.ExternalID = externalID

	if err := s.repo.Create(ctx, env); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, env.ID)
}

func (s *ServingEnvironmentService) Get(ctx context.Context, projectID, id uuid.UUID) (*domain.ServingEnvironment, error) {
	return s.repo.GetByID(ctx, projectID, id)
}

func (s *ServingEnvironmentService) GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.ServingEnvironment, error) {
	return s.repo.GetByName(ctx, projectID, name)
}

func (s *ServingEnvironmentService) List(ctx context.Context, filter output.ServingEnvironmentFilter) ([]*domain.ServingEnvironment, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.List(ctx, filter)
}

func (s *ServingEnvironmentService) Update(
	ctx context.Context,
	projectID, id uuid.UUID,
	name, description, externalID *string,
) (*domain.ServingEnvironment, error) {
	env, err := s.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		env.Name = *name
	}
	if description != nil {
		env.Description = *description
	}
	if externalID != nil {
		env.ExternalID = *externalID
	}
	env.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, projectID, env); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, id)
}

func (s *ServingEnvironmentService) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	// Check if environment has any inference services
	count, err := s.isvcRepo.CountByEnvironment(ctx, projectID, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrServingEnvHasDeployments
	}

	return s.repo.Delete(ctx, projectID, id)
}

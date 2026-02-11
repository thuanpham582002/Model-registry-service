package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type InferenceServiceService struct {
	repo        output.InferenceServiceRepository
	envRepo     output.ServingEnvironmentRepository
	modelRepo   output.RegisteredModelRepository
	versionRepo output.ModelVersionRepository
}

func NewInferenceServiceService(
	repo output.InferenceServiceRepository,
	envRepo output.ServingEnvironmentRepository,
	modelRepo output.RegisteredModelRepository,
	versionRepo output.ModelVersionRepository,
) *InferenceServiceService {
	return &InferenceServiceService{
		repo:        repo,
		envRepo:     envRepo,
		modelRepo:   modelRepo,
		versionRepo: versionRepo,
	}
}

func (s *InferenceServiceService) Create(
	ctx context.Context,
	projectID uuid.UUID,
	name string,
	servingEnvID uuid.UUID,
	modelID uuid.UUID,
	versionID *uuid.UUID,
	runtime string,
	labels map[string]string,
) (*domain.InferenceService, error) {
	// Validate serving environment exists
	if _, err := s.envRepo.GetByID(ctx, projectID, servingEnvID); err != nil {
		return nil, err
	}

	// Validate model exists
	if _, err := s.modelRepo.GetByID(ctx, projectID, modelID); err != nil {
		return nil, err
	}

	// Validate version if provided
	if versionID != nil {
		if _, err := s.versionRepo.GetByID(ctx, projectID, *versionID); err != nil {
			return nil, err
		}
	}

	isvc, err := domain.NewInferenceService(projectID, name, servingEnvID, modelID, versionID)
	if err != nil {
		return nil, err
	}

	if runtime != "" {
		isvc.Runtime = runtime
	}
	if labels != nil {
		isvc.Labels = labels
	}

	if err := s.repo.Create(ctx, isvc); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, isvc.ID)
}

func (s *InferenceServiceService) Get(ctx context.Context, projectID, id uuid.UUID) (*domain.InferenceService, error) {
	return s.repo.GetByID(ctx, projectID, id)
}

func (s *InferenceServiceService) GetByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*domain.InferenceService, error) {
	return s.repo.GetByExternalID(ctx, projectID, externalID)
}

func (s *InferenceServiceService) GetByName(ctx context.Context, projectID, envID uuid.UUID, name string) (*domain.InferenceService, error) {
	return s.repo.GetByName(ctx, projectID, envID, name)
}

func (s *InferenceServiceService) List(ctx context.Context, filter output.InferenceServiceFilter) ([]*domain.InferenceService, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.List(ctx, filter)
}

func (s *InferenceServiceService) Update(
	ctx context.Context,
	projectID, id uuid.UUID,
	updates map[string]interface{},
) (*domain.InferenceService, error) {
	isvc, err := s.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["name"]; ok && v != nil {
		isvc.Name = v.(string)
	}
	if v, ok := updates["external_id"]; ok && v != nil {
		isvc.ExternalID = v.(string)
	}
	if v, ok := updates["desired_state"]; ok && v != nil {
		isvc.DesiredState = domain.InferenceServiceState(v.(string))
	}
	if v, ok := updates["current_state"]; ok && v != nil {
		isvc.CurrentState = domain.InferenceServiceState(v.(string))
	}
	if v, ok := updates["model_version_id"]; ok {
		if v == nil {
			isvc.ModelVersionID = nil
		} else {
			vID := v.(uuid.UUID)
			isvc.ModelVersionID = &vID
		}
	}
	if v, ok := updates["url"]; ok && v != nil {
		isvc.URL = v.(string)
	}
	if v, ok := updates["last_error"]; ok && v != nil {
		isvc.LastError = v.(string)
	}
	if v, ok := updates["labels"]; ok && v != nil {
		isvc.Labels = v.(map[string]string)
	}

	isvc.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, projectID, isvc); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, id)
}

func (s *InferenceServiceService) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	isvc, err := s.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return err
	}

	// Optionally check if currently deployed - prevent deletion
	if isvc.CurrentState == domain.ISStateDeployed {
		return domain.ErrCannotDeleteDeployed
	}

	return s.repo.Delete(ctx, projectID, id)
}

func (s *InferenceServiceService) CountByModel(ctx context.Context, projectID, modelID uuid.UUID) (int, error) {
	return s.repo.CountByModel(ctx, projectID, modelID)
}

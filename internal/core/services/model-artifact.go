package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"
)

type ModelArtifactService struct {
	versionRepo ports.ModelVersionRepository
	modelRepo   ports.RegisteredModelRepository
}

func NewModelArtifactService(versionRepo ports.ModelVersionRepository, modelRepo ports.RegisteredModelRepository) *ModelArtifactService {
	return &ModelArtifactService{versionRepo: versionRepo, modelRepo: modelRepo}
}

func (s *ModelArtifactService) Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.ModelVersion, error) {
	return s.versionRepo.GetByID(ctx, projectID, id)
}

func (s *ModelArtifactService) List(ctx context.Context, projectID uuid.UUID, filter ports.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	filter.ProjectID = projectID
	return s.versionRepo.List(ctx, filter)
}

func (s *ModelArtifactService) Find(ctx context.Context, projectID uuid.UUID, name, externalID string, modelID *uuid.UUID) (*domain.ModelVersion, error) {
	return s.versionRepo.FindByParams(ctx, projectID, name, externalID, modelID)
}

// Create creates a new ModelVersion row (artifact endpoints map to model_version table).
func (s *ModelArtifactService) Create(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, name, description, artifactType, framework, frameworkVersion, containerImage, uri, accessKey, secretKey string, labels map[string]string, prebuiltContainerID *uuid.UUID) (*domain.ModelVersion, error) {
	if _, err := s.modelRepo.GetByID(ctx, projectID, modelID); err != nil {
		return nil, err
	}

	at := domain.ArtifactType(artifactType)
	if at == "" {
		at = domain.ArtifactTypeModel
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	now := time.Now()
	version := &domain.ModelVersion{
		ID:                    uuid.New(),
		CreatedAt:             now,
		UpdatedAt:             now,
		RegisteredModelID:     modelID,
		Name:                  name,
		Description:           description,
		State:                 domain.ModelStateLive,
		Status:                domain.VersionStatusPending,
		ArtifactType:          at,
		ModelFramework:        framework,
		ModelFrameworkVersion: frameworkVersion,
		ContainerImage:        containerImage,
		URI:                   uri,
		AccessKey:             accessKey,
		SecretKey:             secretKey,
		Labels:                labels,
		PrebuiltContainerID:   prebuiltContainerID,
	}

	if err := s.versionRepo.Create(ctx, version); err != nil {
		return nil, err
	}

	return s.versionRepo.GetByID(ctx, projectID, version.ID)
}

func (s *ModelArtifactService) Update(ctx context.Context, projectID uuid.UUID, id uuid.UUID, updates map[string]interface{}) (*domain.ModelVersion, error) {
	version, err := s.versionRepo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["artifact_type"]; ok && v != nil {
		version.ArtifactType = domain.ArtifactType(v.(string))
	}
	if v, ok := updates["model_framework"]; ok && v != nil {
		version.ModelFramework = v.(string)
	}
	if v, ok := updates["model_framework_version"]; ok && v != nil {
		version.ModelFrameworkVersion = v.(string)
	}
	if v, ok := updates["container_image"]; ok && v != nil {
		version.ContainerImage = v.(string)
	}
	if v, ok := updates["uri"]; ok && v != nil {
		version.URI = v.(string)
	}
	if v, ok := updates["labels"]; ok && v != nil {
		version.Labels = v.(map[string]string)
	}

	if err := s.versionRepo.Update(ctx, projectID, version); err != nil {
		return nil, err
	}

	return s.versionRepo.GetByID(ctx, projectID, id)
}

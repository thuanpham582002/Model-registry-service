package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"
)

type ModelVersionService struct {
	repo      ports.ModelVersionRepository
	modelRepo ports.RegisteredModelRepository
}

func NewModelVersionService(repo ports.ModelVersionRepository, modelRepo ports.RegisteredModelRepository) *ModelVersionService {
	return &ModelVersionService{repo: repo, modelRepo: modelRepo}
}

func (s *ModelVersionService) Create(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, name, description string, isDefault bool, artifactType, framework, frameworkVersion, containerImage, catalogName, uri, accessKey, secretKey string, labels map[string]string, prebuiltContainerID *uuid.UUID, createdByID *uuid.UUID, createdByEmail, updatedByEmail string) (*domain.ModelVersion, error) {
	// Verify parent model exists AND belongs to this project
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
		IsDefault:             isDefault,
		State:                 domain.ModelStateLive,
		Status:                domain.VersionStatusPending,
		CreatedByID:           createdByID,
		UpdatedByID:           createdByID,
		CreatedByEmail:        createdByEmail,
		UpdatedByEmail:        updatedByEmail,
		ArtifactType:          at,
		ModelFramework:        framework,
		ModelFrameworkVersion: frameworkVersion,
		ContainerImage:        containerImage,
		ModelCatalogName:      catalogName,
		URI:                   uri,
		AccessKey:             accessKey,
		SecretKey:             secretKey,
		Labels:                labels,
		PrebuiltContainerID:   prebuiltContainerID,
	}

	if err := s.repo.Create(ctx, version); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, version.ID)
}

func (s *ModelVersionService) Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.ModelVersion, error) {
	return s.repo.GetByID(ctx, projectID, id)
}

func (s *ModelVersionService) GetByModel(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, versionID uuid.UUID) (*domain.ModelVersion, error) {
	return s.repo.GetByModelAndVersion(ctx, projectID, modelID, versionID)
}

func (s *ModelVersionService) List(ctx context.Context, filter ports.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.List(ctx, filter)
}

func (s *ModelVersionService) ListByModel(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, filter ports.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	filter.ProjectID = projectID
	return s.repo.ListByModel(ctx, modelID, filter)
}

func (s *ModelVersionService) Find(ctx context.Context, projectID uuid.UUID, name, externalID string, modelID *uuid.UUID) (*domain.ModelVersion, error) {
	return s.repo.FindByParams(ctx, projectID, name, externalID, modelID)
}

func (s *ModelVersionService) Update(ctx context.Context, projectID uuid.UUID, id uuid.UUID, updates map[string]interface{}) (*domain.ModelVersion, error) {
	version, err := s.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["name"]; ok && v != nil {
		version.Name = v.(string)
	}
	if v, ok := updates["description"]; ok && v != nil {
		version.Description = v.(string)
	}
	if v, ok := updates["is_default"]; ok && v != nil {
		version.IsDefault = v.(bool)
	}
	if v, ok := updates["state"]; ok && v != nil {
		version.State = domain.ModelState(v.(string))
	}
	if v, ok := updates["status"]; ok && v != nil {
		version.Status = domain.VersionStatus(v.(string))
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

	if err := s.repo.Update(ctx, projectID, version); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, id)
}

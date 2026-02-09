package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/domain"
)

type ModelArtifactUseCase struct {
	versionRepo domain.ModelVersionRepository
	modelRepo   domain.RegisteredModelRepository
}

func NewModelArtifactUseCase(versionRepo domain.ModelVersionRepository, modelRepo domain.RegisteredModelRepository) *ModelArtifactUseCase {
	return &ModelArtifactUseCase{versionRepo: versionRepo, modelRepo: modelRepo}
}

func (uc *ModelArtifactUseCase) Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.ModelVersion, error) {
	return uc.versionRepo.GetByID(ctx, projectID, id)
}

func (uc *ModelArtifactUseCase) List(ctx context.Context, projectID uuid.UUID, filter domain.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	filter.ProjectID = projectID
	return uc.versionRepo.List(ctx, filter)
}

func (uc *ModelArtifactUseCase) Find(ctx context.Context, projectID uuid.UUID, name, externalID string, modelID *uuid.UUID) (*domain.ModelVersion, error) {
	return uc.versionRepo.FindByParams(ctx, projectID, name, externalID, modelID)
}

// Create creates a new ModelVersion row (artifact endpoints map to model_version table).
func (uc *ModelArtifactUseCase) Create(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, name, description, artifactType, framework, frameworkVersion, containerImage, uri, accessKey, secretKey string, labels map[string]string, prebuiltContainerID *uuid.UUID) (*domain.ModelVersion, error) {
	if _, err := uc.modelRepo.GetByID(ctx, projectID, modelID); err != nil {
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

	if err := uc.versionRepo.Create(ctx, version); err != nil {
		return nil, err
	}

	return uc.versionRepo.GetByID(ctx, projectID, version.ID)
}

func (uc *ModelArtifactUseCase) Update(ctx context.Context, projectID uuid.UUID, id uuid.UUID, updates map[string]interface{}) (*domain.ModelVersion, error) {
	version, err := uc.versionRepo.GetByID(ctx, projectID, id)
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

	if err := uc.versionRepo.Update(ctx, projectID, version); err != nil {
		return nil, err
	}

	return uc.versionRepo.GetByID(ctx, projectID, id)
}

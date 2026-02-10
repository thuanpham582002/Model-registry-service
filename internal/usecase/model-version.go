package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/domain"
)

type ModelVersionUseCase struct {
	repo      domain.ModelVersionRepository
	modelRepo domain.RegisteredModelRepository
}

func NewModelVersionUseCase(repo domain.ModelVersionRepository, modelRepo domain.RegisteredModelRepository) *ModelVersionUseCase {
	return &ModelVersionUseCase{repo: repo, modelRepo: modelRepo}
}

func (uc *ModelVersionUseCase) Create(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, name, description string, isDefault bool, artifactType, framework, frameworkVersion, containerImage, catalogName, uri, accessKey, secretKey string, labels map[string]string, prebuiltContainerID *uuid.UUID, createdByID *uuid.UUID, createdByEmail, updatedByEmail string) (*domain.ModelVersion, error) {
	// Verify parent model exists AND belongs to this project
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

	if err := uc.repo.Create(ctx, version); err != nil {
		return nil, err
	}

	return uc.repo.GetByID(ctx, projectID, version.ID)
}

func (uc *ModelVersionUseCase) Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.ModelVersion, error) {
	return uc.repo.GetByID(ctx, projectID, id)
}

func (uc *ModelVersionUseCase) GetByModel(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, versionID uuid.UUID) (*domain.ModelVersion, error) {
	return uc.repo.GetByModelAndVersion(ctx, projectID, modelID, versionID)
}

func (uc *ModelVersionUseCase) List(ctx context.Context, filter domain.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return uc.repo.List(ctx, filter)
}

func (uc *ModelVersionUseCase) ListByModel(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, filter domain.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	filter.ProjectID = projectID
	return uc.repo.ListByModel(ctx, modelID, filter)
}

func (uc *ModelVersionUseCase) Find(ctx context.Context, projectID uuid.UUID, name, externalID string, modelID *uuid.UUID) (*domain.ModelVersion, error) {
	return uc.repo.FindByParams(ctx, projectID, name, externalID, modelID)
}

func (uc *ModelVersionUseCase) Update(ctx context.Context, projectID uuid.UUID, id uuid.UUID, updates map[string]interface{}) (*domain.ModelVersion, error) {
	version, err := uc.repo.GetByID(ctx, projectID, id)
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

	if err := uc.repo.Update(ctx, projectID, version); err != nil {
		return nil, err
	}

	return uc.repo.GetByID(ctx, projectID, id)
}

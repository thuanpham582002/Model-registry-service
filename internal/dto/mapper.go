package dto

import (
	"model-registry-service/internal/domain"
	"time"
)

const timeFormat = time.RFC3339

func ToRegisteredModelResponse(m *domain.RegisteredModel) RegisteredModelResponse {
	resp := RegisteredModelResponse{
		ID:               m.ID,
		CreatedAt:        m.CreatedAt.Format(timeFormat),
		UpdatedAt:        m.UpdatedAt.Format(timeFormat),
		ProjectID:        m.ProjectID,
		OwnerID:          m.OwnerID,
		OwnerEmail:       m.OwnerEmail,
		Name:             m.Name,
		Slug:             m.Slug,
		Description:      m.Description,
		RegionID:         m.RegionID,
		RegionName:       m.RegionName,
		ModelType:        string(m.ModelType),
		ModelSize:        m.ModelSize,
		State:            string(m.State),
		DeploymentStatus: string(m.DeploymentStatus),
		Tags: &TagsDTO{
			Frameworks:    m.Tags.Frameworks,
			Architectures: m.Tags.Architectures,
			Tasks:         m.Tags.Tasks,
			Subjects:      m.Tags.Subjects,
		},
		Labels:        m.Labels,
		ParentModelID: m.ParentModelID,
		VersionCount:  m.VersionCount,
	}

	if m.LatestVersion != nil {
		lv := ToModelVersionResponse(m.LatestVersion)
		resp.LatestVersion = &lv
	}
	if m.DefaultVersion != nil {
		dv := ToModelVersionResponse(m.DefaultVersion)
		resp.DefaultVersion = &dv
	}

	return resp
}

func ToModelVersionResponse(v *domain.ModelVersion) ModelVersionResponse {
	return ModelVersionResponse{
		ID:                    v.ID,
		CreatedAt:             v.CreatedAt.Format(timeFormat),
		UpdatedAt:             v.UpdatedAt.Format(timeFormat),
		RegisteredModelID:     v.RegisteredModelID,
		Name:                  v.Name,
		Description:           v.Description,
		IsDefault:             v.IsDefault,
		State:                 string(v.State),
		Status:                string(v.Status),
		CreatedByID:           v.CreatedByID,
		UpdatedByID:           v.UpdatedByID,
		CreatedByEmail:        v.CreatedByEmail,
		UpdatedByEmail:        v.UpdatedByEmail,
		ArtifactType:          string(v.ArtifactType),
		ModelFramework:        v.ModelFramework,
		ModelFrameworkVersion: v.ModelFrameworkVersion,
		ContainerImage:        v.ContainerImage,
		ModelCatalogName:      v.ModelCatalogName,
		URI:                   v.URI,
		Labels:                v.Labels,
		PrebuiltContainerID:   v.PrebuiltContainerID,
	}
}

func ToModelArtifactResponse(v *domain.ModelVersion) ModelArtifactResponse {
	return ModelArtifactResponse{
		ID:                    v.ID,
		CreatedAt:             v.CreatedAt.Format(timeFormat),
		UpdatedAt:             v.UpdatedAt.Format(timeFormat),
		RegisteredModelID:     v.RegisteredModelID,
		Name:                  v.Name,
		ArtifactType:          string(v.ArtifactType),
		ModelFramework:        v.ModelFramework,
		ModelFrameworkVersion: v.ModelFrameworkVersion,
		ContainerImage:        v.ContainerImage,
		URI:                   v.URI,
		Labels:                v.Labels,
		PrebuiltContainerID:   v.PrebuiltContainerID,
	}
}

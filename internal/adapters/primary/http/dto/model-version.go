package dto

import (
	"github.com/google/uuid"
)

type CreateModelVersionRequest struct {
	Name                  string            `json:"name" binding:"required,max=100"`
	Description           string            `json:"description"`
	IsDefault             *bool             `json:"is_default"`
	ArtifactType          string            `json:"artifact_type"`
	ModelFramework        string            `json:"model_framework" binding:"required"`
	ModelFrameworkVersion string            `json:"model_framework_version" binding:"required"`
	ContainerImage        string            `json:"container_image"`
	ModelCatalogName      string            `json:"model_catalog_name"`
	URI                   string            `json:"uri" binding:"required"`
	AccessKey             string            `json:"access_key"`
	SecretKey             string            `json:"secret_key"`
	Labels                map[string]string `json:"labels"`
	PrebuiltContainerID   *uuid.UUID        `json:"prebuilt_container_id"`
	CreatedByEmail        string            `json:"created_by_email"`
	UpdatedByEmail        string            `json:"updated_by_email"`
}

type UpdateModelVersionRequest struct {
	Name                  *string           `json:"name"`
	Description           *string           `json:"description"`
	IsDefault             *bool             `json:"is_default"`
	State                 *string           `json:"state"`
	Status                *string           `json:"status"`
	ArtifactType          *string           `json:"artifact_type"`
	ModelFramework        *string           `json:"model_framework"`
	ModelFrameworkVersion *string           `json:"model_framework_version"`
	ContainerImage        *string           `json:"container_image"`
	URI                   *string           `json:"uri"`
	Labels                map[string]string `json:"labels"`
}

type ModelVersionResponse struct {
	ID                    uuid.UUID         `json:"id"`
	CreatedAt             string            `json:"created_at"`
	UpdatedAt             string            `json:"updated_at"`
	RegisteredModelID     uuid.UUID         `json:"registered_model_id"`
	Name                  string            `json:"name"`
	Description           string            `json:"description"`
	IsDefault             bool              `json:"is_default"`
	State                 string            `json:"state"`
	Status                string            `json:"status"`
	CreatedByID           *uuid.UUID        `json:"created_by_id"`
	UpdatedByID           *uuid.UUID        `json:"updated_by_id"`
	CreatedByEmail        string            `json:"created_by_email,omitempty"`
	UpdatedByEmail        string            `json:"updated_by_email,omitempty"`
	ArtifactType          string            `json:"artifact_type"`
	ModelFramework        string            `json:"model_framework"`
	ModelFrameworkVersion string            `json:"model_framework_version"`
	ContainerImage        string            `json:"container_image"`
	ModelCatalogName      string            `json:"model_catalog_name"`
	URI                   string            `json:"uri"`
	Labels                map[string]string `json:"labels"`
	PrebuiltContainerID   *uuid.UUID        `json:"prebuilt_container_id"`
}

type ListModelVersionsResponse struct {
	Items      []ModelVersionResponse `json:"items"`
	Total      int                    `json:"total"`
	PageSize   int                    `json:"page_size"`
	NextOffset int                    `json:"next_offset"`
}

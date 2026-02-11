package dto

import (
	"github.com/google/uuid"
)

type CreateRegisteredModelRequest struct {
	Name          string            `json:"name" binding:"required,max=100"`
	Description   string            `json:"description"`
	OwnerEmail    string            `json:"owner_email"`
	ModelType     string            `json:"model_type"`
	Tags          *TagsDTO          `json:"tags"`
	Labels        map[string]string `json:"labels"`
	ParentModelID *uuid.UUID        `json:"parent_model_id"`
}

type UpdateRegisteredModelRequest struct {
	Name             *string           `json:"name"`
	Description      *string           `json:"description"`
	ModelType        *string           `json:"model_type"`
	ModelSize        *int64            `json:"model_size"`
	State            *string           `json:"state"`
	DeploymentStatus *string           `json:"deployment_status"`
	Tags             *TagsDTO          `json:"tags"`
	Labels           map[string]string `json:"labels"`
}

type TagsDTO struct {
	Frameworks    []string `json:"frameworks"`
	Architectures []string `json:"architectures"`
	Tasks         []string `json:"tasks"`
	Subjects      []string `json:"subjects"`
}

type RegisteredModelResponse struct {
	ID               uuid.UUID         `json:"id"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
	ProjectID        uuid.UUID         `json:"project_id"`
	OwnerID          *uuid.UUID        `json:"owner_id"`
	OwnerEmail       string            `json:"owner_email,omitempty"`
	Name             string            `json:"name"`
	Slug             string            `json:"slug"`
	Description      string            `json:"description"`
	ModelType        string            `json:"model_type"`
	ModelSize        int64             `json:"model_size"`
	State            string            `json:"state"`
	DeploymentStatus string            `json:"deployment_status"`
	Tags             *TagsDTO          `json:"tags"`
	Labels           map[string]string `json:"labels"`
	ParentModelID    *uuid.UUID        `json:"parent_model_id"`
	VersionCount     int               `json:"version_count"`
	LatestVersion    *ModelVersionResponse `json:"latest_version,omitempty"`
	DefaultVersion   *ModelVersionResponse `json:"default_version,omitempty"`
}

type ListRegisteredModelsResponse struct {
	Items      []RegisteredModelResponse `json:"items"`
	Total      int                       `json:"total"`
	PageSize   int                       `json:"page_size"`
	NextOffset int                       `json:"next_offset"`
}

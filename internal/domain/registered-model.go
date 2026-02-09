package domain

import (
	"time"

	"github.com/google/uuid"
)

type ModelType string

const (
	ModelTypeCustomTrain ModelType = "CUSTOMTRAIN"
	ModelTypePretrain    ModelType = "PRETRAIN"
)

type ModelState string

const (
	ModelStateLive     ModelState = "LIVE"
	ModelStateArchived ModelState = "ARCHIVED"
)

type DeploymentStatus string

const (
	DeploymentStatusUndeployed DeploymentStatus = "UNDEPLOYED"
	DeploymentStatusDeployed   DeploymentStatus = "DEPLOYED"
	DeploymentStatusFailed     DeploymentStatus = "FAILED"
)

type Tags struct {
	Frameworks    []string `json:"frameworks"`
	Architectures []string `json:"architectures"`
	Tasks         []string `json:"tasks"`
	Subjects      []string `json:"subjects"`
}

type RegisteredModel struct {
	ID               uuid.UUID        `json:"id"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	ProjectID        uuid.UUID        `json:"project_id"`
	OwnerID          *uuid.UUID       `json:"owner_id"`
	Name             string           `json:"name"`
	Slug             string           `json:"slug"`
	Description      string           `json:"description"`
	RegionID         uuid.UUID        `json:"region_id"`
	ModelType        ModelType        `json:"model_type"`
	ModelSize        int64            `json:"model_size"`
	State            ModelState       `json:"state"`
	DeploymentStatus DeploymentStatus `json:"deployment_status"`
	Tags             Tags             `json:"tags"`
	Labels           map[string]string `json:"labels"`
	ParentModelID    *uuid.UUID       `json:"parent_model_id"`

	// Computed fields (populated by repository)
	OwnerEmail     string        `json:"owner_email,omitempty"`
	RegionName     string        `json:"region_name,omitempty"`
	VersionCount   int           `json:"version_count"`
	LatestVersion  *ModelVersion `json:"latest_version,omitempty"`
	DefaultVersion *ModelVersion `json:"default_version,omitempty"`
}

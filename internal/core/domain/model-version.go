package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type VersionStatus string

const (
	VersionStatusPending VersionStatus = "PENDING"
	VersionStatusReady   VersionStatus = "READY"
	VersionStatusFailed  VersionStatus = "FAILED"
)

type ArtifactType string

const (
	ArtifactTypeModel   ArtifactType = "model-artifact"
	ArtifactTypeDoc     ArtifactType = "doc-artifact"
	ArtifactTypeDataset ArtifactType = "dataset-artifact"
)

// KServe supported model frameworks
// See: https://kserve.github.io/website/docs/model-serving/predictive-inference/frameworks/overview
var SupportedFrameworks = map[string]bool{
	"sklearn":     true,
	"xgboost":     true,
	"tensorflow":  true,
	"pytorch":     true,
	"onnx":        true,
	"triton":      true,
	"lightgbm":    true,
	"paddle":      true,
	"mlflow":      true,
	"huggingface": true,
	"pmml":        true,
}

func ValidateModelFramework(framework string) error {
	if framework == "" {
		return nil
	}
	if !SupportedFrameworks[strings.ToLower(framework)] {
		return ErrUnsupportedFramework
	}
	return nil
}

type ModelVersion struct {
	ID                    uuid.UUID     `json:"id"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
	RegisteredModelID     uuid.UUID     `json:"registered_model_id"`
	Name                  string        `json:"name"`
	Description           string        `json:"description"`
	IsDefault             bool          `json:"is_default"`
	State                 ModelState    `json:"state"`
	Status                VersionStatus `json:"status"`
	CreatedByID           *uuid.UUID    `json:"created_by_id"`
	UpdatedByID           *uuid.UUID    `json:"updated_by_id"`
	ArtifactType          ArtifactType  `json:"artifact_type"`
	ModelFramework        string        `json:"model_framework"`
	ModelFrameworkVersion string        `json:"model_framework_version"`
	ContainerImage        string        `json:"container_image"`
	ModelCatalogName      string        `json:"model_catalog_name"`
	URI                   string        `json:"uri"`
	AccessKey             string        `json:"access_key"`
	SecretKey             string        `json:"secret_key"`
	Labels                map[string]string `json:"labels"`
	PrebuiltContainerID   *uuid.UUID    `json:"prebuilt_container_id"`

	// Computed fields
	CreatedByEmail string `json:"created_by_email,omitempty"`
	UpdatedByEmail string `json:"updated_by_email,omitempty"`
}

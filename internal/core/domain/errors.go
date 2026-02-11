package domain

import "errors"

// ============================================================================
// Model Registry Errors
// ============================================================================

var (
	ErrModelNotFound       = errors.New("registered model not found")
	ErrModelNameConflict   = errors.New("model with this name already exists in the project")
	ErrVersionNotFound     = errors.New("model version not found")
	ErrVersionNameConflict = errors.New("version with this name already exists for this model")
	ErrArtifactNotFound    = errors.New("model artifact not found")
	ErrInvalidModelName    = errors.New("model name is required")
	ErrMissingProjectID    = errors.New("project ID is required (Project-ID header)")
	ErrCannotDeleteModel   = errors.New("cannot delete model: must be archived with no READY versions")
)

// ============================================================================
// Serving Errors
// ============================================================================

// Not found errors
var (
	ErrServingEnvNotFound       = errors.New("serving environment not found")
	ErrInferenceServiceNotFound = errors.New("inference service not found")
	ErrServeModelNotFound       = errors.New("serve model not found")
)

// Conflict errors
var (
	ErrServingEnvNameConflict       = errors.New("serving environment with this name already exists in the project")
	ErrInferenceServiceNameConflict = errors.New("inference service with this name already exists")
)

// Validation errors
var (
	ErrInvalidServingEnvName       = errors.New("serving environment name is required")
	ErrInvalidServingEnvID         = errors.New("serving environment ID is required")
	ErrInvalidInferenceServiceName = errors.New("inference service name is required")
	ErrInvalidInferenceServiceID   = errors.New("inference service ID is required")
	ErrInvalidModelID              = errors.New("registered model ID is required")
	ErrInvalidVersionID            = errors.New("model version ID is required")
	ErrInvalidState                = errors.New("invalid state")
)

// Business rule errors
var (
	ErrModelHasActiveDeployments  = errors.New("model has active deployments")
	ErrCannotDeleteDeployed       = errors.New("cannot delete deployed inference service")
	ErrVersionNotReady            = errors.New("model version is not ready for deployment")
	ErrServingEnvHasDeployments   = errors.New("serving environment has active inference services")
	ErrInferenceServiceNotHealthy = errors.New("inference service is not healthy")
)

// ============================================================================
// Traffic Management Errors
// ============================================================================

// Not found errors
var (
	ErrTrafficConfigNotFound  = errors.New("traffic config not found")
	ErrTrafficVariantNotFound = errors.New("traffic variant not found")
)

// Validation errors
var (
	ErrInvalidVariantName   = errors.New("variant name is required")
	ErrInvalidTrafficWeight = errors.New("traffic weight must be between 0 and 100")
	ErrWeightSumExceeds100  = errors.New("total variant weights cannot exceed 100")
)

// Conflict errors
var (
	ErrVariantAlreadyExists = errors.New("variant with this name already exists")
	ErrCanaryAlreadyExists  = errors.New("canary variant already exists")
)

// Business rule errors
var (
	ErrNoStableVariant       = errors.New("no stable variant configured")
	ErrCannotPromoteInactive = errors.New("cannot promote inactive variant")
	ErrCannotPromoteStable   = errors.New("cannot promote stable variant to itself")
	ErrCannotDeleteStable    = errors.New("cannot delete stable variant")
)

// ============================================================================
// Virtual Model Errors
// ============================================================================

// Not found errors
var (
	ErrVirtualModelNotFound = errors.New("virtual model not found")
	ErrBackendNotFound      = errors.New("backend not found")
)

// Validation errors
var (
	ErrInvalidVirtualModelName = errors.New("virtual model name is required")
	ErrInvalidBackendName      = errors.New("backend name is required")
	ErrInvalidPriority         = errors.New("priority must be >= 0")
)

// Conflict errors
var (
	ErrVirtualModelExists   = errors.New("virtual model with this name already exists")
	ErrBackendAlreadyExists = errors.New("backend already exists for this virtual model")
)

// ============================================================================
// Metrics Errors
// ============================================================================

var (
	ErrMetricNotFound        = errors.New("metric not found")
	ErrInvalidTimeRange      = errors.New("invalid time range")
	ErrPrometheusQueryFailed = errors.New("prometheus query failed")
)

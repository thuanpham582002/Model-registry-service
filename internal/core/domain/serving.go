package domain

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Value Objects
// ============================================================================

// InferenceServiceState represents the state of an InferenceService
type InferenceServiceState string

const (
	ISStateDeployed   InferenceServiceState = "DEPLOYED"
	ISStateUndeployed InferenceServiceState = "UNDEPLOYED"
)

// IsValid checks if the state is valid
func (s InferenceServiceState) IsValid() bool {
	return s == ISStateDeployed || s == ISStateUndeployed
}

// ServeModelState represents the state of a ServeModel
type ServeModelState string

const (
	ServeStatePending ServeModelState = "PENDING"
	ServeStateRunning ServeModelState = "RUNNING"
	ServeStateFailed  ServeModelState = "FAILED"
)

// IsValid checks if the state is valid
func (s ServeModelState) IsValid() bool {
	return s == ServeStatePending || s == ServeStateRunning || s == ServeStateFailed
}

// ============================================================================
// Entities
// ============================================================================

// ServingEnvironment represents a namespace/environment for deployments
type ServingEnvironment struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProjectID   uuid.UUID `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ExternalID  string    `json:"external_id"` // K8s namespace name if different
}

// NewServingEnvironment creates a new ServingEnvironment with validation
func NewServingEnvironment(projectID uuid.UUID, name, description string) (*ServingEnvironment, error) {
	if name == "" {
		return nil, ErrInvalidServingEnvName
	}
	if projectID == uuid.Nil {
		return nil, ErrMissingProjectID
	}

	now := time.Now()
	return &ServingEnvironment{
		ID:          uuid.New(),
		CreatedAt:   now,
		UpdatedAt:   now,
		ProjectID:   projectID,
		Name:        name,
		Description: description,
	}, nil
}

// Update updates the serving environment fields
func (e *ServingEnvironment) Update(name, description *string) {
	if name != nil {
		e.Name = *name
	}
	if description != nil {
		e.Description = *description
	}
	e.UpdatedAt = time.Now()
}

// InferenceService represents a deployed model serving endpoint
type InferenceService struct {
	ID                   uuid.UUID             `json:"id"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	ProjectID            uuid.UUID             `json:"project_id"`
	Name                 string                `json:"name"`
	ExternalID           string                `json:"external_id"` // K8s resource UID
	ServingEnvironmentID uuid.UUID             `json:"serving_environment_id"`
	RegisteredModelID    uuid.UUID             `json:"registered_model_id"`
	DesiredState         InferenceServiceState `json:"desired_state"`
	CurrentState         InferenceServiceState `json:"current_state"`
	Runtime              string                `json:"runtime"` // e.g., "kserve"
	URL                  string                `json:"url"`
	LastError            string                `json:"last_error"`
	Labels               map[string]string     `json:"labels"`

	// Computed/joined fields
	ServingEnvironmentName string `json:"serving_environment_name,omitempty"`
	RegisteredModelName    string `json:"registered_model_name,omitempty"`

	// Related entities (loaded via serve_model junction)
	ServedModels []*ServeModel `json:"served_models,omitempty"`
}

// NewInferenceService creates a new InferenceService with validation
func NewInferenceService(
	projectID uuid.UUID,
	name string,
	envID uuid.UUID,
	modelID uuid.UUID,
) (*InferenceService, error) {
	if name == "" {
		return nil, ErrInvalidInferenceServiceName
	}
	if projectID == uuid.Nil {
		return nil, ErrMissingProjectID
	}
	if envID == uuid.Nil {
		return nil, ErrInvalidServingEnvID
	}
	if modelID == uuid.Nil {
		return nil, ErrInvalidModelID
	}

	now := time.Now()
	return &InferenceService{
		ID:                   uuid.New(),
		CreatedAt:            now,
		UpdatedAt:            now,
		ProjectID:            projectID,
		Name:                 name,
		ServingEnvironmentID: envID,
		RegisteredModelID:    modelID,
		DesiredState:         ISStateDeployed,
		CurrentState:         ISStateUndeployed,
		Runtime:              "kserve",
		Labels:               make(map[string]string),
	}, nil
}

// Deploy marks the service for deployment
func (is *InferenceService) Deploy() {
	is.DesiredState = ISStateDeployed
	is.UpdatedAt = time.Now()
}

// Undeploy marks the service for undeployment
func (is *InferenceService) Undeploy() {
	is.DesiredState = ISStateUndeployed
	is.UpdatedAt = time.Now()
}

// MarkDeployed updates current state to deployed with endpoint URL
func (is *InferenceService) MarkDeployed(url string) {
	is.CurrentState = ISStateDeployed
	is.URL = url
	is.LastError = ""
	is.UpdatedAt = time.Now()
}

// MarkUndeployed updates current state to undeployed
func (is *InferenceService) MarkUndeployed() {
	is.CurrentState = ISStateUndeployed
	is.URL = ""
	is.UpdatedAt = time.Now()
}

// MarkFailed records deployment failure
func (is *InferenceService) MarkFailed(err string) {
	is.LastError = err
	is.UpdatedAt = time.Now()
}

// IsDeployed returns true if currently deployed
func (is *InferenceService) IsDeployed() bool {
	return is.CurrentState == ISStateDeployed
}

// NeedsReconciliation returns true if desired state differs from current
func (is *InferenceService) NeedsReconciliation() bool {
	return is.DesiredState != is.CurrentState
}

// SetExternalID sets the K8s resource UID
func (is *InferenceService) SetExternalID(externalID string) {
	is.ExternalID = externalID
	is.UpdatedAt = time.Now()
}

// ServeModel represents the association between an InferenceService and a ModelVersion
type ServeModel struct {
	ID                 uuid.UUID       `json:"id"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	ProjectID          uuid.UUID       `json:"project_id"`
	InferenceServiceID uuid.UUID       `json:"inference_service_id"`
	ModelVersionID     uuid.UUID       `json:"model_version_id"`
	LastKnownState     ServeModelState `json:"last_known_state"`

	// Computed/joined fields
	InferenceServiceName string `json:"inference_service_name,omitempty"`
	ModelVersionName     string `json:"model_version_name,omitempty"`
}

// NewServeModel creates a new ServeModel with validation
func NewServeModel(
	projectID uuid.UUID,
	inferenceServiceID uuid.UUID,
	modelVersionID uuid.UUID,
) (*ServeModel, error) {
	if projectID == uuid.Nil {
		return nil, ErrMissingProjectID
	}
	if inferenceServiceID == uuid.Nil {
		return nil, ErrInvalidInferenceServiceID
	}
	if modelVersionID == uuid.Nil {
		return nil, ErrInvalidVersionID
	}

	now := time.Now()
	return &ServeModel{
		ID:                 uuid.New(),
		CreatedAt:          now,
		UpdatedAt:          now,
		ProjectID:          projectID,
		InferenceServiceID: inferenceServiceID,
		ModelVersionID:     modelVersionID,
		LastKnownState:     ServeStatePending,
	}, nil
}

// SetState updates the state
func (sm *ServeModel) SetState(state ServeModelState) {
	sm.LastKnownState = state
	sm.UpdatedAt = time.Now()
}

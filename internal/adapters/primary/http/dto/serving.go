package dto

import (
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
)

// ============================================================================
// Serving Environment DTOs
// ============================================================================

type CreateServingEnvironmentRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description"`
	ExternalID  string `json:"external_id"`
}

type UpdateServingEnvironmentRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ExternalID  *string `json:"external_id"`
}

type ServingEnvironmentResponse struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ExternalID  string    `json:"external_id,omitempty"`
}

type ListServingEnvironmentsResponse struct {
	Items      []ServingEnvironmentResponse `json:"items"`
	Total      int                          `json:"total"`
	PageSize   int                          `json:"page_size"`
	NextOffset int                          `json:"next_offset"`
}

func ToServingEnvironmentResponse(env *domain.ServingEnvironment) ServingEnvironmentResponse {
	return ServingEnvironmentResponse{
		ID:          env.ID,
		CreatedAt:   env.CreatedAt,
		UpdatedAt:   env.UpdatedAt,
		Name:        env.Name,
		Description: env.Description,
		ExternalID:  env.ExternalID,
	}
}

// ============================================================================
// Inference Service DTOs
// ============================================================================

type CreateInferenceServiceRequest struct {
	Name                 string            `json:"name" binding:"required,max=100"`
	ServingEnvironmentID uuid.UUID         `json:"serving_environment_id" binding:"required"`
	RegisteredModelID    uuid.UUID         `json:"registered_model_id" binding:"required"`
	ModelVersionID       *uuid.UUID        `json:"model_version_id"`
	Runtime              string            `json:"runtime"`
	Labels               map[string]string `json:"labels"`
}

type UpdateInferenceServiceRequest struct {
	Name           *string           `json:"name"`
	DesiredState   *string           `json:"desired_state"`
	CurrentState   *string           `json:"current_state"`
	ModelVersionID *uuid.UUID        `json:"model_version_id"`
	ExternalID     *string           `json:"external_id"`
	URL            *string           `json:"url"`
	LastError      *string           `json:"last_error"`
	Labels         map[string]string `json:"labels"`
}

type InferenceServiceResponse struct {
	ID                     uuid.UUID         `json:"id"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
	Name                   string            `json:"name"`
	ExternalID             string            `json:"external_id,omitempty"`
	ServingEnvironmentID   uuid.UUID         `json:"serving_environment_id"`
	ServingEnvironmentName string            `json:"serving_environment_name,omitempty"`
	RegisteredModelID      uuid.UUID         `json:"registered_model_id"`
	RegisteredModelName    string            `json:"registered_model_name,omitempty"`
	ModelVersionID         *uuid.UUID        `json:"model_version_id,omitempty"`
	ModelVersionName       string            `json:"model_version_name,omitempty"`
	DesiredState           string            `json:"desired_state"`
	CurrentState           string            `json:"current_state"`
	Runtime                string            `json:"runtime"`
	URL                    string            `json:"url,omitempty"`
	LastError              string            `json:"last_error,omitempty"`
	Labels                 map[string]string `json:"labels"`
}

type ListInferenceServicesResponse struct {
	Items      []InferenceServiceResponse `json:"items"`
	Total      int                        `json:"total"`
	PageSize   int                        `json:"page_size"`
	NextOffset int                        `json:"next_offset"`
}

func ToInferenceServiceResponse(isvc *domain.InferenceService) InferenceServiceResponse {
	labels := isvc.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	return InferenceServiceResponse{
		ID:                     isvc.ID,
		CreatedAt:              isvc.CreatedAt,
		UpdatedAt:              isvc.UpdatedAt,
		Name:                   isvc.Name,
		ExternalID:             isvc.ExternalID,
		ServingEnvironmentID:   isvc.ServingEnvironmentID,
		ServingEnvironmentName: isvc.ServingEnvironmentName,
		RegisteredModelID:      isvc.RegisteredModelID,
		RegisteredModelName:    isvc.RegisteredModelName,
		ModelVersionID:         isvc.ModelVersionID,
		ModelVersionName:       isvc.ModelVersionName,
		DesiredState:           string(isvc.DesiredState),
		CurrentState:           string(isvc.CurrentState),
		Runtime:                isvc.Runtime,
		URL:                    isvc.URL,
		LastError:              isvc.LastError,
		Labels:                 labels,
	}
}

// ============================================================================
// Serve Model DTOs
// ============================================================================

type CreateServeModelRequest struct {
	InferenceServiceID uuid.UUID `json:"inference_service_id" binding:"required"`
	ModelVersionID     uuid.UUID `json:"model_version_id" binding:"required"`
}

type ServeModelResponse struct {
	ID                   uuid.UUID `json:"id"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	InferenceServiceID   uuid.UUID `json:"inference_service_id"`
	InferenceServiceName string    `json:"inference_service_name,omitempty"`
	ModelVersionID       uuid.UUID `json:"model_version_id"`
	ModelVersionName     string    `json:"model_version_name,omitempty"`
	LastKnownState       string    `json:"last_known_state"`
}

type ListServeModelsResponse struct {
	Items      []ServeModelResponse `json:"items"`
	Total      int                  `json:"total"`
	PageSize   int                  `json:"page_size"`
	NextOffset int                  `json:"next_offset"`
}

func ToServeModelResponse(sm *domain.ServeModel) ServeModelResponse {
	return ServeModelResponse{
		ID:                   sm.ID,
		CreatedAt:            sm.CreatedAt,
		UpdatedAt:            sm.UpdatedAt,
		InferenceServiceID:   sm.InferenceServiceID,
		InferenceServiceName: sm.InferenceServiceName,
		ModelVersionID:       sm.ModelVersionID,
		ModelVersionName:     sm.ModelVersionName,
		LastKnownState:       string(sm.LastKnownState),
	}
}

// ============================================================================
// Deploy DTOs
// ============================================================================

type DeployModelRequest struct {
	RegisteredModelID    uuid.UUID         `json:"registered_model_id" binding:"required"`
	ModelVersionID       *uuid.UUID        `json:"model_version_id"`
	ServingEnvironmentID uuid.UUID         `json:"serving_environment_id" binding:"required"`
	Name                 string            `json:"name"`
	Labels               map[string]string `json:"labels"`
}

type DeployModelResponse struct {
	InferenceService InferenceServiceResponse `json:"inference_service"`
	Status           string                   `json:"status"`
	Message          string                   `json:"message"`
}

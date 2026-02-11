package dto

import (
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
)

// ============================================================================
// Request DTOs
// ============================================================================

// CreateTrafficConfigRequest represents a request to create a traffic config
type CreateTrafficConfigRequest struct {
	InferenceServiceID uuid.UUID `json:"inference_service_id" binding:"required"`
	Strategy           string    `json:"strategy"` // canary, ab_test, shadow, blue_green
	StableVersionID    uuid.UUID `json:"stable_version_id" binding:"required"`
}

// StartCanaryRequest represents a request to start a canary deployment
type StartCanaryRequest struct {
	ModelVersionID uuid.UUID `json:"model_version_id" binding:"required"`
	InitialWeight  int       `json:"initial_weight"` // 5-50, default 10
}

// UpdateCanaryWeightRequest represents a request to update canary weight
type UpdateCanaryWeightRequest struct {
	Weight int `json:"weight" binding:"required,min=0,max=100"`
}

// AddVariantRequest represents a request to add a variant
type AddVariantRequest struct {
	VariantName    string    `json:"variant_name" binding:"required"`
	ModelVersionID uuid.UUID `json:"model_version_id" binding:"required"`
	Weight         int       `json:"weight"` // 0-100, default 0
}

// UpdateVariantRequest represents a request to update a variant
type UpdateVariantRequest struct {
	Weight int `json:"weight" binding:"min=0,max=100"`
}

// BulkUpdateWeightsRequest represents a request to update multiple variant weights
type BulkUpdateWeightsRequest struct {
	Weights map[string]int `json:"weights" binding:"required"` // variant_name -> weight
}

// ============================================================================
// Response DTOs
// ============================================================================

// TrafficConfigResponse represents a traffic config response
type TrafficConfigResponse struct {
	ID                   uuid.UUID                `json:"id"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
	InferenceServiceID   uuid.UUID                `json:"inference_service_id"`
	InferenceServiceName string                   `json:"inference_service_name,omitempty"`
	Strategy             string                   `json:"strategy"`
	Status               string                   `json:"status"`
	AIGatewayRouteName   string                   `json:"ai_gateway_route_name,omitempty"`
	Variants             []TrafficVariantResponse `json:"variants"`
}

// TrafficVariantResponse represents a traffic variant response
type TrafficVariantResponse struct {
	ID               uuid.UUID `json:"id"`
	VariantName      string    `json:"variant_name"`
	ModelVersionID   uuid.UUID `json:"model_version_id"`
	ModelVersionName string    `json:"model_version_name,omitempty"`
	Weight           int       `json:"weight"`
	Status           string    `json:"status"`
	KServeISVCName   string    `json:"kserve_isvc_name,omitempty"`
}

// ListTrafficConfigsResponse represents a list of traffic configs
type ListTrafficConfigsResponse struct {
	Items      []TrafficConfigResponse `json:"items"`
	Total      int                     `json:"total"`
	PageSize   int                     `json:"page_size"`
	NextOffset int                     `json:"next_offset"`
}

// ListVariantsResponse represents a list of variants
type ListVariantsResponse struct {
	ConfigID uuid.UUID                `json:"config_id"`
	Variants []TrafficVariantResponse `json:"variants"`
	Total    int                      `json:"total"`
}

// ============================================================================
// Converters
// ============================================================================

// ToTrafficConfigResponse converts a domain TrafficConfig to response DTO
func ToTrafficConfigResponse(config *domain.TrafficConfig) TrafficConfigResponse {
	resp := TrafficConfigResponse{
		ID:                   config.ID,
		CreatedAt:            config.CreatedAt,
		UpdatedAt:            config.UpdatedAt,
		InferenceServiceID:   config.InferenceServiceID,
		InferenceServiceName: config.InferenceServiceName,
		Strategy:             string(config.Strategy),
		Status:               config.Status,
		AIGatewayRouteName:   config.AIGatewayRouteName,
		Variants:             make([]TrafficVariantResponse, 0),
	}

	for _, v := range config.Variants {
		resp.Variants = append(resp.Variants, TrafficVariantResponse{
			ID:               v.ID,
			VariantName:      v.VariantName,
			ModelVersionID:   v.ModelVersionID,
			ModelVersionName: v.ModelVersionName,
			Weight:           v.Weight,
			Status:           string(v.Status),
			KServeISVCName:   v.KServeISVCName,
		})
	}

	return resp
}

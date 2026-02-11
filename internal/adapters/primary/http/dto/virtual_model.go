package dto

import (
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
)

// ============================================================================
// Request DTOs
// ============================================================================

// CreateVirtualModelRequest represents a request to create a virtual model
type CreateVirtualModelRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateVirtualModelRequest represents a request to update a virtual model
type UpdateVirtualModelRequest struct {
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

// AddBackendRequest represents a request to add a backend to a virtual model
type AddBackendRequest struct {
	AIServiceBackendName string  `json:"ai_service_backend_name" binding:"required"`
	AIServiceBackendNS   string  `json:"ai_service_backend_namespace"`
	ModelNameOverride    *string `json:"model_name_override"` // nil = use virtual model name
	Weight               int     `json:"weight"`              // 0-100
	Priority             int     `json:"priority"`            // 0 = primary, 1+ = fallback
}

// UpdateBackendRequest represents a request to update a backend
type UpdateBackendRequest struct {
	Weight   int `json:"weight" binding:"min=0,max=100"`
	Priority int `json:"priority" binding:"min=0"`
}

// ============================================================================
// Response DTOs
// ============================================================================

// VirtualModelResponse represents a virtual model response
type VirtualModelResponse struct {
	ID                 uuid.UUID                `json:"id"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
	Name               string                   `json:"name"`
	Description        string                   `json:"description,omitempty"`
	AIGatewayRouteName string                   `json:"ai_gateway_route_name,omitempty"`
	Status             string                   `json:"status"`
	Backends           []VirtualModelBackendRes `json:"backends"`
}

// VirtualModelBackendRes represents a virtual model backend response
type VirtualModelBackendRes struct {
	ID                        uuid.UUID `json:"id"`
	AIServiceBackendName      string    `json:"ai_service_backend_name"`
	AIServiceBackendNamespace string    `json:"ai_service_backend_namespace,omitempty"`
	ModelNameOverride         *string   `json:"model_name_override,omitempty"`
	Weight                    int       `json:"weight"`
	Priority                  int       `json:"priority"`
	Status                    string    `json:"status"`
}

// ListVirtualModelsResponse represents a list of virtual models
type ListVirtualModelsResponse struct {
	Items []VirtualModelResponse `json:"items"`
	Total int                    `json:"total"`
}

// ============================================================================
// Converters
// ============================================================================

// ToVirtualModelResponse converts a domain VirtualModel to response DTO
func ToVirtualModelResponse(vm *domain.VirtualModel) VirtualModelResponse {
	resp := VirtualModelResponse{
		ID:                 vm.ID,
		CreatedAt:          vm.CreatedAt,
		UpdatedAt:          vm.UpdatedAt,
		Name:               vm.Name,
		Description:        vm.Description,
		AIGatewayRouteName: vm.AIGatewayRouteName,
		Status:             vm.Status,
		Backends:           make([]VirtualModelBackendRes, 0),
	}

	for _, b := range vm.Backends {
		resp.Backends = append(resp.Backends, VirtualModelBackendRes{
			ID:                        b.ID,
			AIServiceBackendName:      b.AIServiceBackendName,
			AIServiceBackendNamespace: b.AIServiceBackendNamespace,
			ModelNameOverride:         b.ModelNameOverride,
			Weight:                    b.Weight,
			Priority:                  b.Priority,
			Status:                    b.Status,
		})
	}

	return resp
}

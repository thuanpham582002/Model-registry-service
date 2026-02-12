package dto

import (
	"time"

	ports "model-registry-service/internal/core/ports/output"
)

// ============================================================================
// Request DTOs
// ============================================================================

// CreateAIServiceBackendRequest represents the request to create an AIServiceBackend
type CreateAIServiceBackendRequest struct {
	Name           string            `json:"name" binding:"required"`
	Namespace      string            `json:"namespace"`
	Schema         string            `json:"schema" binding:"required,oneof=OpenAI Anthropic AWSBedrock GoogleAI AzureOpenAI"`
	BackendRef     BackendRefRequest `json:"backend_ref" binding:"required"`
	HeaderMutation *HeaderMutation   `json:"header_mutation"`
	Labels         map[string]string `json:"labels"`
}

// BackendRefRequest represents a reference to an Envoy Gateway Backend
type BackendRefRequest struct {
	Name      string `json:"name" binding:"required"`
	Namespace string `json:"namespace"`
}

// HeaderMutation configures header transformations
type HeaderMutation struct {
	Set    []HTTPHeader `json:"set"`
	Remove []string     `json:"remove"`
}

// HTTPHeader represents a single HTTP header
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// UpdateAIServiceBackendRequest represents the request to update an AIServiceBackend
type UpdateAIServiceBackendRequest struct {
	Schema         *string           `json:"schema"`
	HeaderMutation *HeaderMutation   `json:"header_mutation"`
	Labels         map[string]string `json:"labels"`
}

// ============================================================================
// Response DTOs
// ============================================================================

// AIServiceBackendResponse represents an AIServiceBackend in API responses
type AIServiceBackendResponse struct {
	Name           string             `json:"name"`
	Namespace      string             `json:"namespace"`
	Schema         string             `json:"schema"`
	BackendRef     BackendRefResponse `json:"backend_ref"`
	HeaderMutation *HeaderMutation    `json:"header_mutation,omitempty"`
	Labels         map[string]string  `json:"labels,omitempty"`
	Status         *BackendStatus     `json:"status,omitempty"`
	CreatedAt      *time.Time         `json:"created_at,omitempty"`
}

// BackendRefResponse represents a Backend reference in responses
type BackendRefResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Group     string `json:"group"`
	Kind      string `json:"kind"`
}

// BackendStatus represents the status of an AIServiceBackend
type BackendStatus struct {
	Ready      bool     `json:"ready"`
	Conditions []string `json:"conditions,omitempty"`
}

// ListAIServiceBackendsResponse represents the list response
type ListAIServiceBackendsResponse struct {
	Items []AIServiceBackendResponse `json:"items"`
	Total int                        `json:"total"`
}

// ============================================================================
// Converters
// ============================================================================

// ToAIServiceBackend converts a CreateAIServiceBackendRequest to ports.AIServiceBackend
func ToAIServiceBackend(req *CreateAIServiceBackendRequest) *ports.AIServiceBackend {
	backend := &ports.AIServiceBackend{
		Name:      req.Name,
		Namespace: req.Namespace,
		Schema:    req.Schema,
		BackendRef: ports.BackendRef{
			Name:      req.BackendRef.Name,
			Namespace: req.BackendRef.Namespace,
			Group:     "gateway.envoyproxy.io",
			Kind:      "Backend",
		},
		Labels: req.Labels,
	}

	// Convert HeaderMutation if provided
	if req.HeaderMutation != nil {
		backend.HeaderMutation = &ports.HeaderMutation{
			Remove: req.HeaderMutation.Remove,
		}
		for _, h := range req.HeaderMutation.Set {
			backend.HeaderMutation.Set = append(backend.HeaderMutation.Set, ports.HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
		}
	}

	return backend
}

// HeaderMutationToPorts converts a DTO HeaderMutation to ports.HeaderMutation
func HeaderMutationToPorts(hm *HeaderMutation) ports.HeaderMutation {
	result := ports.HeaderMutation{
		Remove: hm.Remove,
	}
	for _, h := range hm.Set {
		result.Set = append(result.Set, ports.HTTPHeader{
			Name:  h.Name,
			Value: h.Value,
		})
	}
	return result
}

// ToAIServiceBackendResponse converts a ports.AIServiceBackend to AIServiceBackendResponse
func ToAIServiceBackendResponse(backend *ports.AIServiceBackend) AIServiceBackendResponse {
	resp := AIServiceBackendResponse{
		Name:      backend.Name,
		Namespace: backend.Namespace,
		Schema:    backend.Schema,
		BackendRef: BackendRefResponse{
			Name:      backend.BackendRef.Name,
			Namespace: backend.BackendRef.Namespace,
			Group:     backend.BackendRef.Group,
			Kind:      backend.BackendRef.Kind,
		},
		Labels: backend.Labels,
	}

	// Convert HeaderMutation if present
	if backend.HeaderMutation != nil {
		resp.HeaderMutation = &HeaderMutation{
			Remove: backend.HeaderMutation.Remove,
		}
		for _, h := range backend.HeaderMutation.Set {
			resp.HeaderMutation.Set = append(resp.HeaderMutation.Set, HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
		}
	}

	return resp
}

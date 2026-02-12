package dto

import (
	ports "model-registry-service/internal/core/ports/output"
)

// ============================================================================
// Request DTOs
// ============================================================================

// CreateBackendRequest represents the request to create an Envoy Gateway Backend
type CreateBackendRequest struct {
	Name      string            `json:"name" binding:"required"`
	Namespace string            `json:"namespace"`
	Endpoints []EndpointRequest `json:"endpoints" binding:"required,min=1"`
	Labels    map[string]string `json:"labels"`
}

// EndpointRequest represents a single endpoint in the request
type EndpointRequest struct {
	FQDN *FQDNEndpointRequest `json:"fqdn,omitempty"`
	IP   *IPEndpointRequest   `json:"ip,omitempty"`
}

// FQDNEndpointRequest represents an FQDN endpoint
type FQDNEndpointRequest struct {
	Hostname string `json:"hostname" binding:"required"`
	Port     int32  `json:"port" binding:"required,min=1,max=65535"`
}

// IPEndpointRequest represents an IP endpoint
type IPEndpointRequest struct {
	Address string `json:"address" binding:"required,ip"`
	Port    int32  `json:"port" binding:"required,min=1,max=65535"`
}

// UpdateEnvoyBackendRequest represents the request to update an Envoy Gateway Backend
type UpdateEnvoyBackendRequest struct {
	Endpoints []EndpointRequest `json:"endpoints"`
	Labels    map[string]string `json:"labels"`
}

// ============================================================================
// Response DTOs
// ============================================================================

// BackendResponse represents a Backend in API responses
type BackendResponse struct {
	Name      string             `json:"name"`
	Namespace string             `json:"namespace"`
	Endpoints []EndpointResponse `json:"endpoints"`
	Labels    map[string]string  `json:"labels,omitempty"`
}

// EndpointResponse represents an endpoint in responses
type EndpointResponse struct {
	FQDN *FQDNEndpointResponse `json:"fqdn,omitempty"`
	IP   *IPEndpointResponse   `json:"ip,omitempty"`
}

// FQDNEndpointResponse represents an FQDN endpoint in responses
type FQDNEndpointResponse struct {
	Hostname string `json:"hostname"`
	Port     int32  `json:"port"`
}

// IPEndpointResponse represents an IP endpoint in responses
type IPEndpointResponse struct {
	Address string `json:"address"`
	Port    int32  `json:"port"`
}

// ListBackendsResponse represents the list response
type ListBackendsResponse struct {
	Items []BackendResponse `json:"items"`
	Total int               `json:"total"`
}

// ============================================================================
// Converters
// ============================================================================

// ToBackend converts CreateBackendRequest to ports.Backend
func ToBackend(req *CreateBackendRequest) *ports.Backend {
	backend := &ports.Backend{
		Name:      req.Name,
		Namespace: req.Namespace,
		Labels:    req.Labels,
	}

	for _, ep := range req.Endpoints {
		be := ports.BackendEndpoint{}
		if ep.FQDN != nil {
			be.FQDN = &ports.FQDNEndpoint{
				Hostname: ep.FQDN.Hostname,
				Port:     ep.FQDN.Port,
			}
		}
		if ep.IP != nil {
			be.IP = &ports.IPEndpoint{
				Address: ep.IP.Address,
				Port:    ep.IP.Port,
			}
		}
		backend.Endpoints = append(backend.Endpoints, be)
	}

	return backend
}

// ToBackendResponse converts ports.Backend to BackendResponse
func ToBackendResponse(backend *ports.Backend) BackendResponse {
	resp := BackendResponse{
		Name:      backend.Name,
		Namespace: backend.Namespace,
		Labels:    backend.Labels,
	}

	for _, ep := range backend.Endpoints {
		er := EndpointResponse{}
		if ep.FQDN != nil {
			er.FQDN = &FQDNEndpointResponse{
				Hostname: ep.FQDN.Hostname,
				Port:     ep.FQDN.Port,
			}
		}
		if ep.IP != nil {
			er.IP = &IPEndpointResponse{
				Address: ep.IP.Address,
				Port:    ep.IP.Port,
			}
		}
		resp.Endpoints = append(resp.Endpoints, er)
	}

	return resp
}

// EndpointRequestToPorts converts EndpointRequest to ports.BackendEndpoint
func EndpointRequestToPorts(ep *EndpointRequest) ports.BackendEndpoint {
	be := ports.BackendEndpoint{}
	if ep.FQDN != nil {
		be.FQDN = &ports.FQDNEndpoint{
			Hostname: ep.FQDN.Hostname,
			Port:     ep.FQDN.Port,
		}
	}
	if ep.IP != nil {
		be.IP = &ports.IPEndpoint{
			Address: ep.IP.Address,
			Port:    ep.IP.Port,
		}
	}
	return be
}

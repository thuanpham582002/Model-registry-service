package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ports "model-registry-service/internal/core/ports/output"
)

// ============================================================================
// ToBackend Tests
// ============================================================================

func TestToBackend_FQDNEndpoint(t *testing.T) {
	req := &CreateBackendRequest{
		Name:      "kserve-llama",
		Namespace: "model-serving",
		Endpoints: []EndpointRequest{
			{
				FQDN: &FQDNEndpointRequest{
					Hostname: "llama.svc.cluster.local",
					Port:     80,
				},
			},
		},
		Labels: map[string]string{
			"team": "ml-platform",
		},
	}

	backend := ToBackend(req)

	assert.Equal(t, "kserve-llama", backend.Name)
	assert.Equal(t, "model-serving", backend.Namespace)
	assert.Len(t, backend.Endpoints, 1)
	assert.NotNil(t, backend.Endpoints[0].FQDN)
	assert.Nil(t, backend.Endpoints[0].IP)
	assert.Equal(t, "llama.svc.cluster.local", backend.Endpoints[0].FQDN.Hostname)
	assert.Equal(t, int32(80), backend.Endpoints[0].FQDN.Port)
	assert.Equal(t, "ml-platform", backend.Labels["team"])
}

func TestToBackend_IPEndpoint(t *testing.T) {
	req := &CreateBackendRequest{
		Name:      "external-openai",
		Namespace: "model-serving",
		Endpoints: []EndpointRequest{
			{
				IP: &IPEndpointRequest{
					Address: "10.0.0.1",
					Port:    443,
				},
			},
		},
	}

	backend := ToBackend(req)

	assert.Equal(t, "external-openai", backend.Name)
	assert.Len(t, backend.Endpoints, 1)
	assert.Nil(t, backend.Endpoints[0].FQDN)
	assert.NotNil(t, backend.Endpoints[0].IP)
	assert.Equal(t, "10.0.0.1", backend.Endpoints[0].IP.Address)
	assert.Equal(t, int32(443), backend.Endpoints[0].IP.Port)
}

func TestToBackend_MultipleEndpoints(t *testing.T) {
	req := &CreateBackendRequest{
		Name:      "multi-endpoint",
		Namespace: "model-serving",
		Endpoints: []EndpointRequest{
			{
				FQDN: &FQDNEndpointRequest{
					Hostname: "primary.svc.cluster.local",
					Port:     80,
				},
			},
			{
				IP: &IPEndpointRequest{
					Address: "10.0.0.2",
					Port:    8080,
				},
			},
		},
	}

	backend := ToBackend(req)

	assert.Len(t, backend.Endpoints, 2)
	assert.NotNil(t, backend.Endpoints[0].FQDN)
	assert.NotNil(t, backend.Endpoints[1].IP)
}

func TestToBackend_EmptyNamespace(t *testing.T) {
	req := &CreateBackendRequest{
		Name: "backend-no-ns",
		Endpoints: []EndpointRequest{
			{
				FQDN: &FQDNEndpointRequest{
					Hostname: "test.svc.local",
					Port:     80,
				},
			},
		},
	}

	backend := ToBackend(req)

	assert.Equal(t, "", backend.Namespace)
}

func TestToBackend_NilLabels(t *testing.T) {
	req := &CreateBackendRequest{
		Name: "backend-no-labels",
		Endpoints: []EndpointRequest{
			{
				FQDN: &FQDNEndpointRequest{
					Hostname: "test.svc.local",
					Port:     80,
				},
			},
		},
	}

	backend := ToBackend(req)

	assert.Nil(t, backend.Labels)
}

// ============================================================================
// ToBackendResponse Tests
// ============================================================================

func TestToBackendResponse_FQDNEndpoint(t *testing.T) {
	backend := &ports.Backend{
		Name:      "kserve-llama",
		Namespace: "model-serving",
		Endpoints: []ports.BackendEndpoint{
			{
				FQDN: &ports.FQDNEndpoint{
					Hostname: "llama.svc.cluster.local",
					Port:     80,
				},
			},
		},
		Labels: map[string]string{
			"managed-by": "model-registry",
		},
	}

	resp := ToBackendResponse(backend)

	assert.Equal(t, "kserve-llama", resp.Name)
	assert.Equal(t, "model-serving", resp.Namespace)
	assert.Len(t, resp.Endpoints, 1)
	assert.NotNil(t, resp.Endpoints[0].FQDN)
	assert.Nil(t, resp.Endpoints[0].IP)
	assert.Equal(t, "llama.svc.cluster.local", resp.Endpoints[0].FQDN.Hostname)
	assert.Equal(t, int32(80), resp.Endpoints[0].FQDN.Port)
	assert.Equal(t, "model-registry", resp.Labels["managed-by"])
}

func TestToBackendResponse_IPEndpoint(t *testing.T) {
	backend := &ports.Backend{
		Name:      "external-openai",
		Namespace: "model-serving",
		Endpoints: []ports.BackendEndpoint{
			{
				IP: &ports.IPEndpoint{
					Address: "10.0.0.1",
					Port:    443,
				},
			},
		},
	}

	resp := ToBackendResponse(backend)

	assert.Equal(t, "external-openai", resp.Name)
	assert.Len(t, resp.Endpoints, 1)
	assert.Nil(t, resp.Endpoints[0].FQDN)
	assert.NotNil(t, resp.Endpoints[0].IP)
	assert.Equal(t, "10.0.0.1", resp.Endpoints[0].IP.Address)
	assert.Equal(t, int32(443), resp.Endpoints[0].IP.Port)
}

func TestToBackendResponse_EmptyEndpoints(t *testing.T) {
	backend := &ports.Backend{
		Name:      "empty-backend",
		Namespace: "model-serving",
		Endpoints: []ports.BackendEndpoint{},
	}

	resp := ToBackendResponse(backend)

	assert.Empty(t, resp.Endpoints)
}

func TestToBackendResponse_NilLabels(t *testing.T) {
	backend := &ports.Backend{
		Name:      "backend-no-labels",
		Namespace: "model-serving",
		Endpoints: []ports.BackendEndpoint{
			{
				FQDN: &ports.FQDNEndpoint{
					Hostname: "test.svc.local",
					Port:     80,
				},
			},
		},
	}

	resp := ToBackendResponse(backend)

	assert.Nil(t, resp.Labels)
}

// ============================================================================
// EndpointRequestToPorts Tests
// ============================================================================

func TestEndpointRequestToPorts_FQDN(t *testing.T) {
	ep := &EndpointRequest{
		FQDN: &FQDNEndpointRequest{
			Hostname: "test.svc.cluster.local",
			Port:     8080,
		},
	}

	be := EndpointRequestToPorts(ep)

	assert.NotNil(t, be.FQDN)
	assert.Nil(t, be.IP)
	assert.Equal(t, "test.svc.cluster.local", be.FQDN.Hostname)
	assert.Equal(t, int32(8080), be.FQDN.Port)
}

func TestEndpointRequestToPorts_IP(t *testing.T) {
	ep := &EndpointRequest{
		IP: &IPEndpointRequest{
			Address: "192.168.1.100",
			Port:    9090,
		},
	}

	be := EndpointRequestToPorts(ep)

	assert.Nil(t, be.FQDN)
	assert.NotNil(t, be.IP)
	assert.Equal(t, "192.168.1.100", be.IP.Address)
	assert.Equal(t, int32(9090), be.IP.Port)
}

func TestEndpointRequestToPorts_EmptyEndpoint(t *testing.T) {
	ep := &EndpointRequest{}

	be := EndpointRequestToPorts(ep)

	assert.Nil(t, be.FQDN)
	assert.Nil(t, be.IP)
}

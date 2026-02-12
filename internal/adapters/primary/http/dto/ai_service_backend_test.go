package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ports "model-registry-service/internal/core/ports/output"
)

// ============================================================================
// ToAIServiceBackend Tests
// ============================================================================

func TestToAIServiceBackend_MinimalRequest(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test-backend",
		Schema: "OpenAI",
		BackendRef: BackendRefRequest{
			Name: "openai-svc",
		},
	}

	result := ToAIServiceBackend(req)

	assert.Equal(t, "test-backend", result.Name)
	assert.Equal(t, "", result.Namespace)
	assert.Equal(t, "OpenAI", result.Schema)
	assert.Equal(t, "openai-svc", result.BackendRef.Name)
	assert.Equal(t, "gateway.envoyproxy.io", result.BackendRef.Group)
	assert.Equal(t, "Backend", result.BackendRef.Kind)
	assert.Nil(t, result.HeaderMutation)
	assert.Nil(t, result.Labels)
}

func TestToAIServiceBackend_WithNamespace(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:      "test-backend",
		Namespace: "custom-namespace",
		Schema:    "Anthropic",
		BackendRef: BackendRefRequest{
			Name:      "anthropic-svc",
			Namespace: "backend-ns",
		},
	}

	result := ToAIServiceBackend(req)

	assert.Equal(t, "custom-namespace", result.Namespace)
	assert.Equal(t, "backend-ns", result.BackendRef.Namespace)
}

func TestToAIServiceBackend_WithLabels(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test-backend",
		Schema: "OpenAI",
		BackendRef: BackendRefRequest{
			Name: "openai-svc",
		},
		Labels: map[string]string{
			"env":     "prod",
			"version": "v1",
			"team":    "ml-platform",
		},
	}

	result := ToAIServiceBackend(req)

	assert.NotNil(t, result.Labels)
	assert.Equal(t, "prod", result.Labels["env"])
	assert.Equal(t, "v1", result.Labels["version"])
	assert.Equal(t, "ml-platform", result.Labels["team"])
	assert.Len(t, result.Labels, 3)
}

func TestToAIServiceBackend_WithHeaderMutation_SetOnly(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test-backend",
		Schema: "OpenAI",
		BackendRef: BackendRefRequest{
			Name: "openai-svc",
		},
		HeaderMutation: &HeaderMutation{
			Set: []HTTPHeader{
				{Name: "Authorization", Value: "Bearer sk-test"},
				{Name: "X-API-Version", Value: "2023-01"},
			},
		},
	}

	result := ToAIServiceBackend(req)

	assert.NotNil(t, result.HeaderMutation)
	assert.Len(t, result.HeaderMutation.Set, 2)
	assert.Equal(t, "Authorization", result.HeaderMutation.Set[0].Name)
	assert.Equal(t, "Bearer sk-test", result.HeaderMutation.Set[0].Value)
	assert.Equal(t, "X-API-Version", result.HeaderMutation.Set[1].Name)
	assert.Equal(t, "2023-01", result.HeaderMutation.Set[1].Value)
	assert.Empty(t, result.HeaderMutation.Remove)
}

func TestToAIServiceBackend_WithHeaderMutation_RemoveOnly(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test-backend",
		Schema: "OpenAI",
		BackendRef: BackendRefRequest{
			Name: "openai-svc",
		},
		HeaderMutation: &HeaderMutation{
			Remove: []string{"X-Custom-Header", "X-Debug-Token"},
		},
	}

	result := ToAIServiceBackend(req)

	assert.NotNil(t, result.HeaderMutation)
	assert.Len(t, result.HeaderMutation.Remove, 2)
	assert.Equal(t, "X-Custom-Header", result.HeaderMutation.Remove[0])
	assert.Equal(t, "X-Debug-Token", result.HeaderMutation.Remove[1])
	assert.Empty(t, result.HeaderMutation.Set)
}

func TestToAIServiceBackend_WithHeaderMutation_SetAndRemove(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test-backend",
		Schema: "AWSBedrock",
		BackendRef: BackendRefRequest{
			Name: "bedrock-svc",
		},
		HeaderMutation: &HeaderMutation{
			Set: []HTTPHeader{
				{Name: "X-API-Key", Value: "secret-key"},
			},
			Remove: []string{"X-Debug"},
		},
	}

	result := ToAIServiceBackend(req)

	assert.NotNil(t, result.HeaderMutation)
	assert.Len(t, result.HeaderMutation.Set, 1)
	assert.Equal(t, "X-API-Key", result.HeaderMutation.Set[0].Name)
	assert.Len(t, result.HeaderMutation.Remove, 1)
	assert.Equal(t, "X-Debug", result.HeaderMutation.Remove[0])
}

func TestToAIServiceBackend_WithEmptyHeaderMutation(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test-backend",
		Schema: "OpenAI",
		BackendRef: BackendRefRequest{
			Name: "openai-svc",
		},
		HeaderMutation: &HeaderMutation{},
	}

	result := ToAIServiceBackend(req)

	assert.NotNil(t, result.HeaderMutation)
	assert.Empty(t, result.HeaderMutation.Set)
	assert.Empty(t, result.HeaderMutation.Remove)
}

func TestToAIServiceBackend_AllSchemas(t *testing.T) {
	schemas := []string{"OpenAI", "Anthropic", "AWSBedrock", "GoogleAI", "AzureOpenAI"}

	for _, schema := range schemas {
		t.Run(schema, func(t *testing.T) {
			req := &CreateAIServiceBackendRequest{
				Name:   "test-backend",
				Schema: schema,
				BackendRef: BackendRefRequest{
					Name: "test-svc",
				},
			}

			result := ToAIServiceBackend(req)
			assert.Equal(t, schema, result.Schema)
		})
	}
}

// ============================================================================
// HeaderMutationToPorts Tests
// ============================================================================

func TestHeaderMutationToPorts_SetOnly(t *testing.T) {
	hm := &HeaderMutation{
		Set: []HTTPHeader{
			{Name: "Authorization", Value: "Bearer token"},
			{Name: "X-Custom", Value: "value"},
		},
	}

	result := HeaderMutationToPorts(hm)

	assert.Len(t, result.Set, 2)
	assert.Equal(t, "Authorization", result.Set[0].Name)
	assert.Equal(t, "Bearer token", result.Set[0].Value)
	assert.Equal(t, "X-Custom", result.Set[1].Name)
	assert.Equal(t, "value", result.Set[1].Value)
	assert.Empty(t, result.Remove)
}

func TestHeaderMutationToPorts_RemoveOnly(t *testing.T) {
	hm := &HeaderMutation{
		Remove: []string{"X-Debug", "X-Trace"},
	}

	result := HeaderMutationToPorts(hm)

	assert.Len(t, result.Remove, 2)
	assert.Equal(t, "X-Debug", result.Remove[0])
	assert.Equal(t, "X-Trace", result.Remove[1])
	assert.Empty(t, result.Set)
}

func TestHeaderMutationToPorts_SetAndRemove(t *testing.T) {
	hm := &HeaderMutation{
		Set: []HTTPHeader{
			{Name: "Authorization", Value: "Bearer token"},
		},
		Remove: []string{"X-Debug"},
	}

	result := HeaderMutationToPorts(hm)

	assert.Len(t, result.Set, 1)
	assert.Equal(t, "Authorization", result.Set[0].Name)
	assert.Len(t, result.Remove, 1)
	assert.Equal(t, "X-Debug", result.Remove[0])
}

func TestHeaderMutationToPorts_Empty(t *testing.T) {
	hm := &HeaderMutation{}

	result := HeaderMutationToPorts(hm)

	assert.Empty(t, result.Set)
	assert.Empty(t, result.Remove)
}

// ============================================================================
// ToAIServiceBackendResponse Tests
// ============================================================================

func TestToAIServiceBackendResponse_Minimal(t *testing.T) {
	backend := &ports.AIServiceBackend{
		Name:      "test-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
		BackendRef: ports.BackendRef{
			Name:      "openai-svc",
			Namespace: "backend-ns",
			Group:     "gateway.envoyproxy.io",
			Kind:      "Backend",
		},
	}

	result := ToAIServiceBackendResponse(backend)

	assert.Equal(t, "test-backend", result.Name)
	assert.Equal(t, "model-serving", result.Namespace)
	assert.Equal(t, "OpenAI", result.Schema)
	assert.Equal(t, "openai-svc", result.BackendRef.Name)
	assert.Equal(t, "backend-ns", result.BackendRef.Namespace)
	assert.Equal(t, "gateway.envoyproxy.io", result.BackendRef.Group)
	assert.Equal(t, "Backend", result.BackendRef.Kind)
	assert.Nil(t, result.HeaderMutation)
	assert.Nil(t, result.Labels)
}

func TestToAIServiceBackendResponse_WithLabels(t *testing.T) {
	backend := &ports.AIServiceBackend{
		Name:      "test-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
		BackendRef: ports.BackendRef{
			Name: "openai-svc",
		},
		Labels: map[string]string{
			"env":  "prod",
			"tier": "critical",
		},
	}

	result := ToAIServiceBackendResponse(backend)

	assert.NotNil(t, result.Labels)
	assert.Equal(t, "prod", result.Labels["env"])
	assert.Equal(t, "critical", result.Labels["tier"])
	assert.Len(t, result.Labels, 2)
}

func TestToAIServiceBackendResponse_WithHeaderMutation(t *testing.T) {
	backend := &ports.AIServiceBackend{
		Name:      "test-backend",
		Namespace: "model-serving",
		Schema:    "Anthropic",
		BackendRef: ports.BackendRef{
			Name: "anthropic-svc",
		},
		HeaderMutation: &ports.HeaderMutation{
			Set: []ports.HTTPHeader{
				{Name: "X-API-Key", Value: "sk-ant-xxx"},
				{Name: "Anthropic-Version", Value: "2023-06-01"},
			},
			Remove: []string{"X-Custom-Header"},
		},
	}

	result := ToAIServiceBackendResponse(backend)

	assert.NotNil(t, result.HeaderMutation)
	assert.Len(t, result.HeaderMutation.Set, 2)
	assert.Equal(t, "X-API-Key", result.HeaderMutation.Set[0].Name)
	assert.Equal(t, "sk-ant-xxx", result.HeaderMutation.Set[0].Value)
	assert.Equal(t, "Anthropic-Version", result.HeaderMutation.Set[1].Name)
	assert.Equal(t, "2023-06-01", result.HeaderMutation.Set[1].Value)
	assert.Len(t, result.HeaderMutation.Remove, 1)
	assert.Equal(t, "X-Custom-Header", result.HeaderMutation.Remove[0])
}

func TestToAIServiceBackendResponse_WithEmptyHeaderMutation(t *testing.T) {
	backend := &ports.AIServiceBackend{
		Name:      "test-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
		BackendRef: ports.BackendRef{
			Name: "openai-svc",
		},
		HeaderMutation: &ports.HeaderMutation{},
	}

	result := ToAIServiceBackendResponse(backend)

	assert.NotNil(t, result.HeaderMutation)
	assert.Empty(t, result.HeaderMutation.Set)
	assert.Empty(t, result.HeaderMutation.Remove)
}

func TestToAIServiceBackendResponse_AllSchemas(t *testing.T) {
	schemas := []string{"OpenAI", "Anthropic", "AWSBedrock", "GoogleAI", "AzureOpenAI"}

	for _, schema := range schemas {
		t.Run(schema, func(t *testing.T) {
			backend := &ports.AIServiceBackend{
				Name:      "test-backend",
				Namespace: "model-serving",
				Schema:    schema,
				BackendRef: ports.BackendRef{
					Name: "test-svc",
				},
			}

			result := ToAIServiceBackendResponse(backend)
			assert.Equal(t, schema, result.Schema)
		})
	}
}

// ============================================================================
// Round-trip Conversion Tests
// ============================================================================

func TestRoundTripConversion_MinimalBackend(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test-backend",
		Schema: "OpenAI",
		BackendRef: BackendRefRequest{
			Name:      "openai-svc",
			Namespace: "backend-ns",
		},
	}

	// Request -> Ports
	portsBackend := ToAIServiceBackend(req)

	// Ports -> Response
	response := ToAIServiceBackendResponse(portsBackend)

	assert.Equal(t, req.Name, response.Name)
	assert.Equal(t, req.Schema, response.Schema)
	assert.Equal(t, req.BackendRef.Name, response.BackendRef.Name)
	assert.Equal(t, req.BackendRef.Namespace, response.BackendRef.Namespace)
}

func TestRoundTripConversion_CompleteBackend(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:      "test-backend",
		Namespace: "model-serving",
		Schema:    "Anthropic",
		BackendRef: BackendRefRequest{
			Name:      "anthropic-svc",
			Namespace: "backend-ns",
		},
		HeaderMutation: &HeaderMutation{
			Set: []HTTPHeader{
				{Name: "X-API-Key", Value: "secret"},
			},
			Remove: []string{"X-Debug"},
		},
		Labels: map[string]string{
			"env": "test",
		},
	}

	// Request -> Ports
	portsBackend := ToAIServiceBackend(req)

	// Ports -> Response
	response := ToAIServiceBackendResponse(portsBackend)

	assert.Equal(t, req.Name, response.Name)
	assert.Equal(t, req.Namespace, response.Namespace)
	assert.Equal(t, req.Schema, response.Schema)
	assert.Equal(t, req.BackendRef.Name, response.BackendRef.Name)
	assert.Equal(t, "gateway.envoyproxy.io", response.BackendRef.Group)
	assert.Equal(t, "Backend", response.BackendRef.Kind)

	assert.NotNil(t, response.HeaderMutation)
	assert.Len(t, response.HeaderMutation.Set, 1)
	assert.Equal(t, "X-API-Key", response.HeaderMutation.Set[0].Name)
	assert.Equal(t, "secret", response.HeaderMutation.Set[0].Value)
	assert.Len(t, response.HeaderMutation.Remove, 1)
	assert.Equal(t, "X-Debug", response.HeaderMutation.Remove[0])

	assert.NotNil(t, response.Labels)
	assert.Equal(t, "test", response.Labels["env"])
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestToAIServiceBackend_EmptyStrings(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:      "",
		Namespace: "",
		Schema:    "",
		BackendRef: BackendRefRequest{
			Name:      "",
			Namespace: "",
		},
	}

	result := ToAIServiceBackend(req)

	assert.Equal(t, "", result.Name)
	assert.Equal(t, "", result.Namespace)
	assert.Equal(t, "", result.Schema)
	assert.Equal(t, "", result.BackendRef.Name)
	assert.Equal(t, "", result.BackendRef.Namespace)
	assert.Equal(t, "gateway.envoyproxy.io", result.BackendRef.Group)
	assert.Equal(t, "Backend", result.BackendRef.Kind)
}

func TestToAIServiceBackendResponse_EmptyStrings(t *testing.T) {
	backend := &ports.AIServiceBackend{
		Name:      "",
		Namespace: "",
		Schema:    "",
		BackendRef: ports.BackendRef{
			Name:      "",
			Namespace: "",
			Group:     "",
			Kind:      "",
		},
	}

	result := ToAIServiceBackendResponse(backend)

	assert.Equal(t, "", result.Name)
	assert.Equal(t, "", result.Namespace)
	assert.Equal(t, "", result.Schema)
	assert.Equal(t, "", result.BackendRef.Name)
	assert.Equal(t, "", result.BackendRef.Namespace)
	assert.Equal(t, "", result.BackendRef.Group)
	assert.Equal(t, "", result.BackendRef.Kind)
}

func TestToAIServiceBackend_NilLabels(t *testing.T) {
	req := &CreateAIServiceBackendRequest{
		Name:   "test",
		Schema: "OpenAI",
		BackendRef: BackendRefRequest{
			Name: "svc",
		},
		Labels: nil,
	}

	result := ToAIServiceBackend(req)
	assert.Nil(t, result.Labels)
}

func TestToAIServiceBackendResponse_NilLabels(t *testing.T) {
	backend := &ports.AIServiceBackend{
		Name:   "test",
		Schema: "OpenAI",
		BackendRef: ports.BackendRef{
			Name: "svc",
		},
		Labels: nil,
	}

	result := ToAIServiceBackendResponse(backend)
	assert.Nil(t, result.Labels)
}

func TestHeaderMutationToPorts_NilSlices(t *testing.T) {
	hm := &HeaderMutation{
		Set:    nil,
		Remove: nil,
	}

	result := HeaderMutationToPorts(hm)

	assert.Empty(t, result.Set)
	assert.Empty(t, result.Remove)
}

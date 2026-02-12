package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
	"model-registry-service/internal/testutil"
)

// ============================================================================
// Service Creation Tests
// ============================================================================

func TestNewAIServiceBackendService(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	assert.NotNil(t, svc)
	assert.Equal(t, client, svc.aiGateway)
}

// ============================================================================
// mapK8sError Tests
// ============================================================================

func TestMapK8sError_Nil(t *testing.T) {
	err := mapK8sError(nil)
	assert.NoError(t, err)
}

func TestMapK8sError_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
		expected error
	}{
		{
			name:     "lowercase not found",
			inputErr: errors.New("resource not found"),
			expected: domain.ErrBackendNotFound,
		},
		{
			name:     "CamelCase NotFound",
			inputErr: errors.New("Backend NotFound in namespace"),
			expected: domain.ErrBackendNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapK8sError(tt.inputErr)
			assert.ErrorIs(t, result, tt.expected)
		})
	}
}

func TestMapK8sError_AlreadyExists(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
		expected error
	}{
		{
			name:     "lowercase already exists",
			inputErr: errors.New("backend already exists"),
			expected: domain.ErrBackendAlreadyExists,
		},
		{
			name:     "CamelCase AlreadyExists",
			inputErr: errors.New("Backend AlreadyExists in namespace"),
			expected: domain.ErrBackendAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapK8sError(tt.inputErr)
			assert.ErrorIs(t, result, tt.expected)
		})
	}
}

func TestMapK8sError_UnknownError(t *testing.T) {
	unknownErr := errors.New("connection timeout")
	result := mapK8sError(unknownErr)
	assert.Equal(t, unknownErr, result)
}

// ============================================================================
// Create Tests
// ============================================================================

func TestAIServiceBackendService_Create_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "openai-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
		BackendRef: output.BackendRef{
			Name:      "openai-svc",
			Namespace: "model-serving",
			Group:     "gateway.envoyproxy.io",
			Kind:      "Backend",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("CreateServiceBackend", mock.Anything, backend).Return(nil)

	err := svc.Create(context.Background(), backend)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Create_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "test-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
	}

	client.On("IsAvailable").Return(false)

	err := svc.Create(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Create_NilClient(t *testing.T) {
	svc := NewAIServiceBackendService(nil)

	backend := &output.AIServiceBackend{
		Name:      "test-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
	}

	err := svc.Create(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
}

func TestAIServiceBackendService_Create_AlreadyExists(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "existing-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
	}

	client.On("IsAvailable").Return(true)
	client.On("CreateServiceBackend", mock.Anything, backend).
		Return(errors.New("backend already exists"))

	err := svc.Create(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrBackendAlreadyExists)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Create_WithHeaderMutation(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "openai-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
		BackendRef: output.BackendRef{
			Name: "openai-svc",
		},
		HeaderMutation: &output.HeaderMutation{
			Set: []output.HTTPHeader{
				{Name: "Authorization", Value: "Bearer sk-xxx"},
				{Name: "X-API-Version", Value: "2023-01"},
			},
			Remove: []string{"X-Custom-Header"},
		},
		Labels: map[string]string{
			"env": "prod",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("CreateServiceBackend", mock.Anything, backend).Return(nil)

	err := svc.Create(context.Background(), backend)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

// ============================================================================
// Get Tests
// ============================================================================

func TestAIServiceBackendService_Get_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	expectedBackend := &output.AIServiceBackend{
		Name:      "openai-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
		BackendRef: output.BackendRef{
			Name:  "openai-svc",
			Group: "gateway.envoyproxy.io",
			Kind:  "Backend",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("GetServiceBackend", mock.Anything, "model-serving", "openai-backend").
		Return(expectedBackend, nil)

	backend, err := svc.Get(context.Background(), "model-serving", "openai-backend")
	assert.NoError(t, err)
	assert.Equal(t, "openai-backend", backend.Name)
	assert.Equal(t, "OpenAI", backend.Schema)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Get_NotFound(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("GetServiceBackend", mock.Anything, "model-serving", "nonexistent").
		Return(nil, errors.New("backend not found"))

	backend, err := svc.Get(context.Background(), "model-serving", "nonexistent")
	assert.Nil(t, backend)
	assert.ErrorIs(t, err, domain.ErrBackendNotFound)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Get_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(false)

	backend, err := svc.Get(context.Background(), "model-serving", "test")
	assert.Nil(t, backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// List Tests
// ============================================================================

func TestAIServiceBackendService_List_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	expectedBackends := []*output.AIServiceBackend{
		{
			Name:      "openai-backend",
			Namespace: "model-serving",
			Schema:    "OpenAI",
		},
		{
			Name:      "anthropic-backend",
			Namespace: "model-serving",
			Schema:    "Anthropic",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("ListServiceBackends", mock.Anything, "model-serving").
		Return(expectedBackends, nil)

	backends, err := svc.List(context.Background(), "model-serving")
	assert.NoError(t, err)
	assert.Len(t, backends, 2)
	assert.Equal(t, "openai-backend", backends[0].Name)
	assert.Equal(t, "anthropic-backend", backends[1].Name)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_List_EmptyResult(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("ListServiceBackends", mock.Anything, "model-serving").
		Return([]*output.AIServiceBackend{}, nil)

	backends, err := svc.List(context.Background(), "model-serving")
	assert.NoError(t, err)
	assert.Empty(t, backends)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_List_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(false)

	backends, err := svc.List(context.Background(), "model-serving")
	assert.Nil(t, backends)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// Update Tests
// ============================================================================

func TestAIServiceBackendService_Update_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "openai-backend",
		Namespace: "model-serving",
		Schema:    "OpenAI",
		BackendRef: output.BackendRef{
			Name: "openai-svc-updated",
		},
		Labels: map[string]string{
			"version": "v2",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("UpdateServiceBackend", mock.Anything, backend).Return(nil)

	err := svc.Update(context.Background(), backend)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Update_NotFound(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "nonexistent",
		Namespace: "model-serving",
		Schema:    "OpenAI",
	}

	client.On("IsAvailable").Return(true)
	client.On("UpdateServiceBackend", mock.Anything, backend).
		Return(errors.New("backend not found"))

	err := svc.Update(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrBackendNotFound)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Update_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "test",
		Namespace: "model-serving",
		Schema:    "OpenAI",
	}

	client.On("IsAvailable").Return(false)

	err := svc.Update(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestAIServiceBackendService_Delete_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("DeleteServiceBackend", mock.Anything, "model-serving", "old-backend").
		Return(nil)

	err := svc.Delete(context.Background(), "model-serving", "old-backend")
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Delete_NotFound(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("DeleteServiceBackend", mock.Anything, "model-serving", "nonexistent").
		Return(errors.New("backend not found"))

	err := svc.Delete(context.Background(), "model-serving", "nonexistent")
	assert.ErrorIs(t, err, domain.ErrBackendNotFound)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Delete_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(false)

	err := svc.Delete(context.Background(), "model-serving", "test")
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// Exists Tests
// ============================================================================

func TestAIServiceBackendService_Exists_True(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	backend := &output.AIServiceBackend{
		Name:      "existing",
		Namespace: "model-serving",
		Schema:    "OpenAI",
	}

	client.On("IsAvailable").Return(true)
	client.On("GetServiceBackend", mock.Anything, "model-serving", "existing").
		Return(backend, nil)

	exists := svc.Exists(context.Background(), "model-serving", "existing")
	assert.True(t, exists)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Exists_False(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("GetServiceBackend", mock.Anything, "model-serving", "nonexistent").
		Return(nil, errors.New("not found"))

	exists := svc.Exists(context.Background(), "model-serving", "nonexistent")
	assert.False(t, exists)
	client.AssertExpectations(t)
}

func TestAIServiceBackendService_Exists_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewAIServiceBackendService(client)

	client.On("IsAvailable").Return(false)

	exists := svc.Exists(context.Background(), "model-serving", "test")
	assert.False(t, exists)
	client.AssertExpectations(t)
}

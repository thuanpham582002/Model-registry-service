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

func TestNewBackendService(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	assert.NotNil(t, svc)
	assert.Equal(t, client, svc.aiGateway)
}

// ============================================================================
// mapEnvoyBackendK8sError Tests
// ============================================================================

func TestMapEnvoyBackendK8sError_Nil(t *testing.T) {
	err := mapEnvoyBackendK8sError(nil)
	assert.NoError(t, err)
}

func TestMapEnvoyBackendK8sError_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
		expected error
	}{
		{
			name:     "lowercase not found",
			inputErr: errors.New("resource not found"),
			expected: domain.ErrEnvoyBackendNotFound,
		},
		{
			name:     "CamelCase NotFound",
			inputErr: errors.New("Backend NotFound in namespace"),
			expected: domain.ErrEnvoyBackendNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapEnvoyBackendK8sError(tt.inputErr)
			assert.ErrorIs(t, result, tt.expected)
		})
	}
}

func TestMapEnvoyBackendK8sError_AlreadyExists(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
		expected error
	}{
		{
			name:     "lowercase already exists",
			inputErr: errors.New("backend already exists"),
			expected: domain.ErrEnvoyBackendAlreadyExists,
		},
		{
			name:     "CamelCase AlreadyExists",
			inputErr: errors.New("Backend AlreadyExists in namespace"),
			expected: domain.ErrEnvoyBackendAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapEnvoyBackendK8sError(tt.inputErr)
			assert.ErrorIs(t, result, tt.expected)
		})
	}
}

func TestMapEnvoyBackendK8sError_UnknownError(t *testing.T) {
	unknownErr := errors.New("connection timeout")
	result := mapEnvoyBackendK8sError(unknownErr)
	assert.Equal(t, unknownErr, result)
}

// ============================================================================
// Create Tests
// ============================================================================

func TestBackendService_Create_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "kserve-llama",
		Namespace: "model-serving",
		Endpoints: []output.BackendEndpoint{
			{
				FQDN: &output.FQDNEndpoint{
					Hostname: "llama.svc.cluster.local",
					Port:     80,
				},
			},
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("CreateBackend", mock.Anything, backend).Return(nil)

	err := svc.Create(context.Background(), backend)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestBackendService_Create_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "test-backend",
		Namespace: "model-serving",
	}

	client.On("IsAvailable").Return(false)

	err := svc.Create(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

func TestBackendService_Create_NilClient(t *testing.T) {
	svc := NewBackendService(nil)

	backend := &output.Backend{
		Name:      "test-backend",
		Namespace: "model-serving",
	}

	err := svc.Create(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
}

func TestBackendService_Create_AlreadyExists(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "existing-backend",
		Namespace: "model-serving",
	}

	client.On("IsAvailable").Return(true)
	client.On("CreateBackend", mock.Anything, backend).
		Return(errors.New("backend already exists"))

	err := svc.Create(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrEnvoyBackendAlreadyExists)
	client.AssertExpectations(t)
}

func TestBackendService_Create_WithIPEndpoint(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "external-backend",
		Namespace: "model-serving",
		Endpoints: []output.BackendEndpoint{
			{
				IP: &output.IPEndpoint{
					Address: "10.0.0.1",
					Port:    443,
				},
			},
		},
		Labels: map[string]string{
			"env": "prod",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("CreateBackend", mock.Anything, backend).Return(nil)

	err := svc.Create(context.Background(), backend)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

// ============================================================================
// Get Tests
// ============================================================================

func TestBackendService_Get_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	expectedBackend := &output.Backend{
		Name:      "kserve-llama",
		Namespace: "model-serving",
		Endpoints: []output.BackendEndpoint{
			{
				FQDN: &output.FQDNEndpoint{
					Hostname: "llama.svc.cluster.local",
					Port:     80,
				},
			},
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("GetBackend", mock.Anything, "model-serving", "kserve-llama").
		Return(expectedBackend, nil)

	backend, err := svc.Get(context.Background(), "model-serving", "kserve-llama")
	assert.NoError(t, err)
	assert.Equal(t, "kserve-llama", backend.Name)
	assert.Len(t, backend.Endpoints, 1)
	client.AssertExpectations(t)
}

func TestBackendService_Get_NotFound(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("GetBackend", mock.Anything, "model-serving", "nonexistent").
		Return(nil, errors.New("backend not found"))

	backend, err := svc.Get(context.Background(), "model-serving", "nonexistent")
	assert.Nil(t, backend)
	assert.ErrorIs(t, err, domain.ErrEnvoyBackendNotFound)
	client.AssertExpectations(t)
}

func TestBackendService_Get_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(false)

	backend, err := svc.Get(context.Background(), "model-serving", "test")
	assert.Nil(t, backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// List Tests
// ============================================================================

func TestBackendService_List_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	expectedBackends := []*output.Backend{
		{
			Name:      "kserve-llama",
			Namespace: "model-serving",
		},
		{
			Name:      "kserve-mistral",
			Namespace: "model-serving",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("ListBackends", mock.Anything, "model-serving").
		Return(expectedBackends, nil)

	backends, err := svc.List(context.Background(), "model-serving")
	assert.NoError(t, err)
	assert.Len(t, backends, 2)
	assert.Equal(t, "kserve-llama", backends[0].Name)
	assert.Equal(t, "kserve-mistral", backends[1].Name)
	client.AssertExpectations(t)
}

func TestBackendService_List_EmptyResult(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("ListBackends", mock.Anything, "model-serving").
		Return([]*output.Backend{}, nil)

	backends, err := svc.List(context.Background(), "model-serving")
	assert.NoError(t, err)
	assert.Empty(t, backends)
	client.AssertExpectations(t)
}

func TestBackendService_List_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(false)

	backends, err := svc.List(context.Background(), "model-serving")
	assert.Nil(t, backends)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// Update Tests
// ============================================================================

func TestBackendService_Update_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "kserve-llama",
		Namespace: "model-serving",
		Endpoints: []output.BackendEndpoint{
			{
				FQDN: &output.FQDNEndpoint{
					Hostname: "llama-v2.svc.cluster.local",
					Port:     8080,
				},
			},
		},
		Labels: map[string]string{
			"version": "v2",
		},
	}

	client.On("IsAvailable").Return(true)
	client.On("UpdateBackend", mock.Anything, backend).Return(nil)

	err := svc.Update(context.Background(), backend)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestBackendService_Update_NotFound(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "nonexistent",
		Namespace: "model-serving",
	}

	client.On("IsAvailable").Return(true)
	client.On("UpdateBackend", mock.Anything, backend).
		Return(errors.New("backend not found"))

	err := svc.Update(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrEnvoyBackendNotFound)
	client.AssertExpectations(t)
}

func TestBackendService_Update_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "test",
		Namespace: "model-serving",
	}

	client.On("IsAvailable").Return(false)

	err := svc.Update(context.Background(), backend)
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestBackendService_Delete_Success(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("DeleteBackend", mock.Anything, "model-serving", "old-backend").
		Return(nil)

	err := svc.Delete(context.Background(), "model-serving", "old-backend")
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestBackendService_Delete_NotFound(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("DeleteBackend", mock.Anything, "model-serving", "nonexistent").
		Return(errors.New("backend not found"))

	err := svc.Delete(context.Background(), "model-serving", "nonexistent")
	assert.ErrorIs(t, err, domain.ErrEnvoyBackendNotFound)
	client.AssertExpectations(t)
}

func TestBackendService_Delete_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(false)

	err := svc.Delete(context.Background(), "model-serving", "test")
	assert.ErrorIs(t, err, domain.ErrAIGatewayNotAvailable)
	client.AssertExpectations(t)
}

// ============================================================================
// Exists Tests
// ============================================================================

func TestBackendService_Exists_True(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	backend := &output.Backend{
		Name:      "existing",
		Namespace: "model-serving",
	}

	client.On("IsAvailable").Return(true)
	client.On("GetBackend", mock.Anything, "model-serving", "existing").
		Return(backend, nil)

	exists := svc.Exists(context.Background(), "model-serving", "existing")
	assert.True(t, exists)
	client.AssertExpectations(t)
}

func TestBackendService_Exists_False(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(true)
	client.On("GetBackend", mock.Anything, "model-serving", "nonexistent").
		Return(nil, errors.New("not found"))

	exists := svc.Exists(context.Background(), "model-serving", "nonexistent")
	assert.False(t, exists)
	client.AssertExpectations(t)
}

func TestBackendService_Exists_GatewayNotAvailable(t *testing.T) {
	client := new(testutil.MockAIGatewayClient)
	svc := NewBackendService(client)

	client.On("IsAvailable").Return(false)

	exists := svc.Exists(context.Background(), "model-serving", "test")
	assert.False(t, exists)
	client.AssertExpectations(t)
}

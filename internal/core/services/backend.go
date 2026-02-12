package services

import (
	"context"
	"strings"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

// BackendService handles Envoy Gateway Backend CRUD operations
type BackendService struct {
	aiGateway output.AIGatewayClient
}

// NewBackendService creates a new BackendService
func NewBackendService(aiGateway output.AIGatewayClient) *BackendService {
	return &BackendService{aiGateway: aiGateway}
}

// mapEnvoyBackendK8sError converts K8s API errors to domain errors
func mapEnvoyBackendK8sError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "NotFound") {
		return domain.ErrEnvoyBackendNotFound
	}
	if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "AlreadyExists") {
		return domain.ErrEnvoyBackendAlreadyExists
	}
	return err
}

// Create creates a new Backend in K8s
func (s *BackendService) Create(ctx context.Context, backend *output.Backend) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return domain.ErrAIGatewayNotAvailable
	}
	return mapEnvoyBackendK8sError(s.aiGateway.CreateBackend(ctx, backend))
}

// Get retrieves a Backend from K8s
func (s *BackendService) Get(ctx context.Context, namespace, name string) (*output.Backend, error) {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return nil, domain.ErrAIGatewayNotAvailable
	}
	backend, err := s.aiGateway.GetBackend(ctx, namespace, name)
	return backend, mapEnvoyBackendK8sError(err)
}

// List lists all Backends in a namespace
func (s *BackendService) List(ctx context.Context, namespace string) ([]*output.Backend, error) {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return nil, domain.ErrAIGatewayNotAvailable
	}
	backends, err := s.aiGateway.ListBackends(ctx, namespace)
	return backends, mapEnvoyBackendK8sError(err)
}

// Update updates an existing Backend
func (s *BackendService) Update(ctx context.Context, backend *output.Backend) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return domain.ErrAIGatewayNotAvailable
	}
	return mapEnvoyBackendK8sError(s.aiGateway.UpdateBackend(ctx, backend))
}

// Delete removes a Backend from K8s
func (s *BackendService) Delete(ctx context.Context, namespace, name string) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return domain.ErrAIGatewayNotAvailable
	}
	return mapEnvoyBackendK8sError(s.aiGateway.DeleteBackend(ctx, namespace, name))
}

// Exists checks if a Backend exists
func (s *BackendService) Exists(ctx context.Context, namespace, name string) bool {
	_, err := s.Get(ctx, namespace, name)
	return err == nil
}

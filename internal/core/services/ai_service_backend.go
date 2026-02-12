package services

import (
	"context"
	"strings"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

// AIServiceBackendService handles AIServiceBackend CRUD operations
type AIServiceBackendService struct {
	aiGateway output.AIGatewayClient
}

// NewAIServiceBackendService creates a new AIServiceBackendService
func NewAIServiceBackendService(aiGateway output.AIGatewayClient) *AIServiceBackendService {
	return &AIServiceBackendService{aiGateway: aiGateway}
}

// mapK8sError converts K8s API errors to domain errors
func mapK8sError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "NotFound") {
		return domain.ErrBackendNotFound
	}
	if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "AlreadyExists") {
		return domain.ErrBackendAlreadyExists
	}
	return err
}

// Create creates a new AIServiceBackend in K8s
func (s *AIServiceBackendService) Create(ctx context.Context, backend *output.AIServiceBackend) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return domain.ErrAIGatewayNotAvailable
	}
	return mapK8sError(s.aiGateway.CreateServiceBackend(ctx, backend))
}

// Get retrieves an AIServiceBackend from K8s
func (s *AIServiceBackendService) Get(ctx context.Context, namespace, name string) (*output.AIServiceBackend, error) {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return nil, domain.ErrAIGatewayNotAvailable
	}
	backend, err := s.aiGateway.GetServiceBackend(ctx, namespace, name)
	return backend, mapK8sError(err)
}

// List lists all AIServiceBackends in a namespace
func (s *AIServiceBackendService) List(ctx context.Context, namespace string) ([]*output.AIServiceBackend, error) {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return nil, domain.ErrAIGatewayNotAvailable
	}
	backends, err := s.aiGateway.ListServiceBackends(ctx, namespace)
	return backends, mapK8sError(err)
}

// Update updates an existing AIServiceBackend
func (s *AIServiceBackendService) Update(ctx context.Context, backend *output.AIServiceBackend) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return domain.ErrAIGatewayNotAvailable
	}
	return mapK8sError(s.aiGateway.UpdateServiceBackend(ctx, backend))
}

// Delete removes an AIServiceBackend from K8s
func (s *AIServiceBackendService) Delete(ctx context.Context, namespace, name string) error {
	if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
		return domain.ErrAIGatewayNotAvailable
	}
	return mapK8sError(s.aiGateway.DeleteServiceBackend(ctx, namespace, name))
}

// Exists checks if an AIServiceBackend exists
func (s *AIServiceBackendService) Exists(ctx context.Context, namespace, name string) bool {
	_, err := s.Get(ctx, namespace, name)
	return err == nil
}

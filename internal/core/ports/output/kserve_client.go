package ports

import (
	"context"

	"model-registry-service/internal/core/domain"
)

// KServeDeployment represents the result of a KServe deployment
type KServeDeployment struct {
	ExternalID string // K8s resource UID
	URL        string // Inference endpoint URL (if ready)
}

// KServeStatus represents the status of a KServe InferenceService
type KServeStatus struct {
	URL   string
	Ready bool
	Error string
}

// KServeClient defines the contract for KServe/K8s operations
type KServeClient interface {
	// Deploy creates a KServe InferenceService CR in Kubernetes
	Deploy(ctx context.Context, namespace string, isvc *domain.InferenceService, version *domain.ModelVersion) (*KServeDeployment, error)

	// Undeploy deletes the KServe InferenceService CR from Kubernetes
	Undeploy(ctx context.Context, namespace, name string) error

	// GetStatus retrieves current deployment status from Kubernetes
	GetStatus(ctx context.Context, namespace, name string) (*KServeStatus, error)

	// IsAvailable checks if KServe integration is enabled and configured
	IsAvailable() bool
}

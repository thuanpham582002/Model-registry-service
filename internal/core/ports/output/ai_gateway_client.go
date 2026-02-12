package ports

import (
	"context"
)

// ============================================================================
// AI Gateway Route Types
// ============================================================================

// AIGatewayRoute represents an AI Gateway routing configuration
type AIGatewayRoute struct {
	Name            string            // Route CR name
	Namespace       string            // K8s namespace
	ModelName       string            // x-ai-eg-model header match value
	Backends        []WeightedBackend // Backend references with weights
	RateLimit       *RateLimitConfig  // Optional rate limiting config
	Labels          map[string]string // K8s labels
	LLMRequestCosts []LLMRequestCost  // Token counting configuration
}

// WeightedBackend represents a backend with traffic weight and optional model override
type WeightedBackend struct {
	Name              string  // AIServiceBackend name
	Namespace         string  // AIServiceBackend namespace (empty = same as route)
	Weight            int     // Traffic weight 0-100 (default 1)
	Priority          int     // Priority for fallback: 0 = primary, 1+ = fallback tiers
	VariantTag        string  // Variant identifier: stable, canary, shadow, etc.
	ModelNameOverride *string // Override model name sent to upstream (nil = use original)
}

// LLMRequestCost configures token cost capture in Envoy metadata
type LLMRequestCost struct {
	MetadataKey string // Key in io.envoy.ai_gateway namespace
	Type        string // InputToken, OutputToken, TotalToken, CachedInputToken, CEL
	CEL         string // CEL expression (only for Type=CEL)
}

// AIGatewayRouteStatus represents the status of a route
type AIGatewayRouteStatus struct {
	Ready       bool
	Conditions  []string
	LastUpdated string
}

// ============================================================================
// AI Service Backend Types
// ============================================================================

// AIServiceBackend represents an AI Gateway backend configuration
type AIServiceBackend struct {
	Name           string            // Backend CR name
	Namespace      string            // K8s namespace
	Schema         string            // API schema: OpenAI, Anthropic, AWSBedrock, etc.
	BackendRef     BackendRef        // Reference to Envoy Gateway Backend
	HeaderMutation *HeaderMutation   // Optional header mutation for API key injection
	Labels         map[string]string // K8s labels
}

// HeaderMutation configures header transformations for upstream requests
type HeaderMutation struct {
	Set    []HTTPHeader // Headers to set
	Remove []string     // Headers to remove
}

// HTTPHeader represents a single HTTP header
type HTTPHeader struct {
	Name  string
	Value string
}

// BackendRef references an Envoy Gateway Backend resource
type BackendRef struct {
	Name      string // Backend resource name
	Namespace string // Backend namespace
	Group     string // API group (gateway.envoyproxy.io)
	Kind      string // Resource kind (Backend)
}

// ============================================================================
// Envoy Gateway Backend Types
// ============================================================================

// Backend represents an Envoy Gateway Backend resource
type Backend struct {
	Name      string            // Backend CR name
	Namespace string            // K8s namespace
	Endpoints []BackendEndpoint // Backend endpoints
	Labels    map[string]string // K8s labels
}

// BackendEndpoint represents a single endpoint (FQDN or IP)
type BackendEndpoint struct {
	FQDN *FQDNEndpoint // FQDN endpoint
	IP   *IPEndpoint   // IP endpoint
}

// FQDNEndpoint represents an FQDN-based endpoint
type FQDNEndpoint struct {
	Hostname string
	Port     int32
}

// IPEndpoint represents an IP-based endpoint
type IPEndpoint struct {
	Address string
	Port    int32
}

// ============================================================================
// Rate Limiting Types
// ============================================================================

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	TenantHeader   string // Header containing tenant ID (from JWT)
	TokenBudget    int64  // Total tokens per period
	RequestsPerMin int    // RPM limit
	BudgetPeriod   string // Period: Hour, Day, Week, Month
}

// ============================================================================
// AI Gateway Client Interface
// ============================================================================

// AIGatewayClient defines operations for AI Gateway CRD management
type AIGatewayClient interface {
	// Route management (AIGatewayRoute CRD)
	CreateRoute(ctx context.Context, route *AIGatewayRoute) error
	UpdateRoute(ctx context.Context, route *AIGatewayRoute) error
	DeleteRoute(ctx context.Context, namespace, name string) error
	GetRoute(ctx context.Context, namespace, name string) (*AIGatewayRoute, error)
	GetRouteStatus(ctx context.Context, namespace, name string) (*AIGatewayRouteStatus, error)

	// Backend management (AIServiceBackend CRD)
	CreateServiceBackend(ctx context.Context, backend *AIServiceBackend) error
	UpdateServiceBackend(ctx context.Context, backend *AIServiceBackend) error
	DeleteServiceBackend(ctx context.Context, namespace, name string) error
	GetServiceBackend(ctx context.Context, namespace, name string) (*AIServiceBackend, error)
	ListServiceBackends(ctx context.Context, namespace string) ([]*AIServiceBackend, error)

	// Traffic policy operations
	UpdateTrafficWeights(ctx context.Context, namespace, routeName string, backends []WeightedBackend) error

	// Rate limiting (BackendTrafficPolicy from Envoy Gateway)
	CreateRateLimitPolicy(ctx context.Context, namespace string, config *RateLimitConfig) error
	UpdateRateLimitPolicy(ctx context.Context, namespace string, config *RateLimitConfig) error

	// Envoy Gateway Backend management (Backend CRD)
	CreateBackend(ctx context.Context, backend *Backend) error
	UpdateBackend(ctx context.Context, backend *Backend) error
	DeleteBackend(ctx context.Context, namespace, name string) error
	GetBackend(ctx context.Context, namespace, name string) (*Backend, error)
	ListBackends(ctx context.Context, namespace string) ([]*Backend, error)

	// Availability check
	IsAvailable() bool
}

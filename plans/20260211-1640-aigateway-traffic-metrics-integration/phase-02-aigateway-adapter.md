# Phase 2: AI Gateway Adapter

## Objective

Create K8s client adapter for managing AI Gateway CRDs (AIGatewayRoute, AIServiceBackend) and Envoy Gateway CRDs (BackendTrafficPolicy, Backend).

---

## 2.1 Port Interface

**File**: `internal/core/ports/output/ai_gateway_client.go`

```go
package ports

import (
	"context"
	"model-registry-service/internal/core/domain"
)

// AIGatewayRoute represents an AI Gateway routing configuration
type AIGatewayRoute struct {
	Name        string
	Namespace   string
	ModelName   string           // x-ai-eg-model header match (OpenTelemetry convention)
	Backends    []WeightedBackend
	RateLimit   *RateLimitConfig
	Labels      map[string]string
	LLMRequestCosts []LLMRequestCost // Token counting configuration
}

// LLMRequestCost configures token cost capture in Envoy metadata
type LLMRequestCost struct {
	MetadataKey string // Key in io.envoy.ai_gateway namespace
	Type        string // InputToken, OutputToken, TotalToken, CachedInputToken, CEL
	CEL         string // CEL expression (only for Type=CEL)
}

// AIServiceBackend represents an AI Gateway backend configuration
type AIServiceBackend struct {
	Name        string
	Namespace   string
	Schema      string // OpenAI, Anthropic, AWSBedrock, AzureOpenAI, etc.
	BackendRef  BackendRef
	Labels      map[string]string
}

// BackendRef references an Envoy Gateway Backend resource
type BackendRef struct {
	Name      string
	Namespace string
	Group     string // gateway.envoyproxy.io
	Kind      string // Backend
}

// WeightedBackend represents a backend with traffic weight
type WeightedBackend struct {
	Name              string  // AIServiceBackend name (references KServe InferenceService)
	Namespace         string
	Weight            int     // 0-100 (default 1)
	Priority          int     // 0 = primary, 1+ = fallback
	VariantTag        string  // stable, canary, shadow
	ModelNameOverride *string // Override model name sent to upstream (nil = use original)
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	TenantHeader    string // Header containing tenant ID (from JWT)
	TokenBudget     int64  // Total tokens per period
	RequestsPerMin  int    // RPM limit
	BudgetPeriod    string // daily, weekly, monthly
}

// AIGatewayRouteStatus represents the status of a route
type AIGatewayRouteStatus struct {
	Ready       bool
	Conditions  []string
	LastUpdated string
}

// AIGatewayClient defines operations for AI Gateway management
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

	// Traffic policy
	UpdateTrafficWeights(ctx context.Context, namespace, routeName string, backends []WeightedBackend) error

	// Rate limiting (BackendTrafficPolicy from Envoy Gateway)
	CreateRateLimitPolicy(ctx context.Context, namespace string, config *RateLimitConfig) error
	UpdateRateLimitPolicy(ctx context.Context, namespace string, config *RateLimitConfig) error

	// Availability
	IsAvailable() bool
}
```

---

## 2.2 Adapter Implementation

**File**: `internal/adapters/secondary/aigateway/client.go`

```go
package aigateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"model-registry-service/internal/config"
	output "model-registry-service/internal/core/ports/output"
)

var (
	// AI Gateway CRDs
	aiGatewayRouteGVR = schema.GroupVersionResource{
		Group:    "aigateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "aigatewayroutes",
	}

	aiServiceBackendGVR = schema.GroupVersionResource{
		Group:    "aigateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "aiservicebackends",
	}

	// Envoy Gateway CRDs
	backendGVR = schema.GroupVersionResource{
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "backends",
	}

	backendTrafficPolicyGVR = schema.GroupVersionResource{
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "backendtrafficpolicies",
	}
)

type aiGatewayClient struct {
	client    dynamic.Interface
	enabled   bool
	defaultNS string
}

// NewAIGatewayClient creates a new AI Gateway client adapter
func NewAIGatewayClient(cfg *config.AIGatewayConfig) (output.AIGatewayClient, error) {
	if !cfg.Enabled {
		return &aiGatewayClient{enabled: false}, nil
	}

	var restCfg *rest.Config
	var err error

	if cfg.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else if cfg.KubeConfigPath != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfigPath)
	} else {
		home, _ := os.UserHomeDir()
		kubeconfig := filepath.Join(home, ".kube", "config")
		restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("build k8s config: %w", err)
	}

	client, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	return &aiGatewayClient{
		client:    client,
		enabled:   true,
		defaultNS: cfg.DefaultNamespace,
	}, nil
}

func (c *aiGatewayClient) IsAvailable() bool {
	return c.enabled
}

func (c *aiGatewayClient) CreateRoute(ctx context.Context, route *output.AIGatewayRoute) error {
	namespace := route.Namespace
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj := c.buildAIGatewayRouteCR(route)

	_, err := c.client.Resource(aiGatewayRouteGVR).
		Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create aigatewayroute: %w", err)
	}

	return nil
}

func (c *aiGatewayClient) UpdateRoute(ctx context.Context, route *output.AIGatewayRoute) error {
	namespace := route.Namespace
	if namespace == "" {
		namespace = c.defaultNS
	}

	// Get existing resource
	existing, err := c.client.Resource(aiGatewayRouteGVR).
		Namespace(namespace).
		Get(ctx, route.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get aigatewayroute: %w", err)
	}

	// Build updated CR
	obj := c.buildAIGatewayRouteCR(route)
	obj.SetResourceVersion(existing.GetResourceVersion())

	_, err = c.client.Resource(aiGatewayRouteGVR).
		Namespace(namespace).
		Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update aigatewayroute: %w", err)
	}

	return nil
}

func (c *aiGatewayClient) DeleteRoute(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		namespace = c.defaultNS
	}

	err := c.client.Resource(aiGatewayRouteGVR).
		Namespace(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete aigatewayroute: %w", err)
	}

	return nil
}

func (c *aiGatewayClient) GetRoute(ctx context.Context, namespace, name string) (*output.AIGatewayRoute, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj, err := c.client.Resource(aiGatewayRouteGVR).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get aigatewayroute: %w", err)
	}

	return c.parseAIGatewayRoute(obj)
}

func (c *aiGatewayClient) GetRouteStatus(ctx context.Context, namespace, name string) (*output.AIGatewayRouteStatus, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj, err := c.client.Resource(aiGatewayRouteGVR).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get aigatewayroute: %w", err)
	}

	return c.parseRouteStatus(obj), nil
}

func (c *aiGatewayClient) UpdateTrafficWeights(
	ctx context.Context,
	namespace, routeName string,
	backends []output.WeightedBackend,
) error {
	route, err := c.GetRoute(ctx, namespace, routeName)
	if err != nil {
		return err
	}

	route.Backends = backends
	return c.UpdateRoute(ctx, route)
}

func (c *aiGatewayClient) CreateRateLimitPolicy(
	ctx context.Context,
	namespace string,
	config *output.RateLimitConfig,
) error {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj := c.buildBackendTrafficPolicyCR(namespace, config)

	_, err := c.client.Resource(backendTrafficPolicyGVR).
		Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create backendtrafficpolicy: %w", err)
	}

	return nil
}

func (c *aiGatewayClient) UpdateRateLimitPolicy(
	ctx context.Context,
	namespace string,
	config *output.RateLimitConfig,
) error {
	// Similar to CreateRateLimitPolicy but with Update
	return nil
}

// buildAIGatewayRouteCR builds the AIGatewayRoute CR
func (c *aiGatewayClient) buildAIGatewayRouteCR(route *output.AIGatewayRoute) *unstructured.Unstructured {
	// Build backendRefs with optional modelNameOverride for virtual model support
	backendRefs := make([]interface{}, 0, len(route.Backends))
	for _, b := range route.Backends {
		backendRef := map[string]interface{}{
			"name":   b.Name,
			"weight": int64(b.Weight),
		}
		if b.Namespace != "" {
			backendRef["namespace"] = b.Namespace
		}
		if b.Priority > 0 {
			backendRef["priority"] = int64(b.Priority)
		}
		// modelNameOverride: Override model name sent to upstream (for virtual model)
		if b.ModelNameOverride != nil && *b.ModelNameOverride != "" {
			backendRef["modelNameOverride"] = *b.ModelNameOverride
		}
		backendRefs = append(backendRefs, backendRef)
	}

	// Build match rule using x-ai-eg-model header (AI Gateway convention)
	matches := []interface{}{}
	if route.ModelName != "" {
		matches = append(matches, map[string]interface{}{
			"headers": []interface{}{
				map[string]interface{}{
					"type":  "Exact",
					"name":  "x-ai-eg-model",
					"value": route.ModelName,
				},
			},
		})
	}

	rule := map[string]interface{}{
		"backendRefs": backendRefs,
	}
	if len(matches) > 0 {
		rule["matches"] = matches
	}

	labels := route.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["managed-by"] = "model-registry"

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aigateway.envoyproxy.io/v1alpha1",
			"kind":       "AIGatewayRoute",
			"metadata": map[string]interface{}{
				"name":   route.Name,
				"labels": labels,
			},
			"spec": map[string]interface{}{
				"parentRefs": []interface{}{
					map[string]interface{}{
						"name":      "ai-gateway",
						"namespace": "envoy-gateway-system",
					},
				},
				"rules": []interface{}{rule},
			},
		},
	}
}

// buildBackendTrafficPolicyCR builds rate limit policy
func (c *aiGatewayClient) buildBackendTrafficPolicyCR(
	namespace string,
	config *output.RateLimitConfig,
) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gateway.envoyproxy.io/v1alpha1",
			"kind":       "BackendTrafficPolicy",
			"metadata": map[string]interface{}{
				"name":      "tenant-rate-limit",
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"targetRefs": []interface{}{
					map[string]interface{}{
						"name":  "ai-gateway",
						"kind":  "Gateway",
						"group": "gateway.networking.k8s.io",
					},
				},
				"rateLimit": map[string]interface{}{
					"type": "Global",
					"global": map[string]interface{}{
						"rules": []interface{}{
							map[string]interface{}{
								"clientSelectors": []interface{}{
									map[string]interface{}{
										"headers": []interface{}{
											map[string]interface{}{
												"name": config.TenantHeader,
												"type": "Distinct",
											},
										},
									},
								},
								"limit": map[string]interface{}{
									"requests": config.TokenBudget,
									"unit":     config.BudgetPeriod,
								},
								"cost": map[string]interface{}{
									"response": map[string]interface{}{
										"from": "Metadata",
										"metadata": map[string]interface{}{
											"namespace": "io.envoy.ai_gateway",
											"key":       "llm_total_tokens",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (c *aiGatewayClient) parseAIGatewayRoute(obj *unstructured.Unstructured) (*output.AIGatewayRoute, error) {
	route := &output.AIGatewayRoute{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Labels:    obj.GetLabels(),
	}

	// Parse spec.rules[0].backendRefs
	rules, found, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	if found && len(rules) > 0 {
		rule := rules[0].(map[string]interface{})
		backendRefs, _, _ := unstructured.NestedSlice(rule, "backendRefs")

		for _, br := range backendRefs {
			brMap := br.(map[string]interface{})
			backend := output.WeightedBackend{
				Name: brMap["name"].(string),
			}
			if w, ok := brMap["weight"].(int64); ok {
				backend.Weight = int(w)
			}
			if ns, ok := brMap["namespace"].(string); ok {
				backend.Namespace = ns
			}
			route.Backends = append(route.Backends, backend)
		}
	}

	return route, nil
}

func (c *aiGatewayClient) parseRouteStatus(obj *unstructured.Unstructured) *output.AIGatewayRouteStatus {
	status := &output.AIGatewayRouteStatus{}

	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, cond := range conditions {
			condMap := cond.(map[string]interface{})
			if condMap["type"] == "Accepted" && condMap["status"] == "True" {
				status.Ready = true
			}
			if msg, ok := condMap["message"].(string); ok {
				status.Conditions = append(status.Conditions, msg)
			}
		}
	}

	return status
}

var _ output.AIGatewayClient = (*aiGatewayClient)(nil)
```

---

## 2.3 Configuration Update

**File**: `internal/config/config.go` (add)

```go
type AIGatewayConfig struct {
	Enabled          bool   `mapstructure:"AIGATEWAY_ENABLED"`
	InCluster        bool   `mapstructure:"AIGATEWAY_IN_CLUSTER"`
	KubeConfigPath   string `mapstructure:"AIGATEWAY_KUBECONFIG"`
	DefaultNamespace string `mapstructure:"AIGATEWAY_DEFAULT_NAMESPACE"`
	GatewayName      string `mapstructure:"AIGATEWAY_GATEWAY_NAME"`
	GatewayNamespace string `mapstructure:"AIGATEWAY_GATEWAY_NAMESPACE"`
}

// In Config struct add:
type Config struct {
	// ... existing fields ...
	AIGateway AIGatewayConfig
}

// In Load() add defaults:
v.SetDefault("AIGATEWAY_ENABLED", false)
v.SetDefault("AIGATEWAY_IN_CLUSTER", false)
v.SetDefault("AIGATEWAY_KUBECONFIG", "")
v.SetDefault("AIGATEWAY_DEFAULT_NAMESPACE", "model-serving")
v.SetDefault("AIGATEWAY_GATEWAY_NAME", "ai-gateway")
v.SetDefault("AIGATEWAY_GATEWAY_NAMESPACE", "envoy-gateway-system")
```

---

## 2.4 RBAC Manifest

**File**: `deploy/k8s/aigateway-rbac.yaml`

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: model-registry-aigateway
rules:
  # AI Gateway CRDs
  - apiGroups: ["aigateway.envoyproxy.io"]
    resources: ["aigatewayroutes", "aiservicebackends"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # Envoy Gateway CRDs
  - apiGroups: ["gateway.envoyproxy.io"]
    resources: ["backends", "backendtrafficpolicies"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # Gateway API (read-only)
  - apiGroups: ["gateway.networking.k8s.io"]
    resources: ["gateways", "httproutes"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: model-registry-aigateway
subjects:
  - kind: ServiceAccount
    name: model-registry
    namespace: model-registry
roleRef:
  kind: ClusterRole
  name: model-registry-aigateway
  apiGroup: rbac.authorization.k8s.io
```

---

## Resource Chain

The AI Gateway requires a specific resource chain for routing:

```
AIGatewayRoute (aigateway.envoyproxy.io/v1alpha1)
  └── backendRefs → AIServiceBackend (aigateway.envoyproxy.io/v1alpha1)
                      └── backendRef → Backend (gateway.envoyproxy.io/v1alpha1)
                                         └── endpoints → K8s Service (KServe ISVC)
```

For canary deployments:
1. Create AIServiceBackend for each variant (stable, canary)
2. Each AIServiceBackend references a Backend pointing to KServe ISVC
3. AIGatewayRoute references both AIServiceBackends with weights

---

## Checklist

- [ ] Create `internal/core/ports/output/ai_gateway_client.go`
- [ ] Create `internal/adapters/secondary/aigateway/client.go`
- [ ] Implement AIServiceBackend CRUD operations
- [ ] Implement LLMRequestCosts configuration for token counting
- [ ] Update `internal/config/config.go`
- [ ] Create `deploy/k8s/aigateway-rbac.yaml`
- [ ] Test AIGatewayRoute creation with x-ai-eg-model header matching
- [ ] Test AIServiceBackend creation with Backend reference
- [ ] Verify traffic weight updates work

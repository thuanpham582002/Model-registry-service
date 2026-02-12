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

// GVR definitions for AI Gateway CRDs
var (
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
	backendTrafficPolicyGVR = schema.GroupVersionResource{
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "backendtrafficpolicies",
	}
)

type aiGatewayClient struct {
	client           dynamic.Interface
	enabled          bool
	defaultNS        string
	gatewayName      string
	gatewayNamespace string
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

	defaultNS := cfg.DefaultNamespace
	if defaultNS == "" {
		defaultNS = "model-serving"
	}

	gatewayName := cfg.GatewayName
	if gatewayName == "" {
		gatewayName = "ai-gateway"
	}

	gatewayNS := cfg.GatewayNamespace
	if gatewayNS == "" {
		gatewayNS = "envoy-gateway-system"
	}

	return &aiGatewayClient{
		client:           client,
		enabled:          true,
		defaultNS:        defaultNS,
		gatewayName:      gatewayName,
		gatewayNamespace: gatewayNS,
	}, nil
}

func (c *aiGatewayClient) IsAvailable() bool {
	return c.enabled
}

// ============================================================================
// Route Management
// ============================================================================

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

	existing, err := c.client.Resource(aiGatewayRouteGVR).
		Namespace(namespace).
		Get(ctx, route.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get aigatewayroute: %w", err)
	}

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

// ============================================================================
// Backend Management
// ============================================================================

func (c *aiGatewayClient) CreateServiceBackend(ctx context.Context, backend *output.AIServiceBackend) error {
	namespace := backend.Namespace
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj := c.buildAIServiceBackendCR(backend)

	_, err := c.client.Resource(aiServiceBackendGVR).
		Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create aiservicebackend: %w", err)
	}

	return nil
}

func (c *aiGatewayClient) UpdateServiceBackend(ctx context.Context, backend *output.AIServiceBackend) error {
	namespace := backend.Namespace
	if namespace == "" {
		namespace = c.defaultNS
	}

	existing, err := c.client.Resource(aiServiceBackendGVR).
		Namespace(namespace).
		Get(ctx, backend.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get aiservicebackend: %w", err)
	}

	obj := c.buildAIServiceBackendCR(backend)
	obj.SetResourceVersion(existing.GetResourceVersion())

	_, err = c.client.Resource(aiServiceBackendGVR).
		Namespace(namespace).
		Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update aiservicebackend: %w", err)
	}

	return nil
}

func (c *aiGatewayClient) DeleteServiceBackend(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		namespace = c.defaultNS
	}

	err := c.client.Resource(aiServiceBackendGVR).
		Namespace(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete aiservicebackend: %w", err)
	}

	return nil
}

func (c *aiGatewayClient) GetServiceBackend(ctx context.Context, namespace, name string) (*output.AIServiceBackend, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj, err := c.client.Resource(aiServiceBackendGVR).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get aiservicebackend: %w", err)
	}

	return c.parseAIServiceBackend(obj), nil
}

func (c *aiGatewayClient) ListServiceBackends(ctx context.Context, namespace string) ([]*output.AIServiceBackend, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	list, err := c.client.Resource(aiServiceBackendGVR).
		Namespace(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list aiservicebackends: %w", err)
	}

	var backends []*output.AIServiceBackend
	for _, item := range list.Items {
		backends = append(backends, c.parseAIServiceBackend(&item))
	}
	return backends, nil
}

// ============================================================================
// Rate Limiting
// ============================================================================

func (c *aiGatewayClient) CreateRateLimitPolicy(ctx context.Context, namespace string, config *output.RateLimitConfig) error {
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

func (c *aiGatewayClient) UpdateRateLimitPolicy(ctx context.Context, namespace string, config *output.RateLimitConfig) error {
	if namespace == "" {
		namespace = c.defaultNS
	}

	policyName := "tenant-rate-limit"

	existing, err := c.client.Resource(backendTrafficPolicyGVR).
		Namespace(namespace).
		Get(ctx, policyName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get backendtrafficpolicy: %w", err)
	}

	obj := c.buildBackendTrafficPolicyCR(namespace, config)
	obj.SetResourceVersion(existing.GetResourceVersion())

	_, err = c.client.Resource(backendTrafficPolicyGVR).
		Namespace(namespace).
		Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update backendtrafficpolicy: %w", err)
	}

	return nil
}

// ============================================================================
// CR Builders
// ============================================================================

func (c *aiGatewayClient) buildAIGatewayRouteCR(route *output.AIGatewayRoute) *unstructured.Unstructured {
	// Build backendRefs with optional modelNameOverride
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

	// Build match rule using x-ai-eg-model header
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

	// Convert labels to interface{}
	labelsInterface := make(map[string]interface{})
	for k, v := range labels {
		labelsInterface[k] = v
	}

	spec := map[string]interface{}{
		"parentRefs": []interface{}{
			map[string]interface{}{
				"name":      c.gatewayName,
				"namespace": c.gatewayNamespace,
			},
		},
		"rules": []interface{}{rule},
	}

	// Add LLMRequestCosts if configured
	if len(route.LLMRequestCosts) > 0 {
		costs := make([]interface{}, 0, len(route.LLMRequestCosts))
		for _, cost := range route.LLMRequestCosts {
			costMap := map[string]interface{}{
				"metadataKey": cost.MetadataKey,
				"type":        cost.Type,
			}
			if cost.CEL != "" {
				costMap["cel"] = cost.CEL
			}
			costs = append(costs, costMap)
		}
		spec["llmRequestCosts"] = costs
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aigateway.envoyproxy.io/v1alpha1",
			"kind":       "AIGatewayRoute",
			"metadata": map[string]interface{}{
				"name":   route.Name,
				"labels": labelsInterface,
			},
			"spec": spec,
		},
	}
}

func (c *aiGatewayClient) buildAIServiceBackendCR(backend *output.AIServiceBackend) *unstructured.Unstructured {
	labels := backend.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["managed-by"] = "model-registry"

	labelsInterface := make(map[string]interface{})
	for k, v := range labels {
		labelsInterface[k] = v
	}

	spec := map[string]interface{}{
		"schema": map[string]interface{}{
			"name": backend.Schema,
		},
	}

	// Add backendRef if specified
	if backend.BackendRef.Name != "" {
		backendRef := map[string]interface{}{
			"name": backend.BackendRef.Name,
		}
		if backend.BackendRef.Namespace != "" {
			backendRef["namespace"] = backend.BackendRef.Namespace
		}
		if backend.BackendRef.Group != "" {
			backendRef["group"] = backend.BackendRef.Group
		}
		if backend.BackendRef.Kind != "" {
			backendRef["kind"] = backend.BackendRef.Kind
		}
		spec["backendRef"] = backendRef
	}

	// Add headerMutation if specified
	if backend.HeaderMutation != nil {
		headerMutation := map[string]interface{}{}
		if len(backend.HeaderMutation.Set) > 0 {
			setHeaders := make([]interface{}, 0, len(backend.HeaderMutation.Set))
			for _, h := range backend.HeaderMutation.Set {
				setHeaders = append(setHeaders, map[string]interface{}{
					"name":  h.Name,
					"value": h.Value,
				})
			}
			headerMutation["set"] = setHeaders
		}
		if len(backend.HeaderMutation.Remove) > 0 {
			headerMutation["remove"] = backend.HeaderMutation.Remove
		}
		if len(headerMutation) > 0 {
			spec["headerMutation"] = headerMutation
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aigateway.envoyproxy.io/v1alpha1",
			"kind":       "AIServiceBackend",
			"metadata": map[string]interface{}{
				"name":   backend.Name,
				"labels": labelsInterface,
			},
			"spec": spec,
		},
	}
}

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
						"name":  c.gatewayName,
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

// ============================================================================
// Parsers
// ============================================================================

func (c *aiGatewayClient) parseAIGatewayRoute(obj *unstructured.Unstructured) (*output.AIGatewayRoute, error) {
	route := &output.AIGatewayRoute{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Labels:    obj.GetLabels(),
	}

	// Parse spec.rules[0].backendRefs
	rules, found, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	if found && len(rules) > 0 {
		rule, ok := rules[0].(map[string]interface{})
		if ok {
			backendRefs, _, _ := unstructured.NestedSlice(rule, "backendRefs")
			for _, br := range backendRefs {
				brMap, ok := br.(map[string]interface{})
				if !ok {
					continue
				}
				backend := output.WeightedBackend{}
				if name, ok := brMap["name"].(string); ok {
					backend.Name = name
				}
				if w, ok := brMap["weight"].(int64); ok {
					backend.Weight = int(w)
				}
				if ns, ok := brMap["namespace"].(string); ok {
					backend.Namespace = ns
				}
				if p, ok := brMap["priority"].(int64); ok {
					backend.Priority = int(p)
				}
				if override, ok := brMap["modelNameOverride"].(string); ok {
					backend.ModelNameOverride = &override
				}
				route.Backends = append(route.Backends, backend)
			}

			// Parse matches for model name
			matches, _, _ := unstructured.NestedSlice(rule, "matches")
			if len(matches) > 0 {
				match, ok := matches[0].(map[string]interface{})
				if ok {
					headers, _, _ := unstructured.NestedSlice(match, "headers")
					if len(headers) > 0 {
						header, ok := headers[0].(map[string]interface{})
						if ok {
							if name, ok := header["name"].(string); ok && name == "x-ai-eg-model" {
								if value, ok := header["value"].(string); ok {
									route.ModelName = value
								}
							}
						}
					}
				}
			}
		}
	}

	return route, nil
}

func (c *aiGatewayClient) parseAIServiceBackend(obj *unstructured.Unstructured) *output.AIServiceBackend {
	backend := &output.AIServiceBackend{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Labels:    obj.GetLabels(),
	}

	// Parse schema
	schema, _, _ := unstructured.NestedString(obj.Object, "spec", "schema", "name")
	backend.Schema = schema

	// Parse backendRef
	backendRefMap, found, _ := unstructured.NestedMap(obj.Object, "spec", "backendRef")
	if found {
		if name, ok := backendRefMap["name"].(string); ok {
			backend.BackendRef.Name = name
		}
		if ns, ok := backendRefMap["namespace"].(string); ok {
			backend.BackendRef.Namespace = ns
		}
		if group, ok := backendRefMap["group"].(string); ok {
			backend.BackendRef.Group = group
		}
		if kind, ok := backendRefMap["kind"].(string); ok {
			backend.BackendRef.Kind = kind
		}
	}

	// Parse headerMutation
	_, found, _ = unstructured.NestedMap(obj.Object, "spec", "headerMutation")
	if found {
		backend.HeaderMutation = &output.HeaderMutation{}

		// Parse set headers
		setHeaders, _, _ := unstructured.NestedSlice(obj.Object, "spec", "headerMutation", "set")
		for _, h := range setHeaders {
			hMap, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			header := output.HTTPHeader{}
			if name, ok := hMap["name"].(string); ok {
				header.Name = name
			}
			if value, ok := hMap["value"].(string); ok {
				header.Value = value
			}
			backend.HeaderMutation.Set = append(backend.HeaderMutation.Set, header)
		}

		// Parse remove headers
		removeHeaders, _, _ := unstructured.NestedStringSlice(obj.Object, "spec", "headerMutation", "remove")
		backend.HeaderMutation.Remove = removeHeaders

		// Clear if empty
		if len(backend.HeaderMutation.Set) == 0 && len(backend.HeaderMutation.Remove) == 0 {
			backend.HeaderMutation = nil
		}
	}

	return backend
}

func (c *aiGatewayClient) parseRouteStatus(obj *unstructured.Unstructured) *output.AIGatewayRouteStatus {
	status := &output.AIGatewayRouteStatus{}

	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := condMap["type"].(string)
			condStatus, _ := condMap["status"].(string)

			if condType == "Accepted" && condStatus == "True" {
				status.Ready = true
			}
			if msg, ok := condMap["message"].(string); ok {
				status.Conditions = append(status.Conditions, msg)
			}
		}
	}

	return status
}

// Ensure interface compliance
var _ output.AIGatewayClient = (*aiGatewayClient)(nil)

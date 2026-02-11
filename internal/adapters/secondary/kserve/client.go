package kserve

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
	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

var inferenceServiceGVR = schema.GroupVersionResource{
	Group:    "serving.kserve.io",
	Version:  "v1beta1",
	Resource: "inferenceservices",
}

type kserveClient struct {
	client    dynamic.Interface
	enabled   bool
	defaultNS string
}

// NewKServeClient creates a new KServe client adapter
func NewKServeClient(cfg *config.KubernetesConfig) (output.KServeClient, error) {
	if !cfg.Enabled {
		return &kserveClient{enabled: false}, nil
	}

	var restCfg *rest.Config
	var err error

	if cfg.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else if cfg.KubeConfigPath != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfigPath)
	} else {
		// Try default kubeconfig location
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

	defaultNS := cfg.DefaultNS
	if defaultNS == "" {
		defaultNS = "model-serving"
	}

	return &kserveClient{
		client:    client,
		enabled:   true,
		defaultNS: defaultNS,
	}, nil
}

func (c *kserveClient) IsAvailable() bool {
	return c.enabled
}

func (c *kserveClient) Deploy(
	ctx context.Context,
	namespace string,
	isvc *domain.InferenceService,
	version *domain.ModelVersion,
) (*output.KServeDeployment, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj := c.buildInferenceServiceCR(isvc, version)

	created, err := c.client.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create kserve inferenceservice: %w", err)
	}

	return &output.KServeDeployment{
		ExternalID: string(created.GetUID()),
	}, nil
}

func (c *kserveClient) Undeploy(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		namespace = c.defaultNS
	}

	err := c.client.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete kserve inferenceservice: %w", err)
	}

	return nil
}

func (c *kserveClient) GetStatus(ctx context.Context, namespace, name string) (*output.KServeStatus, error) {
	if namespace == "" {
		namespace = c.defaultNS
	}

	obj, err := c.client.Resource(inferenceServiceGVR).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get kserve inferenceservice: %w", err)
	}

	return c.parseStatus(obj), nil
}

func (c *kserveClient) buildInferenceServiceCR(
	isvc *domain.InferenceService,
	version *domain.ModelVersion,
) *unstructured.Unstructured {
	labels := map[string]interface{}{
		"modelregistry.ai-platform/inference-service-id": isvc.ID.String(),
		"modelregistry.ai-platform/registered-model-id":  isvc.RegisteredModelID.String(),
	}
	// Add version ID from the version parameter (supports multi-model via serve_model)
	if version != nil {
		labels["modelregistry.ai-platform/model-version-id"] = version.ID.String()
	}

	// Merge user labels
	for k, v := range isvc.Labels {
		labels[k] = v
	}

	// Build predictor spec
	modelSpec := map[string]interface{}{
		"storageUri": version.URI,
	}

	// Add model format if specified
	if version.ModelFramework != "" {
		modelSpec["modelFormat"] = map[string]interface{}{
			"name": version.ModelFramework,
		}
		if version.ModelFrameworkVersion != "" {
			modelSpec["modelFormat"].(map[string]interface{})["version"] = version.ModelFrameworkVersion
		}
	}

	// Add runtime/container image if specified
	if version.ContainerImage != "" {
		modelSpec["runtime"] = version.ContainerImage
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":   isvc.Name,
				"labels": labels,
			},
			"spec": map[string]interface{}{
				"predictor": map[string]interface{}{
					"model": modelSpec,
				},
			},
		},
	}

	return obj
}

func (c *kserveClient) parseStatus(obj *unstructured.Unstructured) *output.KServeStatus {
	status := &output.KServeStatus{}

	statusMap, found, _ := unstructured.NestedMap(obj.Object, "status")
	if !found {
		return status
	}

	// Get URL
	status.URL, _, _ = unstructured.NestedString(statusMap, "url")

	// Check conditions for ready state
	conditions, found, _ := unstructured.NestedSlice(statusMap, "conditions")
	if found {
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := condMap["type"].(string)
			condStatus, _ := condMap["status"].(string)

			if condType == "Ready" {
				status.Ready = condStatus == "True"
				if condStatus == "False" {
					if msg, ok := condMap["message"].(string); ok {
						status.Error = msg
					}
				}
				break
			}
		}
	}

	return status
}

// Ensure interface compliance
var _ output.KServeClient = (*kserveClient)(nil)

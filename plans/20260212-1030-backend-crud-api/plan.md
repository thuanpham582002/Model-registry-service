# Implementation Plan: Backend CRUD API (Envoy Gateway Backend)

**Date:** 2026-02-12
**Author:** Planning Agent
**Status:** ✅ IMPLEMENTED

## Problem Statement

Currently, users can manage AIServiceBackend and VirtualModel resources via API, but Envoy Gateway Backend resources must be created manually via `kubectl`. This breaks the "100% API control" goal where users can manage the entire AI platform programmatically (like `gcloud ai` CLI).

## Solution Overview

Add `/backends` CRUD API to create Envoy Gateway Backend resources. This enables the full flow:

```
POST /backends → POST /ai_service_backends → POST /virtual_models
```

**Backend CRD Structure (Envoy Gateway):**
```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: Backend
metadata:
  name: kserve-llama
spec:
  endpoints:
    - fqdn:
        hostname: llama.svc.cluster.local
        port: 80
    # OR
    - ip:
        address: 10.0.0.1
        port: 80
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                           HTTP Layer                                 │
├─────────────────────────────────────────────────────────────────────┤
│  handlers/backend.go          dto/backend.go                        │
│  - ListBackends               - CreateBackendRequest                │
│  - GetBackend                 - BackendResponse                     │
│  - CreateBackend              - EndpointRequest                     │
│  - UpdateBackend              - converters                          │
│  - DeleteBackend                                                    │
└──────────────────────────────────┬──────────────────────────────────┘
                                   │
┌──────────────────────────────────▼──────────────────────────────────┐
│                          Service Layer                               │
├─────────────────────────────────────────────────────────────────────┤
│  services/backend.go                                                 │
│  - BackendService                                                    │
│  - Create, Get, List, Update, Delete                                 │
│  - mapK8sError (reuse from ai_service_backend.go)                    │
└──────────────────────────────────┬──────────────────────────────────┘
                                   │
┌──────────────────────────────────▼──────────────────────────────────┐
│                          Ports Layer                                 │
├─────────────────────────────────────────────────────────────────────┤
│  ports/output/ai_gateway_client.go                                   │
│  + Backend type definitions                                          │
│  + AIGatewayClient interface extensions:                             │
│    - CreateBackend(ctx, *Backend) error                              │
│    - GetBackend(ctx, namespace, name) (*Backend, error)              │
│    - ListBackends(ctx, namespace) ([]*Backend, error)                │
│    - UpdateBackend(ctx, *Backend) error                              │
│    - DeleteBackend(ctx, namespace, name) error                       │
└──────────────────────────────────┬──────────────────────────────────┘
                                   │
┌──────────────────────────────────▼──────────────────────────────────┐
│                       Adapter Layer (K8s)                            │
├─────────────────────────────────────────────────────────────────────┤
│  adapters/secondary/aigateway/client.go                              │
│  + backendGVR definition                                             │
│  + buildBackendCR, parseBackend methods                              │
│  + Implement Backend CRUD methods                                    │
└─────────────────────────────────────────────────────────────────────┘
```

## Implementation Tasks

### Task 1: Domain Types (ports/output/ai_gateway_client.go)

Add Backend type definitions:

```go
// Backend Types
// ============================================================================

// Backend represents an Envoy Gateway Backend resource
type Backend struct {
    Name      string            // Backend CR name
    Namespace string            // K8s namespace
    Endpoints []BackendEndpoint // Backend endpoints
    Labels    map[string]string // K8s labels
}

// BackendEndpoint represents a single endpoint (FQDN, IP, or UDS)
type BackendEndpoint struct {
    // FQDN endpoint
    FQDN *FQDNEndpoint `json:"fqdn,omitempty"`
    // IP endpoint
    IP *IPEndpoint `json:"ip,omitempty"`
}

// FQDNEndpoint represents an FQDN-based endpoint
type FQDNEndpoint struct {
    Hostname string `json:"hostname"`
    Port     int32  `json:"port"`
}

// IPEndpoint represents an IP-based endpoint
type IPEndpoint struct {
    Address string `json:"address"`
    Port    int32  `json:"port"`
}
```

Extend AIGatewayClient interface:

```go
// Backend management (Envoy Gateway Backend CRD)
CreateBackend(ctx context.Context, backend *Backend) error
UpdateBackend(ctx context.Context, backend *Backend) error
DeleteBackend(ctx context.Context, namespace, name string) error
GetBackend(ctx context.Context, namespace, name string) (*Backend, error)
ListBackends(ctx context.Context, namespace string) ([]*Backend, error)
```

---

### Task 2: Domain Errors (domain/errors.go) ✅ DONE

Added Backend-specific errors to avoid naming collision with VirtualModel:

```go
// Envoy Gateway Backend Errors
// ============================================================================

var (
    ErrEnvoyBackendNotFound      = errors.New("envoy gateway backend not found")
    ErrEnvoyBackendAlreadyExists = errors.New("envoy gateway backend already exists")
)
```

**Note:** These errors have been added and error-mapper.go has been updated to handle them.

---

### Task 3: K8s Client Implementation (adapters/secondary/aigateway/client.go)

Add GVR definition:

```go
var (
    // Existing GVRs...

    // Envoy Gateway Backend CRD
    backendGVR = schema.GroupVersionResource{
        Group:    "gateway.envoyproxy.io",
        Version:  "v1alpha1",
        Resource: "backends",
    }
)
```

Implement CRUD methods:

```go
// ============================================================================
// Envoy Gateway Backend Management
// ============================================================================

func (c *aiGatewayClient) CreateBackend(ctx context.Context, backend *output.Backend) error {
    namespace := backend.Namespace
    if namespace == "" {
        namespace = c.defaultNS
    }
    obj := c.buildBackendCR(backend)
    _, err := c.client.Resource(backendGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
    if err != nil {
        return fmt.Errorf("create backend: %w", err)
    }
    return nil
}

func (c *aiGatewayClient) GetBackend(ctx context.Context, namespace, name string) (*output.Backend, error) {
    if namespace == "" {
        namespace = c.defaultNS
    }
    obj, err := c.client.Resource(backendGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return nil, fmt.Errorf("get backend: %w", err)
    }
    return c.parseBackend(obj), nil
}

func (c *aiGatewayClient) ListBackends(ctx context.Context, namespace string) ([]*output.Backend, error) {
    if namespace == "" {
        namespace = c.defaultNS
    }
    list, err := c.client.Resource(backendGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("list backends: %w", err)
    }
    var backends []*output.Backend
    for _, item := range list.Items {
        backends = append(backends, c.parseBackend(&item))
    }
    return backends, nil
}

func (c *aiGatewayClient) UpdateBackend(ctx context.Context, backend *output.Backend) error {
    namespace := backend.Namespace
    if namespace == "" {
        namespace = c.defaultNS
    }
    existing, err := c.client.Resource(backendGVR).Namespace(namespace).Get(ctx, backend.Name, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("get backend: %w", err)
    }
    obj := c.buildBackendCR(backend)
    obj.SetResourceVersion(existing.GetResourceVersion())
    _, err = c.client.Resource(backendGVR).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
    if err != nil {
        return fmt.Errorf("update backend: %w", err)
    }
    return nil
}

func (c *aiGatewayClient) DeleteBackend(ctx context.Context, namespace, name string) error {
    if namespace == "" {
        namespace = c.defaultNS
    }
    err := c.client.Resource(backendGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
    if err != nil {
        return fmt.Errorf("delete backend: %w", err)
    }
    return nil
}
```

CR Builder:

```go
func (c *aiGatewayClient) buildBackendCR(backend *output.Backend) *unstructured.Unstructured {
    labels := backend.Labels
    if labels == nil {
        labels = make(map[string]string)
    }
    labels["managed-by"] = "model-registry"

    labelsInterface := make(map[string]interface{})
    for k, v := range labels {
        labelsInterface[k] = v
    }

    // Build endpoints
    endpoints := make([]interface{}, 0, len(backend.Endpoints))
    for _, ep := range backend.Endpoints {
        var endpoint map[string]interface{}
        if ep.FQDN != nil {
            endpoint = map[string]interface{}{
                "fqdn": map[string]interface{}{
                    "hostname": ep.FQDN.Hostname,
                    "port":     int64(ep.FQDN.Port),
                },
            }
        } else if ep.IP != nil {
            endpoint = map[string]interface{}{
                "ip": map[string]interface{}{
                    "address": ep.IP.Address,
                    "port":    int64(ep.IP.Port),
                },
            }
        }
        if endpoint != nil {
            endpoints = append(endpoints, endpoint)
        }
    }

    return &unstructured.Unstructured{
        Object: map[string]interface{}{
            "apiVersion": "gateway.envoyproxy.io/v1alpha1",
            "kind":       "Backend",
            "metadata": map[string]interface{}{
                "name":   backend.Name,
                "labels": labelsInterface,
            },
            "spec": map[string]interface{}{
                "endpoints": endpoints,
            },
        },
    }
}
```

CR Parser:

```go
func (c *aiGatewayClient) parseBackend(obj *unstructured.Unstructured) *output.Backend {
    backend := &output.Backend{
        Name:      obj.GetName(),
        Namespace: obj.GetNamespace(),
        Labels:    obj.GetLabels(),
    }

    endpoints, found, _ := unstructured.NestedSlice(obj.Object, "spec", "endpoints")
    if found {
        for _, ep := range endpoints {
            epMap, ok := ep.(map[string]interface{})
            if !ok {
                continue
            }
            be := output.BackendEndpoint{}

            // Parse FQDN endpoint
            if fqdn, ok := epMap["fqdn"].(map[string]interface{}); ok {
                be.FQDN = &output.FQDNEndpoint{}
                if hostname, ok := fqdn["hostname"].(string); ok {
                    be.FQDN.Hostname = hostname
                }
                if port, ok := fqdn["port"].(int64); ok {
                    be.FQDN.Port = int32(port)
                }
            }

            // Parse IP endpoint
            if ip, ok := epMap["ip"].(map[string]interface{}); ok {
                be.IP = &output.IPEndpoint{}
                if addr, ok := ip["address"].(string); ok {
                    be.IP.Address = addr
                }
                if port, ok := ip["port"].(int64); ok {
                    be.IP.Port = int32(port)
                }
            }

            backend.Endpoints = append(backend.Endpoints, be)
        }
    }

    return backend
}
```

---

### Task 4: Service Layer (services/backend.go)

Create new service file:

```go
package services

import (
    "context"

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

// Create creates a new Backend in K8s
func (s *BackendService) Create(ctx context.Context, backend *output.Backend) error {
    if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
        return domain.ErrAIGatewayNotAvailable
    }
    return mapK8sError(s.aiGateway.CreateBackend(ctx, backend))
}

// Get retrieves a Backend from K8s
func (s *BackendService) Get(ctx context.Context, namespace, name string) (*output.Backend, error) {
    if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
        return nil, domain.ErrAIGatewayNotAvailable
    }
    backend, err := s.aiGateway.GetBackend(ctx, namespace, name)
    return backend, mapK8sError(err)
}

// List lists all Backends in a namespace
func (s *BackendService) List(ctx context.Context, namespace string) ([]*output.Backend, error) {
    if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
        return nil, domain.ErrAIGatewayNotAvailable
    }
    backends, err := s.aiGateway.ListBackends(ctx, namespace)
    return backends, mapK8sError(err)
}

// Update updates an existing Backend
func (s *BackendService) Update(ctx context.Context, backend *output.Backend) error {
    if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
        return domain.ErrAIGatewayNotAvailable
    }
    return mapK8sError(s.aiGateway.UpdateBackend(ctx, backend))
}

// Delete removes a Backend from K8s
func (s *BackendService) Delete(ctx context.Context, namespace, name string) error {
    if s.aiGateway == nil || !s.aiGateway.IsAvailable() {
        return domain.ErrAIGatewayNotAvailable
    }
    return mapK8sError(s.aiGateway.DeleteBackend(ctx, namespace, name))
}

// Exists checks if a Backend exists
func (s *BackendService) Exists(ctx context.Context, namespace, name string) bool {
    _, err := s.Get(ctx, namespace, name)
    return err == nil
}
```

---

### Task 5: DTOs (dto/backend.go)

Create new DTO file:

```go
package dto

import (
    ports "model-registry-service/internal/core/ports/output"
)

// ============================================================================
// Request DTOs
// ============================================================================

// CreateBackendRequest represents the request to create an Envoy Gateway Backend
type CreateBackendRequest struct {
    Name      string            `json:"name" binding:"required"`
    Namespace string            `json:"namespace"`
    Endpoints []EndpointRequest `json:"endpoints" binding:"required,min=1"`
    Labels    map[string]string `json:"labels"`
}

// EndpointRequest represents a single endpoint in the request
type EndpointRequest struct {
    // FQDN endpoint (hostname:port)
    FQDN *FQDNEndpointRequest `json:"fqdn,omitempty"`
    // IP endpoint (address:port)
    IP *IPEndpointRequest `json:"ip,omitempty"`
}

// FQDNEndpointRequest represents an FQDN endpoint
type FQDNEndpointRequest struct {
    Hostname string `json:"hostname" binding:"required"`
    Port     int32  `json:"port" binding:"required,min=1,max=65535"`
}

// IPEndpointRequest represents an IP endpoint
type IPEndpointRequest struct {
    Address string `json:"address" binding:"required,ip"`
    Port    int32  `json:"port" binding:"required,min=1,max=65535"`
}

// UpdateBackendRequest represents the request to update a Backend
type UpdateBackendRequest struct {
    Endpoints []EndpointRequest `json:"endpoints"`
    Labels    map[string]string `json:"labels"`
}

// ============================================================================
// Response DTOs
// ============================================================================

// BackendResponse represents a Backend in API responses
type BackendResponse struct {
    Name      string             `json:"name"`
    Namespace string             `json:"namespace"`
    Endpoints []EndpointResponse `json:"endpoints"`
    Labels    map[string]string  `json:"labels,omitempty"`
}

// EndpointResponse represents an endpoint in responses
type EndpointResponse struct {
    FQDN *FQDNEndpointResponse `json:"fqdn,omitempty"`
    IP   *IPEndpointResponse   `json:"ip,omitempty"`
}

// FQDNEndpointResponse represents an FQDN endpoint in responses
type FQDNEndpointResponse struct {
    Hostname string `json:"hostname"`
    Port     int32  `json:"port"`
}

// IPEndpointResponse represents an IP endpoint in responses
type IPEndpointResponse struct {
    Address string `json:"address"`
    Port    int32  `json:"port"`
}

// ListBackendsResponse represents the list response
type ListBackendsResponse struct {
    Items []BackendResponse `json:"items"`
    Total int               `json:"total"`
}

// ============================================================================
// Converters
// ============================================================================

// ToBackend converts CreateBackendRequest to ports.Backend
func ToBackend(req *CreateBackendRequest) *ports.Backend {
    backend := &ports.Backend{
        Name:      req.Name,
        Namespace: req.Namespace,
        Labels:    req.Labels,
    }

    for _, ep := range req.Endpoints {
        be := ports.BackendEndpoint{}
        if ep.FQDN != nil {
            be.FQDN = &ports.FQDNEndpoint{
                Hostname: ep.FQDN.Hostname,
                Port:     ep.FQDN.Port,
            }
        }
        if ep.IP != nil {
            be.IP = &ports.IPEndpoint{
                Address: ep.IP.Address,
                Port:    ep.IP.Port,
            }
        }
        backend.Endpoints = append(backend.Endpoints, be)
    }

    return backend
}

// ToBackendResponse converts ports.Backend to BackendResponse
func ToBackendResponse(backend *ports.Backend) BackendResponse {
    resp := BackendResponse{
        Name:      backend.Name,
        Namespace: backend.Namespace,
        Labels:    backend.Labels,
    }

    for _, ep := range backend.Endpoints {
        er := EndpointResponse{}
        if ep.FQDN != nil {
            er.FQDN = &FQDNEndpointResponse{
                Hostname: ep.FQDN.Hostname,
                Port:     ep.FQDN.Port,
            }
        }
        if ep.IP != nil {
            er.IP = &IPEndpointResponse{
                Address: ep.IP.Address,
                Port:    ep.IP.Port,
            }
        }
        resp.Endpoints = append(resp.Endpoints, er)
    }

    return resp
}

// EndpointRequestToPorts converts EndpointRequest to ports.BackendEndpoint
func EndpointRequestToPorts(ep *EndpointRequest) ports.BackendEndpoint {
    be := ports.BackendEndpoint{}
    if ep.FQDN != nil {
        be.FQDN = &ports.FQDNEndpoint{
            Hostname: ep.FQDN.Hostname,
            Port:     ep.FQDN.Port,
        }
    }
    if ep.IP != nil {
        be.IP = &ports.IPEndpoint{
            Address: ep.IP.Address,
            Port:    ep.IP.Port,
        }
    }
    return be
}
```

---

### Task 6: HTTP Handlers (handlers/backend.go)

Create new handler file:

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    log "github.com/sirupsen/logrus"

    "model-registry-service/internal/adapters/primary/http/dto"
)

// ListBackends lists all Backends in a namespace
func (h *Handler) ListBackends(c *gin.Context) {
    namespace := c.DefaultQuery("namespace", "model-serving")

    backends, err := h.backendSvc.List(c.Request.Context(), namespace)
    if err != nil {
        log.WithError(err).Error("list backends failed")
        mapDomainError(c, err)
        return
    }

    items := make([]dto.BackendResponse, 0, len(backends))
    for _, b := range backends {
        items = append(items, dto.ToBackendResponse(b))
    }

    c.JSON(http.StatusOK, dto.ListBackendsResponse{
        Items: items,
        Total: len(items),
    })
}

// GetBackend retrieves a single Backend
func (h *Handler) GetBackend(c *gin.Context) {
    name := c.Param("name")
    namespace := c.DefaultQuery("namespace", "model-serving")

    backend, err := h.backendSvc.Get(c.Request.Context(), namespace, name)
    if err != nil {
        log.WithError(err).Error("get backend failed")
        mapDomainError(c, err)
        return
    }

    c.JSON(http.StatusOK, dto.ToBackendResponse(backend))
}

// CreateBackend creates a new Backend
func (h *Handler) CreateBackend(c *gin.Context) {
    var req dto.CreateBackendRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Validate: at least one endpoint with valid type
    for i, ep := range req.Endpoints {
        if ep.FQDN == nil && ep.IP == nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint must have fqdn or ip", "index": i})
            return
        }
        if ep.FQDN != nil && ep.IP != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint cannot have both fqdn and ip", "index": i})
            return
        }
    }

    backend := dto.ToBackend(&req)
    if backend.Namespace == "" {
        backend.Namespace = "model-serving"
    }

    if err := h.backendSvc.Create(c.Request.Context(), backend); err != nil {
        log.WithError(err).Error("create backend failed")
        mapDomainError(c, err)
        return
    }

    // Fetch created backend to get full response
    created, _ := h.backendSvc.Get(c.Request.Context(), backend.Namespace, backend.Name)
    if created != nil {
        c.JSON(http.StatusCreated, dto.ToBackendResponse(created))
    } else {
        c.JSON(http.StatusCreated, dto.ToBackendResponse(backend))
    }
}

// UpdateBackend updates an existing Backend
func (h *Handler) UpdateBackend(c *gin.Context) {
    name := c.Param("name")
    namespace := c.DefaultQuery("namespace", "model-serving")

    var req dto.UpdateBackendRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Get existing
    existing, err := h.backendSvc.Get(c.Request.Context(), namespace, name)
    if err != nil {
        mapDomainError(c, err)
        return
    }

    // Apply updates
    if req.Endpoints != nil {
        existing.Endpoints = nil
        for _, ep := range req.Endpoints {
            existing.Endpoints = append(existing.Endpoints, dto.EndpointRequestToPorts(&ep))
        }
    }
    if req.Labels != nil {
        existing.Labels = req.Labels
    }

    if err := h.backendSvc.Update(c.Request.Context(), existing); err != nil {
        log.WithError(err).Error("update backend failed")
        mapDomainError(c, err)
        return
    }

    c.JSON(http.StatusOK, dto.ToBackendResponse(existing))
}

// DeleteBackend deletes a Backend
func (h *Handler) DeleteBackend(c *gin.Context) {
    name := c.Param("name")
    namespace := c.DefaultQuery("namespace", "model-serving")

    if err := h.backendSvc.Delete(c.Request.Context(), namespace, name); err != nil {
        log.WithError(err).Error("delete backend failed")
        mapDomainError(c, err)
        return
    }

    c.Status(http.StatusNoContent)
}
```

---

### Task 7: Update Handler Struct (handlers/handler.go)

Add backendSvc to Handler:

```go
type Handler struct {
    // ... existing fields ...
    backendSvc      *services.BackendService  // NEW
}

func New(
    // ... existing params ...
    backendSvc *services.BackendService,  // NEW
) *Handler {
    return &Handler{
        // ... existing fields ...
        backendSvc:      backendSvc,
    }
}
```

Add routes in RegisterRoutes:

```go
// Envoy Gateway Backends (cluster-scoped, no Project-ID required)
r.GET("/backends", h.ListBackends)
r.GET("/backends/:name", h.GetBackend)
r.POST("/backends", h.CreateBackend)
r.PATCH("/backends/:name", h.UpdateBackend)
r.DELETE("/backends/:name", h.DeleteBackend)
```

---

### Task 8: Wire Up in main.go (cmd/server/main.go)

Add service instantiation:

```go
// Add after aiBackendSvc
backendSvc := services.NewBackendService(aiGatewayClient)

// Update handler construction
h := handlers.New(
    modelSvc, versionSvc, artifactSvc, servingEnvSvc, isvcSvc,
    serveModelSvc, deploySvc, trafficSvc, virtualModelSvc,
    metricsSvc, aiBackendSvc,
    backendSvc,  // NEW
)
```

---

## API Specification

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/backends` | List all backends |
| GET | `/backends/:name` | Get backend by name |
| POST | `/backends` | Create new backend |
| PATCH | `/backends/:name` | Update backend |
| DELETE | `/backends/:name` | Delete backend |

### Query Parameters

- `namespace` - K8s namespace (default: `model-serving`)

### Create Backend Request

```json
{
  "name": "kserve-llama",
  "namespace": "model-serving",
  "endpoints": [
    {
      "fqdn": {
        "hostname": "llama.svc.cluster.local",
        "port": 80
      }
    }
  ],
  "labels": {
    "team": "ml-platform"
  }
}
```

### Create Backend with IP

```json
{
  "name": "external-openai",
  "endpoints": [
    {
      "ip": {
        "address": "10.0.0.1",
        "port": 443
      }
    }
  ]
}
```

### Backend Response

```json
{
  "name": "kserve-llama",
  "namespace": "model-serving",
  "endpoints": [
    {
      "fqdn": {
        "hostname": "llama.svc.cluster.local",
        "port": 80
      }
    }
  ],
  "labels": {
    "team": "ml-platform",
    "managed-by": "model-registry"
  }
}
```

---

## Full Flow Example

```bash
# 1. Create Backend (NEW API)
curl -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "kserve-llama",
    "endpoints": [{"fqdn": {"hostname": "llama-predictor.model-serving.svc.cluster.local", "port": 80}}]
  }'

# 2. Create AIServiceBackend (refs Backend)
curl -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "llama-70b",
    "schema": "OpenAI",
    "backend_ref": {"name": "kserve-llama"}
  }'

# 3. Create VirtualModel + link to AIServiceBackend
curl -X POST "$BASE_URL/virtual_models" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"name": "chat-model"}'

curl -X POST "$BASE_URL/virtual_models/chat-model/backends" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{
    "ai_service_backend_name": "llama-70b",
    "weight": 100
  }'
```

---

## File Changes Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/core/ports/output/ai_gateway_client.go` | MODIFY | Add Backend types + interface methods |
| `internal/core/domain/errors.go` | MODIFY | ✅ Added EnvoyBackend errors |
| `internal/adapters/primary/http/handlers/error-mapper.go` | MODIFY | ✅ Added error mappings |
| `internal/adapters/secondary/aigateway/client.go` | MODIFY | Implement Backend CRUD methods |
| `internal/core/services/backend.go` | CREATE | New BackendService |
| `internal/adapters/primary/http/dto/backend.go` | CREATE | New DTOs |
| `internal/adapters/primary/http/handlers/backend.go` | CREATE | New handlers |
| `internal/adapters/primary/http/handlers/handler.go` | MODIFY | Add backendSvc + routes |
| `cmd/server/main.go` | MODIFY | Wire up BackendService |
| `docs/e2e-tests/backend.md` | CREATE | E2E test documentation |

---

## Testing Strategy

### Unit Tests

**File: `internal/core/services/backend_test.go`**
- Service layer tests with mock AIGatewayClient
- Test Create/Get/List/Update/Delete operations
- Test error handling (not found, already exists, gateway unavailable)
- Follow pattern from `ai_service_backend_test.go`

**File: `internal/adapters/primary/http/dto/backend_test.go`**
- Test `ToBackend()` converter (request → domain)
- Test `ToBackendResponse()` converter (domain → response)
- Test `EndpointRequestToPorts()` for FQDN and IP endpoints
- Test edge cases (nil endpoints, empty labels)

### Integration Tests
- Create/Get/List/Update/Delete Backend via API
- Verify K8s resources created correctly
- Error handling (not found, already exists, validation)

### E2E Test Script

```bash
# Test script: docs/e2e-tests/backend.md

# 1. List (empty)
curl -s "$BASE_URL/backends"
# Expected: {"items":[],"total":0}

# 2. Create with FQDN
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-backend","endpoints":[{"fqdn":{"hostname":"test.svc.local","port":80}}]}'

# 3. Get
curl -s "$BASE_URL/backends/test-backend"

# 4. Update endpoints
curl -s -X PATCH "$BASE_URL/backends/test-backend" \
  -H "Content-Type: application/json" \
  -d '{"endpoints":[{"fqdn":{"hostname":"new.svc.local","port":8080}}]}'

# 5. Verify in K8s
kubectl get backends -n model-serving

# 6. Delete
curl -s -X DELETE "$BASE_URL/backends/test-backend"

# 7. Verify deleted
kubectl get backends -n model-serving
```

---

## Dependencies

- Existing AIGatewayClient infrastructure
- Envoy Gateway Backend CRD installed in cluster
- No database changes required (K8s only)

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Backend CRD not installed | Return `ErrAIGatewayNotAvailable` gracefully |
| Name collision with VirtualModel "backend" | Use clear naming: EnvoyBackend vs VirtualModelBackend |
| TLS support needed later | Spec designed to extend easily |

---

## Future Extensions

- **TLS Support**: Add `tls` field to Backend spec
- **Health Checks**: Add healthCheck configuration
- **Circuit Breaker**: Add circuit breaker policies
- **Load Balancing**: Add LB algorithm configuration

---

## Acceptance Criteria

1. Can create Backend via `POST /backends`
2. Can list/get/update/delete Backends
3. Backend CR appears in K8s with correct spec
4. AIServiceBackend can reference Backend via `backend_ref`
5. Full flow works: Backend -> AIServiceBackend -> VirtualModel
6. Proper error handling (404, 409, validation)
7. All existing tests pass

---

## Implementation Order

1. Ports layer (types + interface)
2. Adapter layer (K8s client methods)
3. Service layer
4. DTO layer
5. Handler layer
6. Wire up in main.go
7. Unit tests
8. E2E test documentation

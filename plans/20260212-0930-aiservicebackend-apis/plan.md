# AIServiceBackend Management APIs

**Date**: 2026-02-12
**Status**: ✅ Implemented
**Author**: Claude

## Executive Summary

Add HTTP APIs to manage AIServiceBackend CRs through Model Registry, enabling full lifecycle management of LLM backends without kubectl.

### Problem

```
Current Flow (broken):
1. POST /virtual_models → Creates VM + AIGatewayRoute ✅
2. POST /virtual_models/:name/backends → Links VM to backend ✅
3. AIServiceBackend doesn't exist in K8s ❌ → Route points to nothing
```

### Solution

Add CRUD APIs for AIServiceBackend management + validation when linking to virtual models.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Model Registry Service                        │
├─────────────────────────────────────────────────────────────────────┤
│  NEW: /ai_service_backends                                           │
│       ├── POST   → Create backend in K8s                            │
│       ├── GET    → List backends from K8s                           │
│       ├── GET/:n → Get backend from K8s                             │
│       ├── PATCH  → Update backend in K8s                            │
│       └── DELETE → Delete backend from K8s                          │
├─────────────────────────────────────────────────────────────────────┤
│  EXISTING: /virtual_models                                           │
│       └── POST /:name/backends → NOW validates backend exists       │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                              │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │
│  │ AIServiceBackend │  │ AIGatewayRoute   │  │ Backend (EG)     │  │
│  │ (schema, ref)    │──│ (routing rules)  │  │ (actual endpoint)│  │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

**Note**: AIServiceBackend APIs are cluster-scoped (not project-scoped) since they manage Kubernetes resources directly.

---

## API Design

### New Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/model-registry/ai_service_backends` | Create backend |
| GET | `/api/v1/model-registry/ai_service_backends` | List backends |
| GET | `/api/v1/model-registry/ai_service_backends/:name` | Get backend |
| PATCH | `/api/v1/model-registry/ai_service_backends/:name` | Update backend |
| DELETE | `/api/v1/model-registry/ai_service_backends/:name` | Delete backend |

### Request/Response DTOs

```go
// CreateAIServiceBackendRequest
{
  "name": "openai-gpt4",           // Required: CR name
  "namespace": "model-serving",     // Optional: defaults to config
  "schema": "OpenAI",              // Required: OpenAI|Anthropic|AWSBedrock|GoogleAI|AzureOpenAI
  "backend_ref": {                 // Required: Envoy Gateway Backend reference
    "name": "openai-backend",
    "namespace": "model-serving"
  },
  "header_mutation": {             // Optional: header transforms
    "set": [{"name": "x-api-key", "value": "..."}],
    "remove": ["x-internal-header"]
  },
  "labels": {"team": "ml-platform"} // Optional
}

// AIServiceBackendResponse
{
  "name": "openai-gpt4",
  "namespace": "model-serving",
  "schema": "OpenAI",
  "backend_ref": {
    "name": "openai-backend",
    "namespace": "model-serving",
    "group": "gateway.envoyproxy.io",
    "kind": "Backend"
  },
  "header_mutation": {...},
  "labels": {...},
  "status": {
    "ready": true,
    "conditions": ["Accepted"]
  },
  "created_at": "2026-02-12T09:30:00Z"
}
```

### Supported Schemas

| Schema | Description |
|--------|-------------|
| `OpenAI` | OpenAI API format |
| `Anthropic` | Anthropic Claude API |
| `AWSBedrock` | AWS Bedrock |
| `GoogleAI` | Google AI (Gemini) |
| `AzureOpenAI` | Azure OpenAI Service |

---

## Implementation Plan

### Phase 1: Extend AI Gateway Client

**File**: `internal/core/ports/output/ai_gateway_client.go`

Add HeaderMutation to AIServiceBackend struct (after line ~57):
```go
type AIServiceBackend struct {
    Name           string
    Namespace      string
    Schema         string
    BackendRef     BackendRef
    HeaderMutation *HeaderMutation  // NEW
    Labels         map[string]string
}

// NEW: Header mutation for API key injection
type HeaderMutation struct {
    Set    []HTTPHeader
    Remove []string
}

type HTTPHeader struct {
    Name  string
    Value string
}
```

Add List method to interface:

```go
type AIGatewayClient interface {
    // ... existing methods ...

    // NEW: List all AIServiceBackends in namespace
    ListServiceBackends(ctx context.Context, namespace string) ([]*AIServiceBackend, error)
}
```

**File**: `internal/adapters/secondary/aigateway/client.go`

Implement ListServiceBackends:

```go
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
```

### Phase 2: Add Domain Errors

**File**: `internal/core/domain/errors.go`

Add new errors (some already exist, verify before adding):

```go
// AI Gateway Errors
var (
    ErrAIGatewayNotAvailable = errors.New("AI Gateway integration not available")
    ErrInvalidSchema         = errors.New("invalid API schema")
)

// Note: ErrBackendNotFound and ErrBackendAlreadyExists already exist
```

### Phase 3: Create Service Layer

**File**: `internal/core/services/ai_service_backend.go`

```go
package services

import (
    "context"
    "strings"

    "model-registry-service/internal/core/domain"
    output "model-registry-service/internal/core/ports/output"
)

type AIServiceBackendService struct {
    aiGateway output.AIGatewayClient
}

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
    return s.aiGateway.ListServiceBackends(ctx, namespace)
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
```

### Phase 4: Create DTOs

**File**: `internal/adapters/primary/http/dto/ai_service_backend.go`

```go
package dto

import (
    "time"

    ports "model-registry-service/internal/core/ports/output"
)

// ============================================================================
// Request DTOs
// ============================================================================

type CreateAIServiceBackendRequest struct {
    Name           string            `json:"name" binding:"required"`
    Namespace      string            `json:"namespace"`
    Schema         string            `json:"schema" binding:"required,oneof=OpenAI Anthropic AWSBedrock GoogleAI AzureOpenAI"`
    BackendRef     BackendRefRequest `json:"backend_ref" binding:"required"`
    HeaderMutation *HeaderMutation   `json:"header_mutation"`
    Labels         map[string]string `json:"labels"`
}

type BackendRefRequest struct {
    Name      string `json:"name" binding:"required"`
    Namespace string `json:"namespace"`
}

type HeaderMutation struct {
    Set    []HTTPHeader `json:"set"`
    Remove []string     `json:"remove"`
}

type HTTPHeader struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type UpdateAIServiceBackendRequest struct {
    Schema         *string           `json:"schema"`
    HeaderMutation *HeaderMutation   `json:"header_mutation"`
    Labels         map[string]string `json:"labels"`
}

// ============================================================================
// Response DTOs
// ============================================================================

type AIServiceBackendResponse struct {
    Name           string             `json:"name"`
    Namespace      string             `json:"namespace"`
    Schema         string             `json:"schema"`
    BackendRef     BackendRefResponse `json:"backend_ref"`
    HeaderMutation *HeaderMutation    `json:"header_mutation,omitempty"`
    Labels         map[string]string  `json:"labels,omitempty"`
    Status         *BackendStatus     `json:"status,omitempty"`
    CreatedAt      *time.Time         `json:"created_at,omitempty"`
}

type BackendRefResponse struct {
    Name      string `json:"name"`
    Namespace string `json:"namespace,omitempty"`
    Group     string `json:"group"`
    Kind      string `json:"kind"`
}

type BackendStatus struct {
    Ready      bool     `json:"ready"`
    Conditions []string `json:"conditions,omitempty"`
}

type ListAIServiceBackendsResponse struct {
    Items []AIServiceBackendResponse `json:"items"`
    Total int                        `json:"total"`
}

// ============================================================================
// Converters
// ============================================================================

func ToAIServiceBackend(req *CreateAIServiceBackendRequest) *ports.AIServiceBackend {
    backend := &ports.AIServiceBackend{
        Name:      req.Name,
        Namespace: req.Namespace,
        Schema:    req.Schema,
        BackendRef: ports.BackendRef{
            Name:      req.BackendRef.Name,
            Namespace: req.BackendRef.Namespace,
            Group:     "gateway.envoyproxy.io",
            Kind:      "Backend",
        },
        Labels: req.Labels,
    }
    // Convert HeaderMutation if provided
    if req.HeaderMutation != nil {
        backend.HeaderMutation = &ports.HeaderMutation{
            Remove: req.HeaderMutation.Remove,
        }
        for _, h := range req.HeaderMutation.Set {
            backend.HeaderMutation.Set = append(backend.HeaderMutation.Set, ports.HTTPHeader{
                Name:  h.Name,
                Value: h.Value,
            })
        }
    }
    return backend
}

func ToAIServiceBackendResponse(backend *ports.AIServiceBackend) AIServiceBackendResponse {
    resp := AIServiceBackendResponse{
        Name:      backend.Name,
        Namespace: backend.Namespace,
        Schema:    backend.Schema,
        BackendRef: BackendRefResponse{
            Name:      backend.BackendRef.Name,
            Namespace: backend.BackendRef.Namespace,
            Group:     backend.BackendRef.Group,
            Kind:      backend.BackendRef.Kind,
        },
        Labels: backend.Labels,
    }
    // Convert HeaderMutation if present
    if backend.HeaderMutation != nil {
        resp.HeaderMutation = &HeaderMutation{
            Remove: backend.HeaderMutation.Remove,
        }
        for _, h := range backend.HeaderMutation.Set {
            resp.HeaderMutation.Set = append(resp.HeaderMutation.Set, HTTPHeader{
                Name:  h.Name,
                Value: h.Value,
            })
        }
    }
    return resp
}
```

### Phase 5: Create HTTP Handlers

**File**: `internal/adapters/primary/http/handlers/ai_service_backend.go`

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    log "github.com/sirupsen/logrus"

    "model-registry-service/internal/adapters/primary/http/dto"
)

func (h *Handler) ListAIServiceBackends(c *gin.Context) {
    namespace := c.DefaultQuery("namespace", "model-serving")

    backends, err := h.aiBackendSvc.List(c.Request.Context(), namespace)
    if err != nil {
        log.WithError(err).Error("list ai service backends failed")
        mapDomainError(c, err)
        return
    }

    items := make([]dto.AIServiceBackendResponse, 0, len(backends))
    for _, b := range backends {
        items = append(items, dto.ToAIServiceBackendResponse(b))
    }

    c.JSON(http.StatusOK, dto.ListAIServiceBackendsResponse{
        Items: items,
        Total: len(items),
    })
}

func (h *Handler) GetAIServiceBackend(c *gin.Context) {
    name := c.Param("name")
    namespace := c.DefaultQuery("namespace", "model-serving")

    backend, err := h.aiBackendSvc.Get(c.Request.Context(), namespace, name)
    if err != nil {
        log.WithError(err).Error("get ai service backend failed")
        mapDomainError(c, err)
        return
    }

    c.JSON(http.StatusOK, dto.ToAIServiceBackendResponse(backend))
}

func (h *Handler) CreateAIServiceBackend(c *gin.Context) {
    var req dto.CreateAIServiceBackendRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    backend := dto.ToAIServiceBackend(&req)
    if backend.Namespace == "" {
        backend.Namespace = "model-serving"
    }

    if err := h.aiBackendSvc.Create(c.Request.Context(), backend); err != nil {
        log.WithError(err).Error("create ai service backend failed")
        mapDomainError(c, err)
        return
    }

    // Fetch created backend to get full response
    created, _ := h.aiBackendSvc.Get(c.Request.Context(), backend.Namespace, backend.Name)
    if created != nil {
        c.JSON(http.StatusCreated, dto.ToAIServiceBackendResponse(created))
    } else {
        c.JSON(http.StatusCreated, dto.ToAIServiceBackendResponse(backend))
    }
}

func (h *Handler) UpdateAIServiceBackend(c *gin.Context) {
    name := c.Param("name")
    namespace := c.DefaultQuery("namespace", "model-serving")

    var req dto.UpdateAIServiceBackendRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Get existing
    existing, err := h.aiBackendSvc.Get(c.Request.Context(), namespace, name)
    if err != nil {
        mapDomainError(c, err)
        return
    }

    // Apply updates
    if req.Schema != nil {
        existing.Schema = *req.Schema
    }
    if req.Labels != nil {
        existing.Labels = req.Labels
    }

    if err := h.aiBackendSvc.Update(c.Request.Context(), existing); err != nil {
        log.WithError(err).Error("update ai service backend failed")
        mapDomainError(c, err)
        return
    }

    c.JSON(http.StatusOK, dto.ToAIServiceBackendResponse(existing))
}

func (h *Handler) DeleteAIServiceBackend(c *gin.Context) {
    name := c.Param("name")
    namespace := c.DefaultQuery("namespace", "model-serving")

    if err := h.aiBackendSvc.Delete(c.Request.Context(), namespace, name); err != nil {
        log.WithError(err).Error("delete ai service backend failed")
        mapDomainError(c, err)
        return
    }

    c.Status(http.StatusNoContent)
}
```

### Phase 6: Update Handler Struct and Routes

**File**: `internal/adapters/primary/http/handlers/handler.go`

Update Handler struct (add field after line 19):
```go
type Handler struct {
    modelSvc        *services.RegisteredModelService
    versionSvc      *services.ModelVersionService
    artifactSvc     *services.ModelArtifactService
    servingEnvSvc   *services.ServingEnvironmentService
    isvcSvc         *services.InferenceServiceService
    serveModelSvc   *services.ServeModelService
    deploySvc       *services.DeployService
    trafficSvc      *services.TrafficService
    virtualModelSvc *services.VirtualModelService
    metricsSvc      *services.MetricsService
    aiBackendSvc    *services.AIServiceBackendService  // NEW
}
```

Update New() function (add parameter after line 32):
```go
func New(
    modelSvc *services.RegisteredModelService,
    versionSvc *services.ModelVersionService,
    artifactSvc *services.ModelArtifactService,
    servingEnvSvc *services.ServingEnvironmentService,
    isvcSvc *services.InferenceServiceService,
    serveModelSvc *services.ServeModelService,
    deploySvc *services.DeployService,
    trafficSvc *services.TrafficService,
    virtualModelSvc *services.VirtualModelService,
    metricsSvc *services.MetricsService,
    aiBackendSvc *services.AIServiceBackendService,  // NEW
) *Handler {
    return &Handler{
        modelSvc:        modelSvc,
        versionSvc:      versionSvc,
        artifactSvc:     artifactSvc,
        servingEnvSvc:   servingEnvSvc,
        isvcSvc:         isvcSvc,
        serveModelSvc:   serveModelSvc,
        deploySvc:       deploySvc,
        trafficSvc:      trafficSvc,
        virtualModelSvc: virtualModelSvc,
        metricsSvc:      metricsSvc,
        aiBackendSvc:    aiBackendSvc,  // NEW
    }
}
```

Add routes in RegisterRoutes() (after Virtual Models section):
```go
// AI Service Backends (cluster-scoped, no Project-ID required)
r.GET("/ai_service_backends", h.ListAIServiceBackends)
r.GET("/ai_service_backends/:name", h.GetAIServiceBackend)
r.POST("/ai_service_backends", h.CreateAIServiceBackend)
r.PATCH("/ai_service_backends/:name", h.UpdateAIServiceBackend)
r.DELETE("/ai_service_backends/:name", h.DeleteAIServiceBackend)
```

### Phase 7: Update Error Mapper

**File**: `internal/adapters/primary/http/handlers/error-mapper.go`

Add new error mappings:

```go
func mapDomainError(c *gin.Context, err error) {
    switch {
    // Not found errors (add ErrAIGatewayNotAvailable if needed elsewhere)
    case errors.Is(err, domain.ErrModelNotFound),
        // ... existing cases ...
        errors.Is(err, domain.ErrBackendNotFound):
        c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

    // Service unavailable errors (NEW)
    case errors.Is(err, domain.ErrAIGatewayNotAvailable):
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})

    // Bad request / validation errors
    case errors.Is(err, domain.ErrInvalidModelName),
        // ... existing cases ...
        errors.Is(err, domain.ErrInvalidSchema):  // NEW
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

    // ... rest of function
    }
}
```

### Phase 8: Add Backend Validation to Virtual Model

**File**: `internal/core/services/virtual_model.go`

Modify `AddBackend` to validate backend exists (use correct field names):

```go
func (s *VirtualModelService) AddBackend(ctx context.Context, req AddBackendRequest) (*domain.VirtualModelBackend, error) {
    // ... existing validation ...

    // NEW: Validate AIServiceBackend exists in K8s
    if s.aiGateway != nil && s.aiGateway.IsAvailable() {
        namespace := req.AIServiceBackendNS  // CORRECT field name
        if namespace == "" {
            namespace = "model-serving"
        }
        _, err := s.aiGateway.GetServiceBackend(ctx, namespace, req.AIServiceBackendName)  // CORRECT field name
        if err != nil {
            return nil, fmt.Errorf("backend %s/%s not found: %w", namespace, req.AIServiceBackendName, domain.ErrBackendNotFound)
        }
    }

    // ... continue with existing logic ...
}
```

### Phase 9: Update main.go Wiring

**File**: `cmd/server/main.go`

```go
// Create AI Service Backend service (after line ~115)
aiBackendSvc := services.NewAIServiceBackendService(aiGatewayClient)

// Update Handler constructor (line ~119)
h := handlers.New(
    modelSvc, versionSvc, artifactSvc,
    servingEnvSvc, isvcSvc, serveModelSvc,
    deploySvc, trafficSvc, virtualModelSvc, metricsSvc,
    aiBackendSvc,  // NEW - 11th parameter
)
```

---

## Files to Create/Modify

| Action | File |
|--------|------|
| Modify | `internal/core/ports/output/ai_gateway_client.go` |
| Modify | `internal/adapters/secondary/aigateway/client.go` |
| Modify | `internal/core/domain/errors.go` |
| Create | `internal/core/services/ai_service_backend.go` |
| Create | `internal/adapters/primary/http/dto/ai_service_backend.go` |
| Create | `internal/adapters/primary/http/handlers/ai_service_backend.go` |
| Modify | `internal/adapters/primary/http/handlers/handler.go` |
| Modify | `internal/adapters/primary/http/handlers/error-mapper.go` |
| Modify | `internal/core/services/virtual_model.go` |
| Modify | `cmd/server/main.go` |

---

## Estimated Effort

| Phase | Effort |
|-------|--------|
| Phase 1: Extend AI Gateway Client | 0.5h |
| Phase 2: Add Domain Errors | 0.25h |
| Phase 3: Service Layer | 0.5h |
| Phase 4: DTOs | 0.5h |
| Phase 5: HTTP Handlers | 1h |
| Phase 6: Update Handler Struct | 0.5h |
| Phase 7: Update Error Mapper | 0.25h |
| Phase 8: Backend Validation | 0.5h |
| Phase 9: Update main.go | 0.25h |
| Testing | 1h |
| **Total** | **~5.25h** |

---

## Testing

### Unit Tests

1. Service layer tests with mock AI Gateway client
2. Handler tests with httptest
3. DTO conversion tests

### Integration Tests

```bash
# 1. Create Backend
curl -X POST http://localhost:8080/api/v1/model-registry/ai_service_backends \
  -H "Content-Type: application/json" \
  -d '{
    "name": "openai-gpt4",
    "schema": "OpenAI",
    "backend_ref": {
      "name": "openai-backend"
    }
  }'

# 2. List Backends
curl http://localhost:8080/api/v1/model-registry/ai_service_backends

# 3. Create Virtual Model
curl -X POST http://localhost:8080/api/v1/model-registry/virtual_models \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"name": "gpt-4"}'

# 4. Add Backend (should validate existence)
curl -X POST http://localhost:8080/api/v1/model-registry/virtual_models/gpt-4/backends \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{
    "ai_service_backend_name": "openai-gpt4",
    "weight": 100
  }'
```

---

## Future Enhancements

1. **Envoy Gateway Backend CRUD** - Create the underlying Backend resources
2. **Backend Health Checks** - Query backend readiness
3. **Batch Operations** - Bulk create/update backends
4. **Import from K8s** - Discover existing backends

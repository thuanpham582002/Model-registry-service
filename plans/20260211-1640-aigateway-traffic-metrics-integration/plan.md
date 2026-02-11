# AI Gateway + Traffic Management + Metrics Integration

**Date**: 2026-02-11
**Status**: ✅ Completed
**Author**: Claude
**Last Updated**: 2026-02-11

## Executive Summary

Extend Model Registry Service with AI Gateway integration for advanced traffic management (canary deployments, A/B testing) and metrics visualization through existing Prometheus/Grafana stack. Billing will be calculated from metrics data later.

## Requirements

| # | Feature | Priority | Complexity |
|---|---------|----------|------------|
| 1 | Traffic Splitting (Canary) | High | Medium |
| 2 | Metrics Visualization | High | Medium |
| 3 | AI Gateway Integration | High | High |

## Constraints & Decisions

| Question | Answer |
|----------|--------|
| Token counting | AI Gateway built-in (for metrics) |
| Billing | Calculated from Prometheus metrics later |
| Grafana | Already deployed |
| Metrics storage | Prometheus → Grafana |
| Istio | Already deployed |
| AI Gateway | Already deployed |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CONTROL PLANE                                   │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                    Model Registry Service (Extended)                    │ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌───────────────┐  │ │
│  │  │ Model CRUD   │ │ Deploy Svc   │ │ Traffic Svc  │ │ Metrics Svc   │  │ │
│  │  │ Version CRUD │ │ KServe ISVC  │ │ Canary mgmt  │ │ Prometheus    │  │ │
│  │  │ Env CRUD     │ │ AIGatewayRt  │ │ A/B testing  │ │ Dashboards    │  │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └───────────────┘  │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                    │                   │                   │                 │
│                    ▼                   ▼                   ▼                 │
│  ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐   │
│  │   KServe CRDs       │ │  AI Gateway CRDs    │ │   PostgreSQL        │   │
│  │ • InferenceService  │ │ • AIGatewayRoute    │ │ • traffic_config    │   │
│  │ • InferenceGraph    │ │ • BackendTrafficPol │ │ • traffic_variant   │   │
│  └─────────────────────┘ └─────────────────────┘ └─────────────────────┘   │
│                                                   └─────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                               DATA PLANE                                     │
│                                                                              │
│   Client ──► JWT + Project-ID ──► Envoy AI Gateway ──┬──► ISVC v1 (stable)  │
│              │                    • Route by model    ├──► ISVC v2 (canary)  │
│              │                    • Traffic weights   └──► ISVC v3 (shadow)  │
│              │                    • Token counting                           │
│              │                    • Rate limiting                            │
│              └──► Project-ID header for metrics tracking                    │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                            OBSERVABILITY                                     │
│                                                                              │
│  Prometheus ◄── AI Gateway metrics ──► Grafana Dashboards                   │
│             ◄── KServe metrics                                              │
│             ◄── Istio metrics                                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Implementation Phases

| Phase | Name | Duration | Dependencies | Status |
|-------|------|----------|--------------|--------|
| 1 | Database & Domain | 1 week | None | ✅ Done |
| 2 | AI Gateway Adapter | 1 week | Phase 1 | ✅ Done |
| 3 | Traffic Service | 1.5 weeks | Phase 1, 2 | ✅ Done |
| 4 | Metrics Integration | 1 week | Phase 1-3 | ✅ Done |
| 5 | **Serve Model Refactor** | 0.5 day | Phase 1-4 | ✅ Done |

**Total**: ~4.5 weeks

### Phase 5: Serve Model Refactoring (Post-Implementation)

Refactored to support **multi-model serving** per InferenceService:

| Change | Before | After |
|--------|--------|-------|
| Model-Version link | `inference_service.model_version_id` | `serve_model` junction table |
| Cardinality | 1 ISVC : 1 Version | 1 ISVC : N Versions |
| Deploy API | `model_version_id: UUID` | `model_version_ids: []UUID` |
| Response | `model_version_id` field | `served_models[]` array |

**Migration**: `000005_refactor_serve_model.up.sql` migrates existing data automatically.

---

## New API Endpoints

### Traffic Management (Multi-Variant Support)
```
# Config CRUD
POST   /api/v1/model-registry/traffic_configs
GET    /api/v1/model-registry/traffic_configs
GET    /api/v1/model-registry/traffic_configs/:id
PATCH  /api/v1/model-registry/traffic_configs/:id
DELETE /api/v1/model-registry/traffic_configs/:id

# Variant CRUD (supports N variants: stable, canary, variant_a, variant_b, shadow, etc.)
POST   /api/v1/model-registry/traffic_configs/:id/variants
GET    /api/v1/model-registry/traffic_configs/:id/variants
GET    /api/v1/model-registry/traffic_configs/:id/variants/:name
PATCH  /api/v1/model-registry/traffic_configs/:id/variants/:name
DELETE /api/v1/model-registry/traffic_configs/:id/variants/:name

# Bulk weight update
PATCH  /api/v1/model-registry/traffic_configs/:id/weights

# Quick actions (convenience endpoints)
POST   /api/v1/model-registry/traffic_configs/:id/promote/:variant_name
POST   /api/v1/model-registry/traffic_configs/:id/rollback
```

### Virtual Model (Model Name Virtualization)
```
# Virtual model CRUD
POST   /api/v1/model-registry/virtual_models
GET    /api/v1/model-registry/virtual_models
GET    /api/v1/model-registry/virtual_models/:name
PATCH  /api/v1/model-registry/virtual_models/:name
DELETE /api/v1/model-registry/virtual_models/:name

# Backend mappings for a virtual model
POST   /api/v1/model-registry/virtual_models/:name/backends
GET    /api/v1/model-registry/virtual_models/:name/backends
PATCH  /api/v1/model-registry/virtual_models/:name/backends/:backend_id
DELETE /api/v1/model-registry/virtual_models/:name/backends/:backend_id
```

### Metrics
```
GET    /api/v1/model-registry/metrics/deployments/:isvc_id
GET    /api/v1/model-registry/metrics/compare
GET    /api/v1/model-registry/metrics/tokens
```

---

## Files to Create/Modify

### New Files

```
# Domain
internal/core/domain/traffic.go
internal/core/domain/virtual_model.go

# Ports
internal/core/ports/output/traffic_repository.go
internal/core/ports/output/virtual_model_repository.go
internal/core/ports/output/ai_gateway_client.go
internal/core/ports/output/prometheus_client.go

# Adapters - Secondary
internal/adapters/secondary/postgres/traffic_config_repo.go
internal/adapters/secondary/postgres/traffic_variant_repo.go
internal/adapters/secondary/postgres/virtual_model_repo.go
internal/adapters/secondary/aigateway/client.go
internal/adapters/secondary/prometheus/client.go

# Services
internal/core/services/traffic.go
internal/core/services/virtual_model.go
internal/core/services/metrics.go

# DTOs & Handlers
internal/adapters/primary/http/dto/traffic.go
internal/adapters/primary/http/dto/virtual_model.go
internal/adapters/primary/http/dto/metrics.go
internal/adapters/primary/http/handlers/traffic.go
internal/adapters/primary/http/handlers/virtual_model.go
internal/adapters/primary/http/handlers/metrics.go

# Migrations
migrations/000003_add_traffic_tables.up.sql
migrations/000003_add_traffic_tables.down.sql
migrations/000004_add_virtual_model_tables.up.sql
migrations/000004_add_virtual_model_tables.down.sql
migrations/000005_refactor_serve_model.up.sql      # Phase 5: Multi-model serving
migrations/000005_refactor_serve_model.down.sql

# Config
internal/config/config.go (update)

# K8s Manifests
deploy/k8s/aigateway-rbac.yaml
deploy/grafana/dashboards/
```

### Modified Files

```
cmd/server/main.go
internal/adapters/primary/http/handlers/handler.go
internal/adapters/primary/http/handlers/error-mapper.go
internal/core/domain/errors.go
```

---

## Phase Details

- [Phase 1: Database & Domain](./phase-01-database-domain.md)
- [Phase 2: AI Gateway Adapter](./phase-02-aigateway-adapter.md)
- [Phase 3: Traffic Service](./phase-03-traffic-service.md)
- [Phase 4: Metrics Integration](./phase-04-metrics-integration.md)

---

## Usage Examples

### 1. Canary Deployment (2 variants)
```bash
# Create config with stable variant
POST /traffic_configs
{ "inference_service_id": "...", "strategy": "canary", "stable_version_id": "v1" }

# Add canary variant with 10% traffic
POST /traffic_configs/:id/variants
{ "variant_name": "canary", "model_version_id": "v2", "weight": 10 }

# Increase canary to 50%
PATCH /traffic_configs/:id/weights
{ "weights": { "stable": 50, "canary": 50 } }

# Promote canary to stable
POST /traffic_configs/:id/promote/canary
```

### 2. A/B Testing (N variants)
```bash
# Create config
POST /traffic_configs
{ "inference_service_id": "...", "strategy": "ab_test", "stable_version_id": "v1" }

# Add multiple variants
POST /traffic_configs/:id/variants
{ "variant_name": "variant_b", "model_version_id": "v2", "weight": 0 }

POST /traffic_configs/:id/variants
{ "variant_name": "variant_c", "model_version_id": "v3", "weight": 0 }

# Set traffic split
PATCH /traffic_configs/:id/weights
{ "weights": { "stable": 34, "variant_b": 33, "variant_c": 33 } }

# Compare performance
GET /metrics/compare?variant=stable&variant=variant_b&variant=variant_c
```

### 3. Blue/Green Deployment
```bash
# Setup: blue=100%, green=0%
PATCH /traffic_configs/:id/weights
{ "weights": { "blue": 100, "green": 0 } }

# Switch: blue=0%, green=100%
PATCH /traffic_configs/:id/weights
{ "weights": { "blue": 0, "green": 100 } }

# Rollback if needed
POST /traffic_configs/:id/rollback
```

### 4. Virtual Model (Multi-Provider)
```bash
# Create virtual model "claude-4-sonnet" that maps to different providers
POST /virtual_models
{
  "name": "claude-4-sonnet",
  "description": "Claude 4 Sonnet across providers"
}

# Add AWS Bedrock backend
POST /virtual_models/claude-4-sonnet/backends
{
  "ai_service_backend_id": "aws-anthropic-backend",
  "model_name_override": "anthropic.claude-sonnet-4-20250514-v1:0",
  "weight": 50,
  "priority": 0
}

# Add GCP backend
POST /virtual_models/claude-4-sonnet/backends
{
  "ai_service_backend_id": "gcp-anthropic-backend",
  "model_name_override": "claude-sonnet-4@20250514",
  "weight": 50,
  "priority": 0
}

# Client requests with virtual name
curl -H "x-ai-eg-model: claude-4-sonnet" ...
# AI Gateway routes to AWS or GCP based on weights
```

### 5. Virtual Model (Fallback)
```bash
# Create virtual model with fallback to cheaper model
POST /virtual_models
{ "name": "gpt-5-nano" }

# Primary: expensive model
POST /virtual_models/gpt-5-nano/backends
{
  "ai_service_backend_id": "openai-backend",
  "model_name_override": "gpt-5-nano",  # or null to use virtual name
  "weight": 100,
  "priority": 0  # Primary
}

# Fallback: cheaper model (same backend, different model)
POST /virtual_models/gpt-5-nano/backends
{
  "ai_service_backend_id": "openai-backend",
  "model_name_override": "gpt-5-nano-mini",
  "weight": 0,
  "priority": 1  # Fallback when primary fails
}
```

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| AI Gateway API changes | High | Pin CRD versions, integration tests |
| Token counting accuracy | Medium | Validate against AI Gateway logs |
| Prometheus query performance | Medium | Add indexes, cache results |
| Project-ID header missing | Low | Return 400 with clear error message |
| Weight sum > 100% | Medium | Validate at service layer |

---

## Success Criteria

- [x] Canary deployment with configurable traffic split
- [x] A/B testing with N variants (unlimited)
- [x] Blue/Green instant switch
- [x] Promote any variant to stable
- [x] Bulk weight update in single request
- [x] Rollback in <30 seconds
- [x] **Virtual model name mapping** (one-to-many)
- [x] **Multi-provider routing** with modelNameOverride
- [x] **Fallback to different model** on same provider
- [x] Grafana dashboards showing traffic/latency/tokens
- [x] Metrics API returns deployment performance data
- [x] E2E tests created for serving/traffic/metrics
- [x] **Multi-model serving per InferenceService** (Phase 5)

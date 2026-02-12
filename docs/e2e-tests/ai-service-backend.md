# E2E Test: AIServiceBackend APIs

End-to-end test documentation for AIServiceBackend CRUD operations in model-registry-service.

## Prerequisites

1. **Kubernetes cluster** with AI Gateway CRDs installed
2. **model-registry-service** running and accessible
3. **kubectl** configured with cluster access
4. **curl** for API testing

## Environment Setup

```bash
# Set base URL
BASE_URL="http://localhost:8080/api/v1/model-registry"

# Set default namespace (optional - defaults to "model-serving" if not specified)
NAMESPACE="model-serving"

# Verify service is healthy
curl -s http://localhost:8080/healthz
```

## Full CRUD Test Script

Copy-paste and run these commands sequentially to test all AIServiceBackend operations.

### 1. LIST (Empty State)

```bash
curl -s "$BASE_URL/ai_service_backends"
```

**Expected Result:**
```json
{
  "items": [],
  "total": 0
}
```

### 2. CREATE Backend with Labels

```bash
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "openai-gpt4",
    "namespace": "model-serving",
    "schema": "OpenAI",
    "backend_ref": {
      "name": "openai-backend",
      "namespace": "model-serving"
    },
    "labels": {
      "team": "ml-platform"
    }
  }'
```

**Expected Result:**
```json
{
  "name": "openai-gpt4",
  "namespace": "model-serving",
  "schema": "OpenAI",
  "backend_ref": {
    "name": "openai-backend",
    "namespace": "model-serving"
  },
  "labels": {
    "team": "ml-platform"
  },
  "created_at": "2026-02-12T08:00:00Z",
  "updated_at": "2026-02-12T08:00:00Z"
}
```

### 3. Verify in Kubernetes

```bash
# List AIServiceBackend resources
kubectl get aiservicebackends -n model-serving

# Get detailed view
kubectl get aiservicebackend openai-gpt4 -n model-serving -o yaml
```

**Expected Output:**
```
NAME          SCHEMA    AGE
openai-gpt4   OpenAI    1m
```

### 4. GET Single Backend

```bash
curl -s "$BASE_URL/ai_service_backends/openai-gpt4"
```

**Expected Result:**
```json
{
  "name": "openai-gpt4",
  "namespace": "model-serving",
  "schema": "OpenAI",
  "backend_ref": {
    "name": "openai-backend",
    "namespace": "model-serving"
  },
  "labels": {
    "team": "ml-platform"
  },
  "created_at": "2026-02-12T08:00:00Z",
  "updated_at": "2026-02-12T08:00:00Z"
}
```

### 5. UPDATE Schema and Labels

```bash
curl -s -X PATCH "$BASE_URL/ai_service_backends/openai-gpt4" \
  -H "Content-Type: application/json" \
  -d '{
    "schema": "Anthropic",
    "labels": {
      "team": "ai-team",
      "env": "prod"
    }
  }'
```

**Expected Result:**
```json
{
  "name": "openai-gpt4",
  "namespace": "model-serving",
  "schema": "Anthropic",
  "backend_ref": {
    "name": "openai-backend",
    "namespace": "model-serving"
  },
  "labels": {
    "team": "ai-team",
    "env": "prod"
  },
  "created_at": "2026-02-12T08:00:00Z",
  "updated_at": "2026-02-12T08:01:00Z"
}
```

**Verify Update in K8s:**
```bash
kubectl get aiservicebackend openai-gpt4 -n model-serving -o jsonpath='{.spec.schema}{"\n"}'
# Expected: Anthropic

kubectl get aiservicebackend openai-gpt4 -n model-serving -o jsonpath='{.metadata.labels}{"\n"}'
# Expected: {"env":"prod","team":"ai-team"}
```

### 6. LIST After Update

```bash
curl -s "$BASE_URL/ai_service_backends"
```

**Expected Result:**
```json
{
  "items": [
    {
      "name": "openai-gpt4",
      "namespace": "model-serving",
      "schema": "Anthropic",
      "backend_ref": {
        "name": "openai-backend",
        "namespace": "model-serving"
      },
      "labels": {
        "team": "ai-team",
        "env": "prod"
      },
      "created_at": "2026-02-12T08:00:00Z",
      "updated_at": "2026-02-12T08:01:00Z"
    }
  ],
  "total": 1
}
```

### 7. DELETE Backend

```bash
curl -s -X DELETE "$BASE_URL/ai_service_backends/openai-gpt4"
```

**Expected Result:**
```json
{
  "message": "AIServiceBackend openai-gpt4 deleted successfully"
}
```

**Verify Deletion in K8s:**
```bash
kubectl get aiservicebackends -n model-serving
# Expected: No resources found (or openai-gpt4 not in list)
```

### 8. CREATE with HeaderMutation

```bash
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "anthropic-claude",
    "schema": "Anthropic",
    "backend_ref": {
      "name": "anthropic-backend"
    },
    "header_mutation": {
      "set": [
        {
          "name": "x-api-key",
          "value": "sk-ant-xxx"
        }
      ],
      "remove": ["x-internal-header"]
    }
  }'
```

**Expected Result:**
```json
{
  "name": "anthropic-claude",
  "namespace": "model-serving",
  "schema": "Anthropic",
  "backend_ref": {
    "name": "anthropic-backend",
    "namespace": "model-serving"
  },
  "header_mutation": {
    "set": [
      {
        "name": "x-api-key",
        "value": "sk-ant-xxx"
      }
    ],
    "remove": ["x-internal-header"]
  },
  "created_at": "2026-02-12T08:02:00Z",
  "updated_at": "2026-02-12T08:02:00Z"
}
```

### 9. Verify HeaderMutation in K8s

```bash
# Get headerMutation field
kubectl get aiservicebackend anthropic-claude -n model-serving \
  -o jsonpath='{.spec.headerMutation}' | jq .

# Get full spec
kubectl get aiservicebackend anthropic-claude -n model-serving -o yaml
```

**Expected Output:**
```json
{
  "set": [
    {
      "name": "x-api-key",
      "value": "sk-ant-xxx"
    }
  ],
  "remove": ["x-internal-header"]
}
```

## Cleanup

```bash
# Delete all test backends
curl -s -X DELETE "$BASE_URL/ai_service_backends/anthropic-claude"

# Verify cleanup in K8s
kubectl get aiservicebackends -n model-serving
```

## Test Scenarios Summary

| Operation | Endpoint | Method | Key Features |
|-----------|----------|--------|--------------|
| LIST empty | `/ai_service_backends` | GET | Returns empty list |
| CREATE basic | `/ai_service_backends` | POST | With labels |
| GET single | `/ai_service_backends/:name` | GET | Retrieve by name |
| UPDATE | `/ai_service_backends/:name` | PATCH | Schema + labels |
| LIST populated | `/ai_service_backends` | GET | Returns updated items |
| DELETE | `/ai_service_backends/:name` | DELETE | K8s resource removed |
| CREATE advanced | `/ai_service_backends` | POST | HeaderMutation set/remove |
| K8s verification | `kubectl get` | - | Confirm CRD sync |

## Error Scenarios

### 404 - Not Found
```bash
curl -s "$BASE_URL/ai_service_backends/nonexistent"
# Expected: {"error": "AIServiceBackend not found"}
```

### 409 - Conflict (Duplicate Name)
```bash
# Create first backend
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name": "duplicate-test", "schema": "OpenAI", "backend_ref": {"name": "backend"}}'

# Try to create again
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name": "duplicate-test", "schema": "OpenAI", "backend_ref": {"name": "backend"}}'
# Expected: {"error": "AIServiceBackend duplicate-test already exists"}
```

### 400 - Invalid Schema
```bash
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name": "invalid", "schema": "InvalidSchema", "backend_ref": {"name": "backend"}}'
# Expected: {"error": "invalid schema: must be one of OpenAI, Anthropic, VertexAI, etc."}
```

## Multi-Model Traffic Split Test

Complex scenario with **4 backends** and **3 virtual models**.

### Setup: Create 4 AIServiceBackends

```bash
# Backend 1: OpenAI
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name":"openai-gpt4","schema":"OpenAI","backend_ref":{"name":"openai-svc"}}'

# Backend 2: Anthropic
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name":"anthropic-claude","schema":"Anthropic","backend_ref":{"name":"anthropic-svc"}}'

# Backend 3: Gemini (using OpenAI-compatible schema)
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name":"gemini-pro","schema":"OpenAI","backend_ref":{"name":"google-svc"}}'

# Backend 4: Azure OpenAI
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name":"azure-openai","schema":"AzureOpenAI","backend_ref":{"name":"azure-svc"}}'
```

### Create 3 Virtual Models

```bash
PROJECT_ID="11111111-1111-1111-1111-111111111111"

# Model 1: Chat (2 backends)
curl -s -X POST "$BASE_URL/virtual_models" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"name":"chat-model","description":"Chat completion model"}'

# Model 2: Code (2 backends)
curl -s -X POST "$BASE_URL/virtual_models" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"name":"code-model","description":"Code generation model"}'

# Model 3: Enterprise (4 backends)
curl -s -X POST "$BASE_URL/virtual_models" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"name":"enterprise-model","description":"Enterprise with 4 backends"}'
```

### Configure Traffic Split

```bash
# chat-model: 80% OpenAI, 20% Anthropic
curl -s -X POST "$BASE_URL/virtual_models/chat-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"openai-gpt4","ai_service_backend_namespace":"model-serving","weight":80,"priority":0}'

curl -s -X POST "$BASE_URL/virtual_models/chat-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"anthropic-claude","ai_service_backend_namespace":"model-serving","weight":20,"priority":1}'

# code-model: 70% Anthropic, 30% OpenAI
curl -s -X POST "$BASE_URL/virtual_models/code-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"anthropic-claude","ai_service_backend_namespace":"model-serving","weight":70,"priority":0}'

curl -s -X POST "$BASE_URL/virtual_models/code-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"openai-gpt4","ai_service_backend_namespace":"model-serving","weight":30,"priority":1}'

# enterprise-model: 40% OpenAI, 30% Anthropic, 20% Gemini, 10% Azure
curl -s -X POST "$BASE_URL/virtual_models/enterprise-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"openai-gpt4","ai_service_backend_namespace":"model-serving","weight":40,"priority":0}'

curl -s -X POST "$BASE_URL/virtual_models/enterprise-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"anthropic-claude","ai_service_backend_namespace":"model-serving","weight":30,"priority":0}'

curl -s -X POST "$BASE_URL/virtual_models/enterprise-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"gemini-pro","ai_service_backend_namespace":"model-serving","weight":20,"priority":1}'

curl -s -X POST "$BASE_URL/virtual_models/enterprise-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"azure-openai","ai_service_backend_namespace":"model-serving","weight":10,"priority":2}'
```

### Verify in Kubernetes

```bash
# List all backends
kubectl get aiservicebackends -n model-serving

# List all routes
kubectl get aigatewayroutes -n model-serving

# Check traffic split for each model
kubectl get aigatewayroute <route-name> -n model-serving -o jsonpath='{.spec.rules[0].backendRefs}'
```

### Expected Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    4 AIServiceBackends                          │
├─────────────────┬─────────────────┬───────────────┬─────────────┤
│  openai-gpt4    │ anthropic-claude│  gemini-pro   │ azure-openai│
│  (OpenAI)       │ (Anthropic)     │  (OpenAI)     │ (AzureOpenAI)│
└────────┬────────┴────────┬────────┴───────┬───────┴──────┬──────┘
         │                 │                │              │
         ▼                 ▼                ▼              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    3 Virtual Models                             │
├─────────────────────────────────────────────────────────────────┤
│  chat-model          │  code-model         │  enterprise-model  │
│  ├─ openai: 80%      │  ├─ anthropic: 70%  │  ├─ openai: 40%    │
│  └─ anthropic: 20%   │  └─ openai: 30%     │  ├─ anthropic: 30% │
│                      │                     │  ├─ gemini: 20%    │
│                      │                     │  └─ azure: 10%     │
└─────────────────────────────────────────────────────────────────┘
```

### Traffic Split Summary

| Model | Backends | Weights | Priority |
|-------|----------|---------|----------|
| chat-model | openai-gpt4, anthropic-claude | 80/20 | 0, 1 |
| code-model | anthropic-claude, openai-gpt4 | 70/30 | 0, 1 |
| enterprise-model | openai, anthropic, gemini, azure | 40/30/20/10 | 0, 0, 1, 2 |

### Backend Validation Test

```bash
# This should FAIL - backend doesn't exist
curl -s -X POST "$BASE_URL/virtual_models/chat-model/backends" \
  -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"non-existent","ai_service_backend_namespace":"model-serving","weight":10}'

# Expected: {"error":"backend model-serving/non-existent not found: backend not found"}
```

### Cleanup

```bash
# Delete virtual models (also removes AIGatewayRoutes)
curl -s -X DELETE "$BASE_URL/virtual_models/chat-model" -H "Project-ID: $PROJECT_ID"
curl -s -X DELETE "$BASE_URL/virtual_models/code-model" -H "Project-ID: $PROJECT_ID"
curl -s -X DELETE "$BASE_URL/virtual_models/enterprise-model" -H "Project-ID: $PROJECT_ID"

# Delete backends
curl -s -X DELETE "$BASE_URL/ai_service_backends/openai-gpt4"
curl -s -X DELETE "$BASE_URL/ai_service_backends/anthropic-claude"
curl -s -X DELETE "$BASE_URL/ai_service_backends/gemini-pro"
curl -s -X DELETE "$BASE_URL/ai_service_backends/azure-openai"

# Verify cleanup
kubectl get aiservicebackends,aigatewayroutes -n model-serving
```

---

## Notes

- Default namespace is `model-serving` if not specified in request
- All timestamps use RFC3339 format
- Labels are stored as K8s metadata labels
- HeaderMutation applies to AI Gateway routing
- K8s verification confirms bidirectional sync between API and CRDs
- Backend validation ensures AIServiceBackend exists before linking to VirtualModel

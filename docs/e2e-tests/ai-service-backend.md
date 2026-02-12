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

## Notes

- Default namespace is `model-serving` if not specified in request
- All timestamps use RFC3339 format
- Labels are stored as K8s metadata labels
- HeaderMutation applies to AI Gateway routing
- K8s verification confirms bidirectional sync between API and CRDs

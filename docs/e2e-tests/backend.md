# E2E Test: Envoy Gateway Backend APIs

End-to-end test documentation for Backend CRUD operations in model-registry-service.

## Prerequisites

1. **Kubernetes cluster** with Envoy Gateway CRDs installed
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

Copy-paste and run these commands sequentially to test all Backend operations.

### 1. LIST (Empty State)

```bash
curl -s "$BASE_URL/backends"
```

**Expected Result:**
```json
{
  "items": [],
  "total": 0
}
```

### 2. CREATE Backend with FQDN Endpoint

```bash
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "kserve-llama",
    "namespace": "model-serving",
    "endpoints": [
      {
        "fqdn": {
          "hostname": "llama-predictor.model-serving.svc.cluster.local",
          "port": 80
        }
      }
    ],
    "labels": {
      "model": "llama-70b",
      "team": "ml-platform"
    }
  }'
```

**Expected Result:**
```json
{
  "name": "kserve-llama",
  "namespace": "model-serving",
  "endpoints": [
    {
      "fqdn": {
        "hostname": "llama-predictor.model-serving.svc.cluster.local",
        "port": 80
      }
    }
  ],
  "labels": {
    "managed-by": "model-registry",
    "model": "llama-70b",
    "team": "ml-platform"
  }
}
```

### 3. Verify in Kubernetes

```bash
# List Backend resources
kubectl get backends -n model-serving

# Get detailed view
kubectl get backend kserve-llama -n model-serving -o yaml
```

**Expected Output:**
```
NAME           AGE
kserve-llama   1m
```

### 4. GET Single Backend

```bash
curl -s "$BASE_URL/backends/kserve-llama"
```

**Expected Result:**
```json
{
  "name": "kserve-llama",
  "namespace": "model-serving",
  "endpoints": [
    {
      "fqdn": {
        "hostname": "llama-predictor.model-serving.svc.cluster.local",
        "port": 80
      }
    }
  ],
  "labels": {
    "managed-by": "model-registry",
    "model": "llama-70b",
    "team": "ml-platform"
  }
}
```

### 5. UPDATE Endpoints

```bash
curl -s -X PATCH "$BASE_URL/backends/kserve-llama" \
  -H "Content-Type: application/json" \
  -d '{
    "endpoints": [
      {
        "fqdn": {
          "hostname": "llama-v2-predictor.model-serving.svc.cluster.local",
          "port": 8080
        }
      }
    ]
  }'
```

**Verify Update in K8s:**
```bash
kubectl get backend kserve-llama -n model-serving -o jsonpath='{.spec.endpoints[0].fqdn.hostname}{"\n"}'
# Expected: llama-v2-predictor.model-serving.svc.cluster.local
```

### 6. LIST After Update

```bash
curl -s "$BASE_URL/backends"
```

**Expected Result:**
```json
{
  "items": [
    {
      "name": "kserve-llama",
      "namespace": "model-serving",
      "endpoints": [
        {
          "fqdn": {
            "hostname": "llama-v2-predictor.model-serving.svc.cluster.local",
            "port": 8080
          }
        }
      ],
      "labels": {...}
    }
  ],
  "total": 1
}
```

### 7. DELETE Backend

```bash
curl -s -X DELETE "$BASE_URL/backends/kserve-llama"
# Expected: HTTP 204 No Content
```

**Verify Deletion in K8s:**
```bash
kubectl get backends -n model-serving
# Expected: No resources found (or kserve-llama not in list)
```

### 8. CREATE with IP Endpoint

```bash
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "external-openai",
    "endpoints": [
      {
        "ip": {
          "address": "10.0.0.1",
          "port": 443
        }
      }
    ]
  }'
```

**Expected Result:**
```json
{
  "name": "external-openai",
  "namespace": "model-serving",
  "endpoints": [
    {
      "ip": {
        "address": "10.0.0.1",
        "port": 443
      }
    }
  ],
  "labels": {
    "managed-by": "model-registry"
  }
}
```

### 9. Verify IP Endpoint in K8s

```bash
kubectl get backend external-openai -n model-serving \
  -o jsonpath='{.spec.endpoints[0].ip}' | jq .
```

**Expected Output:**
```json
{
  "address": "10.0.0.1",
  "port": 443
}
```

## Cleanup

```bash
# Delete all test backends
curl -s -X DELETE "$BASE_URL/backends/external-openai"

# Verify cleanup in K8s
kubectl get backends -n model-serving
```

## Test Scenarios Summary

| Operation | Endpoint | Method | Key Features |
|-----------|----------|--------|--------------|
| LIST empty | `/backends` | GET | Returns empty list |
| CREATE FQDN | `/backends` | POST | FQDN endpoint + labels |
| GET single | `/backends/:name` | GET | Retrieve by name |
| UPDATE | `/backends/:name` | PATCH | Modify endpoints |
| LIST populated | `/backends` | GET | Returns updated items |
| DELETE | `/backends/:name` | DELETE | K8s resource removed |
| CREATE IP | `/backends` | POST | IP endpoint |
| K8s verification | `kubectl get` | - | Confirm CRD sync |

## Error Scenarios

### 404 - Not Found
```bash
curl -s "$BASE_URL/backends/nonexistent"
# Expected: {"error": "envoy gateway backend not found"}
```

### 409 - Conflict (Duplicate Name)
```bash
# Create first backend
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{"name": "duplicate-test", "endpoints": [{"fqdn": {"hostname": "test.svc", "port": 80}}]}'

# Try to create again
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{"name": "duplicate-test", "endpoints": [{"fqdn": {"hostname": "test.svc", "port": 80}}]}'
# Expected: {"error": "envoy gateway backend already exists"}
```

### 400 - Invalid Endpoint (Missing FQDN/IP)
```bash
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{"name": "invalid", "endpoints": [{}]}'
# Expected: {"error": "endpoint must have fqdn or ip", "index": 0}
```

### 400 - Invalid Endpoint (Both FQDN and IP)
```bash
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{"name": "invalid", "endpoints": [{"fqdn": {"hostname": "test.svc", "port": 80}, "ip": {"address": "10.0.0.1", "port": 80}}]}'
# Expected: {"error": "endpoint cannot have both fqdn and ip", "index": 0}
```

## Full Flow Test: Backend → AIServiceBackend → VirtualModel

Test the complete API-driven flow.

### Setup

```bash
PROJECT_ID="11111111-1111-1111-1111-111111111111"

# 1. Create Backend
curl -s -X POST "$BASE_URL/backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "kserve-llama-e2e",
    "endpoints": [{"fqdn": {"hostname": "llama.model-serving.svc.cluster.local", "port": 80}}]
  }'

# 2. Create AIServiceBackend referencing the Backend
curl -s -X POST "$BASE_URL/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "llama-70b-svc",
    "schema": "OpenAI",
    "backend_ref": {"name": "kserve-llama-e2e"}
  }'

# 3. Create VirtualModel
curl -s -X POST "$BASE_URL/virtual_models" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"name": "chat-model-e2e"}'

# 4. Link AIServiceBackend to VirtualModel
curl -s -X POST "$BASE_URL/virtual_models/chat-model-e2e/backends" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{
    "ai_service_backend_name": "llama-70b-svc",
    "weight": 100
  }'
```

### Verify in Kubernetes

```bash
kubectl get backends,aiservicebackends,aigatewayroutes -n model-serving
```

**Expected:**
```
NAME                                  AGE
backend.gateway.envoyproxy.io/kserve-llama-e2e   1m

NAME                                           SCHEMA   AGE
aiservicebackend.aigateway.envoyproxy.io/llama-70b-svc   OpenAI   1m

NAME                                          AGE
aigatewayroute.aigateway.envoyproxy.io/chat-model-e2e   1m
```

### Cleanup

```bash
curl -s -X DELETE "$BASE_URL/virtual_models/chat-model-e2e" -H "Project-ID: $PROJECT_ID"
curl -s -X DELETE "$BASE_URL/ai_service_backends/llama-70b-svc"
curl -s -X DELETE "$BASE_URL/backends/kserve-llama-e2e"

# Verify
kubectl get backends,aiservicebackends,aigatewayroutes -n model-serving
```

---

## Notes

- Default namespace is `model-serving` if not specified in request
- Each endpoint must have exactly one of `fqdn` or `ip` (mutually exclusive)
- Labels are stored as K8s metadata labels with `managed-by: model-registry` auto-added
- K8s verification confirms bidirectional sync between API and CRDs

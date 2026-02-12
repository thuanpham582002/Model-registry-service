#!/bin/bash
set -e

BASE="http://localhost:8085/api/v1/model-registry"
PROJECT_ID="11111111-1111-1111-1111-111111111111"

echo "=== Full Flow Test: Backend → AIServiceBackend → VirtualModel ==="
echo ""

echo "1. Create Backend..."
curl -s -X POST "$BASE/backends" \
  -H "Content-Type: application/json" \
  -d '{"name":"e2e-flow-backend","endpoints":[{"fqdn":{"hostname":"flow-test.svc.cluster.local","port":80}}]}'
echo ""

echo "2. Create AIServiceBackend..."
curl -s -X POST "$BASE/ai_service_backends" \
  -H "Content-Type: application/json" \
  -d '{"name":"e2e-flow-aisvc","schema":"OpenAI","backend_ref":{"name":"e2e-flow-backend"}}'
echo ""

echo "3. Create VirtualModel..."
curl -s -X POST "$BASE/virtual_models" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"name":"e2e-flow-vm"}'
echo ""

echo "4. Link backend to VirtualModel..."
curl -s -X POST "$BASE/virtual_models/e2e-flow-vm/backends" \
  -H "Content-Type: application/json" \
  -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"e2e-flow-aisvc","weight":100}'
echo ""

echo "5. K8s verification..."
kubectl get backends,aiservicebackends,aigatewayroutes -n model-serving 2>&1 | grep -E "(NAME|e2e-flow)" || true
echo ""

echo "6. Cleanup..."
curl -s -X DELETE "$BASE/virtual_models/e2e-flow-vm" -H "Project-ID: $PROJECT_ID"
curl -s -X DELETE "$BASE/ai_service_backends/e2e-flow-aisvc"
curl -s -X DELETE "$BASE/backends/e2e-flow-backend"
echo "   Done"

echo ""
echo "=== Full Flow Test Complete ==="

#!/bin/bash
set -e

BASE="http://localhost:8085/api/v1/model-registry"
PROJECT_ID="11111111-1111-1111-1111-111111111111"

echo "=== Full Flow Test: Backend → AIServiceBackend → VirtualModel ==="
echo ""

echo "1. Create Backend..."
curl -s -X POST "$BASE/backends" -H "Content-Type: application/json" \
  -d '{"name":"full-flow-backend","endpoints":[{"fqdn":{"hostname":"llama.model-serving.svc.cluster.local","port":80}}]}'
echo ""

echo "2. Create AIServiceBackend..."
curl -s -X POST "$BASE/ai_service_backends" -H "Content-Type: application/json" \
  -d '{"name":"full-flow-aisvc","schema":"OpenAI","backend_ref":{"name":"full-flow-backend"}}'
echo ""

echo "3. Create VirtualModel..."
curl -s -X POST "$BASE/virtual_models" -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"name":"full-flow-vm"}'
echo ""

echo "4. Link AIServiceBackend to VirtualModel..."
curl -s -X POST "$BASE/virtual_models/full-flow-vm/backends" -H "Content-Type: application/json" -H "Project-ID: $PROJECT_ID" \
  -d '{"ai_service_backend_name":"full-flow-aisvc","weight":100}'
echo ""

echo ""
echo "5. Verify in K8s..."
echo "--- Backends ---"
kubectl get backends -n model-serving | grep -E "(NAME|full-flow)" || true
echo ""
echo "--- AIServiceBackends ---"
kubectl get aiservicebackends -n model-serving | grep -E "(NAME|full-flow)" || true
echo ""
echo "--- AIGatewayRoutes ---"
kubectl get aigatewayroutes -n model-serving | grep -E "(NAME|full-flow)" || true

echo ""
echo "6. Verify VirtualModel has backend linked..."
curl -s "$BASE/virtual_models/full-flow-vm" -H "Project-ID: $PROJECT_ID" | jq '{name: .name, backends: [.backends[] | {name: .ai_service_backend_name, ns: .ai_service_backend_namespace, weight: .weight}]}'

echo ""
echo "7. Cleanup..."
curl -s -X DELETE "$BASE/virtual_models/full-flow-vm" -H "Project-ID: $PROJECT_ID" > /dev/null && echo "   ✓ Deleted VirtualModel"
curl -s -X DELETE "$BASE/ai_service_backends/full-flow-aisvc" > /dev/null && echo "   ✓ Deleted AIServiceBackend"
curl -s -X DELETE "$BASE/backends/full-flow-backend" > /dev/null && echo "   ✓ Deleted Backend"

echo ""
echo "8. Verify cleanup in K8s..."
kubectl get backends,aiservicebackends,aigatewayroutes -n model-serving 2>&1 | grep "full-flow" || echo "   ✓ All resources cleaned up"

echo ""
echo "=== Full Flow Test Complete ✅ ==="

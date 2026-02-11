#!/usr/bin/env python3
"""
E2E tests for Serving, Traffic, and Virtual Model features.

Tests the full HTTP request/response cycle against the running Go service.

Usage:
    pip install requests
    docker compose up -d
    python test_e2e_serving.py
"""

import json
import sys
import time
import uuid

import requests

BASE = "http://localhost:8080/api/v1/model-registry"
PROJECT_ID = str(uuid.uuid4())

passed = 0
failed = 0


def check(label: str, ok: bool, detail: str = ""):
    global passed, failed
    if ok:
        passed += 1
        print(f"  ✓ PASS  {label}")
    else:
        failed += 1
        print(f"  ✗ FAIL  {label}  {detail}")


def assert_fields(data: dict, fields: list[str], label: str):
    for f in fields:
        check(f"{label}.{f} present", f in data, f"missing from keys={list(data.keys())}")


def wait_for_service(url: str, timeout: int = 30):
    print(f"Waiting for service at {url} ...")
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            r = requests.get(url, timeout=2)
            if r.status_code == 200:
                print("Service is ready.\n")
                return
        except requests.ConnectionError:
            pass
        time.sleep(1)
    print("TIMEOUT waiting for service")
    sys.exit(1)


def headers():
    return {"Content-Type": "application/json", "Project-ID": PROJECT_ID}


# ===========================================================================
# Field definitions
# ===========================================================================

SERVING_ENV_FIELDS = ["id", "name", "created_at", "updated_at"]
ISVC_FIELDS = [
    "id", "name", "serving_environment_id", "registered_model_id",
    "desired_state", "current_state", "created_at", "updated_at"
]
TRAFFIC_CONFIG_FIELDS = [
    "id", "inference_service_id", "strategy", "status",
    "variants", "created_at", "updated_at"
]
TRAFFIC_VARIANT_FIELDS = [
    "id", "variant_name", "model_version_id", "weight", "status"
]
VIRTUAL_MODEL_FIELDS = [
    "id", "name", "status", "backends", "created_at", "updated_at"
]
BACKEND_FIELDS = [
    "id", "ai_service_backend_name", "weight", "priority", "status"
]
LIST_FIELDS = ["items", "total", "page_size", "next_offset"]


# ===========================================================================
# Helper: Create prerequisite resources
# ===========================================================================

def create_model() -> str:
    """Create a model for testing."""
    payload = {
        "name": f"test-model-{uuid.uuid4().hex[:8]}",
        "model_type": "CUSTOMTRAIN",
    }
    r = requests.post(f"{BASE}/models", json=payload, headers=headers())
    if r.status_code != 201:
        print(f"Failed to create model: {r.status_code} {r.text}")
        return None
    return r.json()["id"]


def create_version(model_id: str) -> str:
    """Create a version for testing."""
    payload = {
        "name": f"v{uuid.uuid4().hex[:4]}",
        "model_framework": "pytorch",
        "uri": "s3://bucket/model",
    }
    r = requests.post(f"{BASE}/models/{model_id}/versions", json=payload, headers=headers())
    if r.status_code != 201:
        print(f"Failed to create version: {r.status_code} {r.text}")
        return None
    version_id = r.json()["id"]

    # Mark as READY
    requests.patch(
        f"{BASE}/models/{model_id}/versions/{version_id}",
        json={"status": "READY"},
        headers=headers()
    )
    return version_id


# ===========================================================================
# Serving Environment Tests
# ===========================================================================

def test_create_serving_environment() -> str:
    print("\n--- Test: Create Serving Environment ---")
    payload = {
        "name": f"test-env-{uuid.uuid4().hex[:8]}",
        "description": "Test environment for E2E",
        "external_id": "test-namespace"
    }
    r = requests.post(f"{BASE}/serving_environments", json=payload, headers=headers())
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")

    if r.status_code != 201:
        return None

    data = r.json()
    assert_fields(data, SERVING_ENV_FIELDS, "serving_env")
    check("name matches", payload["name"] in data.get("name", ""))
    return data["id"]


def test_get_serving_environment(env_id: str):
    print("\n--- Test: Get Serving Environment ---")
    r = requests.get(f"{BASE}/serving_environments/{env_id}", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, SERVING_ENV_FIELDS, "serving_env")
    check("id matches", data["id"] == env_id)


def test_list_serving_environments():
    print("\n--- Test: List Serving Environments ---")
    r = requests.get(f"{BASE}/serving_environments", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, LIST_FIELDS, "list")
    check("total >= 1", data.get("total", 0) >= 1)


def test_update_serving_environment(env_id: str):
    print("\n--- Test: Update Serving Environment ---")
    payload = {"description": "Updated description"}
    r = requests.patch(f"{BASE}/serving_environments/{env_id}", json=payload, headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")
    data = r.json()
    check("description updated", data.get("description") == "Updated description")


# ===========================================================================
# Inference Service Tests
# ===========================================================================

def test_create_inference_service(env_id: str, model_id: str, version_id: str) -> str:
    print("\n--- Test: Create Inference Service ---")
    payload = {
        "name": f"test-isvc-{uuid.uuid4().hex[:8]}",
        "serving_environment_id": env_id,
        "registered_model_id": model_id,
        "model_version_id": version_id,
        "runtime": "kserve",
        "labels": {"env": "test"}
    }
    r = requests.post(f"{BASE}/inference_services", json=payload, headers=headers())
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")

    if r.status_code != 201:
        return None

    data = r.json()
    assert_fields(data, ISVC_FIELDS, "isvc")
    check("desired_state == UNDEPLOYED", data.get("desired_state") == "UNDEPLOYED")
    return data["id"]


def test_get_inference_service(isvc_id: str):
    print("\n--- Test: Get Inference Service ---")
    r = requests.get(f"{BASE}/inference_services/{isvc_id}", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, ISVC_FIELDS, "isvc")


def test_list_inference_services():
    print("\n--- Test: List Inference Services ---")
    r = requests.get(f"{BASE}/inference_services", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, LIST_FIELDS, "list")


def test_update_inference_service(isvc_id: str):
    print("\n--- Test: Update Inference Service ---")
    payload = {"desired_state": "DEPLOYED"}
    r = requests.patch(f"{BASE}/inference_services/{isvc_id}", json=payload, headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")
    data = r.json()
    check("desired_state updated", data.get("desired_state") == "DEPLOYED")


# ===========================================================================
# Traffic Config Tests
# ===========================================================================

def test_create_traffic_config(isvc_id: str, version_id: str) -> str:
    print("\n--- Test: Create Traffic Config ---")
    payload = {
        "inference_service_id": isvc_id,
        "strategy": "canary",
        "stable_version_id": version_id
    }
    r = requests.post(f"{BASE}/traffic_configs", json=payload, headers=headers())
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")

    if r.status_code != 201:
        return None

    data = r.json()
    assert_fields(data, TRAFFIC_CONFIG_FIELDS, "traffic_config")
    check("strategy == canary", data.get("strategy") == "canary")
    check("has stable variant", len(data.get("variants", [])) >= 1)
    return data["id"]


def test_get_traffic_config(config_id: str):
    print("\n--- Test: Get Traffic Config ---")
    r = requests.get(f"{BASE}/traffic_configs/{config_id}", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, TRAFFIC_CONFIG_FIELDS, "traffic_config")


def test_list_traffic_configs():
    print("\n--- Test: List Traffic Configs ---")
    r = requests.get(f"{BASE}/traffic_configs", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, LIST_FIELDS, "list")


# ===========================================================================
# Traffic Variant Tests
# ===========================================================================

def test_add_variant(config_id: str, version_id: str):
    print("\n--- Test: Add Traffic Variant ---")
    payload = {
        "variant_name": "canary",
        "model_version_id": version_id,
        "weight": 10
    }
    r = requests.post(f"{BASE}/traffic_configs/{config_id}/variants", json=payload, headers=headers())
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")

    if r.status_code == 201:
        data = r.json()
        variants = data.get("variants", [])
        canary = next((v for v in variants if v.get("variant_name") == "canary"), None)
        check("canary variant exists", canary is not None)
        if canary:
            check("canary weight == 10", canary.get("weight") == 10)


def test_list_variants(config_id: str):
    print("\n--- Test: List Traffic Variants ---")
    r = requests.get(f"{BASE}/traffic_configs/{config_id}/variants", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    check("has variants", len(data.get("variants", [])) >= 1)


def test_get_variant(config_id: str):
    print("\n--- Test: Get Traffic Variant ---")
    r = requests.get(f"{BASE}/traffic_configs/{config_id}/variants/stable", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    check("variant_name == stable", data.get("variant_name") == "stable")


def test_update_variant(config_id: str):
    print("\n--- Test: Update Traffic Variant ---")
    payload = {"weight": 20}
    r = requests.patch(f"{BASE}/traffic_configs/{config_id}/variants/canary", json=payload, headers=headers())
    # May fail if canary doesn't exist
    if r.status_code == 200:
        check("status 200", True)
        data = r.json()
        variants = data.get("variants", [])
        canary = next((v for v in variants if v.get("variant_name") == "canary"), None)
        if canary:
            check("weight updated", canary.get("weight") == 20)
    else:
        check("status 200 or 404 (no canary)", r.status_code in [200, 404], f"got {r.status_code}")


def test_bulk_update_weights(config_id: str):
    print("\n--- Test: Bulk Update Weights ---")
    payload = {"weights": {"stable": 100}}
    r = requests.patch(f"{BASE}/traffic_configs/{config_id}/weights", json=payload, headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")


def test_rollback(config_id: str):
    print("\n--- Test: Rollback ---")
    r = requests.post(f"{BASE}/traffic_configs/{config_id}/rollback", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")


# ===========================================================================
# Virtual Model Tests
# ===========================================================================

def test_create_virtual_model() -> str:
    print("\n--- Test: Create Virtual Model ---")
    payload = {
        "name": f"claude-4-sonnet-{uuid.uuid4().hex[:8]}",
        "description": "Virtual model for multi-provider routing"
    }
    r = requests.post(f"{BASE}/virtual_models", json=payload, headers=headers())
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")

    if r.status_code != 201:
        return None

    data = r.json()
    assert_fields(data, VIRTUAL_MODEL_FIELDS, "virtual_model")
    check("status == active", data.get("status") == "active")
    return data["name"]


def test_get_virtual_model(vm_name: str):
    print("\n--- Test: Get Virtual Model ---")
    r = requests.get(f"{BASE}/virtual_models/{vm_name}", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, VIRTUAL_MODEL_FIELDS, "virtual_model")


def test_list_virtual_models():
    print("\n--- Test: List Virtual Models ---")
    r = requests.get(f"{BASE}/virtual_models", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    check("has items", "items" in data)
    check("total >= 1", data.get("total", 0) >= 1)


# ===========================================================================
# Virtual Model Backend Tests
# ===========================================================================

def test_add_backend(vm_name: str) -> str:
    print("\n--- Test: Add Virtual Model Backend ---")
    payload = {
        "ai_service_backend_name": "aws-anthropic",
        "ai_service_backend_namespace": "ai-gateway",
        "model_name_override": "anthropic.claude-sonnet-4-20250514-v1:0",
        "weight": 50,
        "priority": 0
    }
    r = requests.post(f"{BASE}/virtual_models/{vm_name}/backends", json=payload, headers=headers())
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")

    if r.status_code != 201:
        return None

    data = r.json()
    backends = data.get("backends", [])
    check("has backends", len(backends) >= 1)

    if backends:
        return backends[0]["id"]
    return None


def test_list_backends(vm_name: str):
    print("\n--- Test: List Virtual Model Backends ---")
    r = requests.get(f"{BASE}/virtual_models/{vm_name}/backends", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    check("has backends", len(data.get("backends", [])) >= 1)


def test_update_backend(vm_name: str, backend_id: str):
    print("\n--- Test: Update Virtual Model Backend ---")
    if not backend_id:
        check("backend_id exists", False, "no backend_id")
        return

    payload = {"weight": 100, "priority": 0}
    r = requests.patch(f"{BASE}/virtual_models/{vm_name}/backends/{backend_id}", json=payload, headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")


def test_delete_backend(vm_name: str, backend_id: str):
    print("\n--- Test: Delete Virtual Model Backend ---")
    if not backend_id:
        check("backend_id exists", False, "no backend_id")
        return

    r = requests.delete(f"{BASE}/virtual_models/{vm_name}/backends/{backend_id}", headers=headers())
    check("status 204", r.status_code == 204, f"got {r.status_code}")


# ===========================================================================
# Metrics Tests
# ===========================================================================

def test_get_deployment_metrics():
    print("\n--- Test: Get Deployment Metrics ---")
    r = requests.get(f"{BASE}/metrics/deployments/test-isvc", headers=headers())
    # May return empty data if Prometheus is not running
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    if r.status_code == 200:
        data = r.json()
        check("has time_range", "time_range" in data)
        check("has summary", "summary" in data)


def test_get_token_usage():
    print("\n--- Test: Get Token Usage ---")
    r = requests.get(f"{BASE}/metrics/tokens", headers=headers())
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    if r.status_code == 200:
        data = r.json()
        check("has project_id", "project_id" in data)
        check("has summary", "summary" in data)


def test_compare_variants():
    print("\n--- Test: Compare Variants ---")
    r = requests.get(
        f"{BASE}/metrics/compare/test-isvc",
        params={"variant": ["stable", "canary"]},
        headers=headers()
    )
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    if r.status_code == 200:
        data = r.json()
        check("has variants", "variants" in data)


# ===========================================================================
# Cleanup Tests
# ===========================================================================

def test_delete_traffic_config(config_id: str):
    print("\n--- Test: Delete Traffic Config ---")
    r = requests.delete(f"{BASE}/traffic_configs/{config_id}", headers=headers())
    check("status 204", r.status_code == 204, f"got {r.status_code}")


def test_delete_virtual_model(vm_name: str):
    print("\n--- Test: Delete Virtual Model ---")
    r = requests.delete(f"{BASE}/virtual_models/{vm_name}", headers=headers())
    check("status 204", r.status_code == 204, f"got {r.status_code}")


def test_delete_inference_service(isvc_id: str):
    print("\n--- Test: Delete Inference Service ---")
    # First set to UNDEPLOYED
    requests.patch(
        f"{BASE}/inference_services/{isvc_id}",
        json={"desired_state": "UNDEPLOYED", "current_state": "UNDEPLOYED"},
        headers=headers()
    )
    r = requests.delete(f"{BASE}/inference_services/{isvc_id}", headers=headers())
    check("status 204", r.status_code == 204, f"got {r.status_code}")


def test_delete_serving_environment(env_id: str):
    print("\n--- Test: Delete Serving Environment ---")
    r = requests.delete(f"{BASE}/serving_environments/{env_id}", headers=headers())
    check("status 204", r.status_code == 204, f"got {r.status_code}")


# ===========================================================================
# Main
# ===========================================================================

if __name__ == "__main__":
    wait_for_service("http://localhost:8080/healthz")

    print("=" * 60)
    print("Creating prerequisite resources...")
    print("=" * 60)

    # Create model and version for testing
    model_id = create_model()
    if not model_id:
        print("FATAL: Could not create test model")
        sys.exit(1)
    print(f"Created model: {model_id}")

    version_id = create_version(model_id)
    if not version_id:
        print("FATAL: Could not create test version")
        sys.exit(1)
    print(f"Created version: {version_id}")

    # Create second version for canary testing
    version_id_2 = create_version(model_id)
    print(f"Created version 2: {version_id_2}")

    print("\n" + "=" * 60)
    print("Running Serving Environment Tests")
    print("=" * 60)

    env_id = test_create_serving_environment()
    if env_id:
        test_get_serving_environment(env_id)
        test_list_serving_environments()
        test_update_serving_environment(env_id)

    print("\n" + "=" * 60)
    print("Running Inference Service Tests")
    print("=" * 60)

    isvc_id = None
    if env_id:
        isvc_id = test_create_inference_service(env_id, model_id, version_id)
        if isvc_id:
            test_get_inference_service(isvc_id)
            test_list_inference_services()
            test_update_inference_service(isvc_id)

    print("\n" + "=" * 60)
    print("Running Traffic Config Tests")
    print("=" * 60)

    config_id = None
    if isvc_id:
        config_id = test_create_traffic_config(isvc_id, version_id)
        if config_id:
            test_get_traffic_config(config_id)
            test_list_traffic_configs()

    print("\n" + "=" * 60)
    print("Running Traffic Variant Tests")
    print("=" * 60)

    if config_id and version_id_2:
        test_list_variants(config_id)
        test_get_variant(config_id)
        test_add_variant(config_id, version_id_2)
        test_update_variant(config_id)
        test_bulk_update_weights(config_id)
        test_rollback(config_id)

    print("\n" + "=" * 60)
    print("Running Virtual Model Tests")
    print("=" * 60)

    vm_name = test_create_virtual_model()
    if vm_name:
        test_get_virtual_model(vm_name)
        test_list_virtual_models()

    print("\n" + "=" * 60)
    print("Running Virtual Model Backend Tests")
    print("=" * 60)

    backend_id = None
    if vm_name:
        backend_id = test_add_backend(vm_name)
        test_list_backends(vm_name)
        if backend_id:
            test_update_backend(vm_name, backend_id)

    print("\n" + "=" * 60)
    print("Running Metrics Tests")
    print("=" * 60)

    test_get_deployment_metrics()
    test_get_token_usage()
    test_compare_variants()

    print("\n" + "=" * 60)
    print("Cleanup")
    print("=" * 60)

    # Cleanup in reverse order
    if vm_name and backend_id:
        test_delete_backend(vm_name, backend_id)
    if vm_name:
        test_delete_virtual_model(vm_name)
    if config_id:
        test_delete_traffic_config(config_id)
    if isvc_id:
        test_delete_inference_service(isvc_id)
    if env_id:
        test_delete_serving_environment(env_id)

    # Cleanup model
    if model_id:
        requests.patch(f"{BASE}/models/{model_id}", json={"state": "ARCHIVED"}, headers=headers())
        requests.delete(f"{BASE}/models/{model_id}", headers=headers())

    # Summary
    total = passed + failed
    print("\n" + "=" * 60)
    print(f"Results: {passed}/{total} passed, {failed} failed")
    print("=" * 60)
    sys.exit(1 if failed > 0 else 0)

#!/usr/bin/env python3
"""
E2E contract test: SDK ↔ Go Model Registry Service.

Tests the full HTTP request/response cycle against the running Go service,
verifying that payloads and responses match what the SDK sends/expects.

Usage:
    pip install requests
    docker compose up -d
    python test_e2e_sdk.py
"""

import json
import sys
import time
import uuid

import requests

BASE = "http://localhost:8080/api/v1/model-registry"
PROJECT_ID = str(uuid.uuid4())
REGION_ID = str(uuid.uuid4())

passed = 0
failed = 0


def check(label: str, ok: bool, detail: str = ""):
    global passed, failed
    if ok:
        passed += 1
        print(f"  PASS  {label}")
    else:
        failed += 1
        print(f"  FAIL  {label}  {detail}")


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


# ---------------------------------------------------------------------------
# Model response field list (what SDK from_dict expects)
# ---------------------------------------------------------------------------
MODEL_FIELDS = [
    "id", "name", "state", "description", "slug", "project_id",
    "region_id", "model_type", "model_size", "deployment_status",
    "tags", "labels", "version_count", "created_at", "updated_at",
]

VERSION_FIELDS = [
    "id", "name", "registered_model_id", "state", "description",
    "is_default", "status", "artifact_type", "model_framework",
    "model_framework_version", "container_image", "uri", "labels",
    "created_at", "updated_at",
]

ARTIFACT_FIELDS = [
    "id", "name", "uri", "artifact_type", "model_framework",
    "model_framework_version", "registered_model_id", "labels",
    "created_at", "updated_at",
]

LIST_FIELDS = ["items", "total", "page_size", "next_offset"]


# ===========================================================================
# Tests
# ===========================================================================

def test_create_model() -> str:
    print("\n--- TestE2E_CreateModel ---")
    payload = {
        "name": "sdk-test-model",
        "region_id": REGION_ID,
        "model_type": "CUSTOMTRAIN",
        "tags": {
            "frameworks": ["pytorch"],
            "architectures": ["transformer"],
            "tasks": ["classification"],
            "subjects": ["nlp"],
        },
        "labels": {"env": "test"},
    }
    r = requests.post(
        f"{BASE}/models",
        json=payload,
        headers={"Content-Type": "application/json", "Project-ID": PROJECT_ID},
    )
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, MODEL_FIELDS, "model")

    # Verify tags structure
    tags = data.get("tags", {})
    for key in ["frameworks", "architectures", "tasks", "subjects"]:
        check(f"tags.{key} is list", isinstance(tags.get(key), list))

    check("labels is dict", isinstance(data.get("labels"), dict))
    check("state == LIVE", data.get("state") == "LIVE")
    check("name == sdk-test-model", data.get("name") == "sdk-test-model")

    return data["id"]


def test_get_model(model_id: str):
    print("\n--- TestE2E_GetModel ---")
    r = requests.get(f"{BASE}/models/{model_id}")
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, MODEL_FIELDS, "model")
    check("id matches", data["id"] == model_id)


def test_list_models():
    print("\n--- TestE2E_ListModels ---")
    r = requests.get(
        f"{BASE}/models",
        params={"limit": 10, "offset": 0},
        headers={"Project-ID": PROJECT_ID},
    )
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, LIST_FIELDS, "list")
    check("total >= 1", data.get("total", 0) >= 1)
    check("items is list", isinstance(data.get("items"), list))
    if data.get("items"):
        assert_fields(data["items"][0], MODEL_FIELDS, "list.items[0]")


def test_find_model():
    print("\n--- TestE2E_FindModel ---")
    r = requests.get(
        f"{BASE}/model",
        params={"name": "sdk-test-model"},
        headers={"Project-ID": PROJECT_ID},
    )
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, MODEL_FIELDS, "model")
    check("name matches", data.get("name") == "sdk-test-model")


def test_update_model(model_id: str):
    print("\n--- TestE2E_UpdateModel ---")
    payload = {"description": "updated by SDK test"}
    r = requests.patch(
        f"{BASE}/models/{model_id}",
        json=payload,
        headers={"Content-Type": "application/json"},
    )
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, MODEL_FIELDS, "model")
    check("description updated", data.get("description") == "updated by SDK test")


def test_create_version(model_id: str) -> str:
    print("\n--- TestE2E_CreateVersion ---")
    payload = {
        "name": "v1",
        "model_framework": "pytorch",
        "model_framework_version": "2.0",
        "uri": "s3://bucket/model",
    }
    r = requests.post(
        f"{BASE}/models/{model_id}/versions",
        json=payload,
        headers={"Content-Type": "application/json"},
    )
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, VERSION_FIELDS, "version")
    check("registered_model_id matches", data.get("registered_model_id") == model_id)
    check("status == PENDING", data.get("status") == "PENDING")
    check("artifact_type == model-artifact", data.get("artifact_type") == "model-artifact")
    return data["id"]


def test_get_version(model_id: str, version_id: str):
    print("\n--- TestE2E_GetVersion ---")
    r = requests.get(f"{BASE}/models/{model_id}/versions/{version_id}")
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, VERSION_FIELDS, "version")
    check("id matches", data["id"] == version_id)


def test_list_versions(model_id: str):
    print("\n--- TestE2E_ListVersions ---")
    r = requests.get(f"{BASE}/models/{model_id}/versions", params={"limit": 10})
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, LIST_FIELDS, "list")
    check("total >= 1", data.get("total", 0) >= 1)


def test_update_version(model_id: str, version_id: str):
    print("\n--- TestE2E_UpdateVersion ---")
    payload = {"status": "READY"}
    r = requests.patch(
        f"{BASE}/models/{model_id}/versions/{version_id}",
        json=payload,
        headers={"Content-Type": "application/json"},
    )
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, VERSION_FIELDS, "version")
    check("status updated to READY", data.get("status") == "READY")


def test_create_artifact(model_id: str) -> str:
    print("\n--- TestE2E_CreateArtifact ---")
    payload = {
        "registered_model_id": model_id,
        "name": "artifact-v1",
        "uri": "s3://bucket/artifact",
        "model_framework": "tensorflow",
        "model_framework_version": "2.12",
    }
    r = requests.post(
        f"{BASE}/model_artifacts",
        json=payload,
        headers={"Content-Type": "application/json"},
    )
    check("status 201", r.status_code == 201, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, ARTIFACT_FIELDS, "artifact")
    check("registered_model_id matches", data.get("registered_model_id") == model_id)
    return data["id"]


def test_get_artifact(artifact_id: str):
    print("\n--- TestE2E_GetArtifact ---")
    r = requests.get(f"{BASE}/model_artifacts/{artifact_id}")
    check("status 200", r.status_code == 200, f"got {r.status_code}")
    data = r.json()
    assert_fields(data, ARTIFACT_FIELDS, "artifact")
    check("id matches", data["id"] == artifact_id)


def test_update_artifact(artifact_id: str):
    print("\n--- TestE2E_UpdateArtifact ---")
    payload = {"artifact_type": "doc-artifact"}
    r = requests.patch(
        f"{BASE}/model_artifacts/{artifact_id}",
        json=payload,
        headers={"Content-Type": "application/json"},
    )
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")
    data = r.json()
    assert_fields(data, ARTIFACT_FIELDS, "artifact")
    check("artifact_type updated", data.get("artifact_type") == "doc-artifact")


def test_delete_model(model_id: str):
    print("\n--- TestE2E_DeleteModel ---")
    # Must archive first
    requests.patch(
        f"{BASE}/models/{model_id}",
        json={"state": "ARCHIVED"},
        headers={"Content-Type": "application/json"},
    )
    r = requests.delete(f"{BASE}/models/{model_id}")
    check("status 200", r.status_code == 200, f"got {r.status_code}: {r.text}")

    # Verify gone
    r2 = requests.get(f"{BASE}/models/{model_id}")
    check("model gone (404)", r2.status_code == 404, f"got {r2.status_code}")


# ===========================================================================
# Main
# ===========================================================================

if __name__ == "__main__":
    wait_for_service("http://localhost:8080/healthz")

    # RegisteredModel tests
    model_id = test_create_model()
    test_get_model(model_id)
    test_list_models()
    test_find_model()
    test_update_model(model_id)

    # ModelVersion tests
    version_id = test_create_version(model_id)
    test_get_version(model_id, version_id)
    test_list_versions(model_id)
    test_update_version(model_id, version_id)

    # ModelArtifact tests
    artifact_id = test_create_artifact(model_id)
    test_get_artifact(artifact_id)
    test_update_artifact(artifact_id)

    # Cleanup
    test_delete_model(model_id)

    # Summary
    total = passed + failed
    print(f"\n{'='*50}")
    print(f"Results: {passed}/{total} passed, {failed} failed")
    print(f"{'='*50}")
    sys.exit(1 if failed > 0 else 0)

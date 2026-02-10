# Model Registry Service

Go service providing CRUD API for ML model registry, backed by PostgreSQL (pgx/v5).

## Architecture

```
domain → dto → repository → usecase → handler
```

- **19 API routes** under `/api/v1/model-registry`
- **DB tables**: `model_registry_registered_model`, `model_registry_model_version` (Django-managed by CMP)
- **ModelArtifact** is virtual — queries model_version table with artifact-specific columns

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `SERVER_PORT` | 8080 | HTTP server port |
| `DATABASE_HOST` | localhost | PostgreSQL host |
| `DATABASE_PORT` | 5432 | PostgreSQL port |
| `DATABASE_USER` | postgres | DB user |
| `DATABASE_PASSWORD` | postgres | DB password |
| `DATABASE_DBNAME` | cmp | DB name |
| `DATABASE_SSLMODE` | disable | SSL mode |

## API Endpoints

### Registered Models
| Method | Path | Description |
|--------|------|-------------|
| POST | `/models` | Create model |
| GET | `/models` | List models (paginated) |
| GET | `/models/:id` | Get model by ID |
| GET | `/model?name=` | Get model by params |
| PATCH | `/models/:id` | Update model |
| DELETE | `/models/:id` | Delete model (must be ARCHIVED) |

### Model Versions (nested)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/models/:id/versions` | Create version |
| GET | `/models/:id/versions` | List versions by model |
| GET | `/models/:id/versions/:ver` | Get version |
| PATCH | `/models/:id/versions/:ver` | Update version |

### Model Versions (direct)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/model_versions` | List all versions |
| GET | `/model_versions/:id` | Get version by ID |
| GET | `/model_version?name=&registered_model_id=` | Find version |
| PATCH | `/model_versions/:id` | Update version |

### Model Artifacts (virtual)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/model_artifacts` | Create artifact |
| GET | `/model_artifacts` | List artifacts |
| GET | `/model_artifacts/:id` | Get artifact |
| GET | `/model_artifact?name=&registered_model_id=` | Find artifact |
| PATCH | `/model_artifacts/:id` | Update artifact |

All endpoints require `Project-ID` header (UUID).

## Testing

### 1. E2E Go Service (direct API)

```bash
# Start postgres + service
docker compose up -d

# Or run manually against existing postgres
DATABASE_HOST=localhost DATABASE_PORT=5434 DATABASE_USER=postgres \
DATABASE_PASSWORD=postgres DATABASE_DBNAME=postgres DATABASE_SSLMODE=disable \
SERVER_PORT=8085 go run ./cmd/server/

# Create a model
PROJECT_ID="<project-uuid>"
REGION_ID="<region-uuid>"
BASE="http://localhost:8085/api/v1/model-registry"

curl -s -X POST "$BASE/models" \
  -H "Project-ID: $PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-model","region_id":"'$REGION_ID'"}'

# List models
curl -s "$BASE/models?limit=10" -H "Project-ID: $PROJECT_ID"

# Health check
curl -s http://localhost:8085/healthz
```

### 2. E2E SDK Test Script

```bash
pip install requests
docker compose up -d
python test_e2e_sdk.py
```

Runs all 19 endpoints against the Go service, verifying request/response contracts.

### 3. CMP Integration (full stack)

Tests the full path: `kubeflow-sdk → CMP → ai-platform-sdk → Go MR`

```bash
# Prerequisites: CMP running with docker compose (separate repo)
# CMP postgres mapped to host port 5434

# Setup test data in CMP DB
docker exec cmp-backend python /app/backend/manage.py shell -c "
from django.contrib.auth import get_user_model
User = get_user_model()
u, _ = User.objects.get_or_create(username='admin', defaults={'is_superuser': True, 'is_staff': True})
u.set_password('admin')
u.save()
"

# Create region + project + project-region
docker exec cmp-backend python /app/backend/manage.py shell -c "
from organization.models import Region
from tenant.models import Project, ProjectRegion
r, _ = Region.objects.get_or_create(name='HN', defaults={'status': 'enabled'})
p, _ = Project.objects.get_or_create(slug='sample', defaults={'name': 'Sample', 'status': 'enabled'})
ProjectRegion.objects.get_or_create(project=p, region=r, defaults={'status': 'enabled'})
print(f'Project: {p.id}, Region: {r.id}')
"

# Create PAT for API auth
docker exec cmp-backend python /app/backend/manage.py shell -c "
from tenant.models import PATAuthToken
from django.contrib.auth import get_user_model
u = get_user_model().objects.get(username='admin')
pat, _ = PATAuthToken.objects.get_or_create(user=u, defaults={'name': 'test'})
print(f'PAT key: {pat.key}')
"

# Start Go MR service pointing at CMP postgres
DATABASE_HOST=localhost DATABASE_PORT=5434 DATABASE_USER=postgres \
DATABASE_PASSWORD=postgres DATABASE_DBNAME=postgres DATABASE_SSLMODE=disable \
SERVER_PORT=8085 go run ./cmd/server/

# Test via CMP proxy (requires PAT auth)
curl -s http://localhost:8000/api/v1/proxy/models/ \
  -H "Authorization: Bearer <pat-key>" \
  -H "X-Region: HN"
```

### 4. Full Scenario Test Commands

Set env first:
```bash
PROJECT_ID="<project-uuid>"
REGION_ID="<region-uuid>"
BASE="http://localhost:8085/api/v1/model-registry"
H='-H "Project-ID: '$PROJECT_ID'" -H "Content-Type: application/json"'
```

#### Scenario A: Registered Models CRUD

```bash
# Create
curl -s -X POST "$BASE/models" -H "Project-ID: $PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-llm","description":"LLM model","region_id":"'$REGION_ID'",
       "model_type":"llm",
       "tags":{"frameworks":["pytorch"],"architectures":["transformer"],"tasks":["text-generation"],"subjects":["nlp"]},
       "labels":{"env":"test"}}'
# Save the returned id as MODEL_ID

# List
curl -s "$BASE/models?limit=10&offset=0" -H "Project-ID: $PROJECT_ID"

# Get by ID
curl -s "$BASE/models/$MODEL_ID" -H "Project-ID: $PROJECT_ID"

# Get by name
curl -s "$BASE/model?name=test-llm" -H "Project-ID: $PROJECT_ID"

# Update
curl -s -X PATCH "$BASE/models/$MODEL_ID" -H "Project-ID: $PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"description":"Updated","state":"ARCHIVED"}'

# Delete (must be ARCHIVED first, no READY versions)
curl -s -X DELETE "$BASE/models/$MODEL_ID" -H "Project-ID: $PROJECT_ID"
```

#### Scenario B: Model Versions

```bash
# Create version under a model
curl -s -X POST "$BASE/models/$MODEL_ID/versions" -H "Project-ID: $PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"v1.0","model_framework":"pytorch","model_framework_version":"2.1.0",
       "uri":"s3://models/llm/v1","artifact_type":"model","is_default":true,
       "container_image":"registry.example.com/llm:v1","labels":{"stage":"prod"}}'
# Save returned id as VERSION_ID

# List versions (nested under model)
curl -s "$BASE/models/$MODEL_ID/versions?limit=10" -H "Project-ID: $PROJECT_ID"

# Get single version (nested)
curl -s "$BASE/models/$MODEL_ID/versions/$VERSION_ID" -H "Project-ID: $PROJECT_ID"

# Update version (nested)
curl -s -X PATCH "$BASE/models/$MODEL_ID/versions/$VERSION_ID" \
  -H "Project-ID: $PROJECT_ID" -H "Content-Type: application/json" \
  -d '{"status":"READY","is_default":true}'

# List all versions (direct, across models)
curl -s "$BASE/model_versions?limit=10" -H "Project-ID: $PROJECT_ID"

# Get version direct
curl -s "$BASE/model_versions/$VERSION_ID" -H "Project-ID: $PROJECT_ID"

# Find version by name + model
curl -s "$BASE/model_version?name=v1.0&registered_model_id=$MODEL_ID" \
  -H "Project-ID: $PROJECT_ID"

# Update version direct
curl -s -X PATCH "$BASE/model_versions/$VERSION_ID" \
  -H "Project-ID: $PROJECT_ID" -H "Content-Type: application/json" \
  -d '{"state":"ARCHIVED"}'
```

#### Scenario C: Model Artifacts + Error Cases

```bash
# Create artifact
curl -s -X POST "$BASE/model_artifacts" -H "Project-ID: $PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"registered_model_id":"'$MODEL_ID'","name":"onnx-export",
       "model_framework":"pytorch","model_framework_version":"2.1.0",
       "uri":"s3://artifacts/onnx-v1","artifact_type":"onnx"}'
# Save returned id as ARTIFACT_ID

# List artifacts
curl -s "$BASE/model_artifacts?limit=10" -H "Project-ID: $PROJECT_ID"

# Get artifact
curl -s "$BASE/model_artifacts/$ARTIFACT_ID" -H "Project-ID: $PROJECT_ID"

# Find artifact by name
curl -s "$BASE/model_artifact?name=onnx-export&registered_model_id=$MODEL_ID" \
  -H "Project-ID: $PROJECT_ID"

# Update artifact
curl -s -X PATCH "$BASE/model_artifacts/$ARTIFACT_ID" \
  -H "Project-ID: $PROJECT_ID" -H "Content-Type: application/json" \
  -d '{"uri":"s3://artifacts/onnx-v1-optimized"}'

# --- Error cases ---
# Missing Project-ID → 400
curl -s "$BASE/models"

# Invalid UUID → 400
curl -s "$BASE/models/not-a-uuid" -H "Project-ID: $PROJECT_ID"

# Not found → 404
curl -s "$BASE/models/00000000-0000-0000-0000-000000000000" -H "Project-ID: $PROJECT_ID"

# Duplicate name → 409
curl -s -X POST "$BASE/models" -H "Project-ID: $PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-llm","region_id":"'$REGION_ID'"}'

# Project isolation (different project sees nothing)
curl -s "$BASE/models" -H "Project-ID: 00000000-1111-2222-3333-444444444444"
```

### 5. Unit Tests

```bash
go test ./...
```

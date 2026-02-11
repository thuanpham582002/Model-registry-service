# Phase 1: Domain & Data Layer (Revised)

## Objective
Add serving entities (ServingEnvironment, InferenceService, ServeModel) and database schema.

## Tasks

### 1.1 Create Domain Entities

**File**: `internal/domain/serving.go`

```go
package domain

import (
	"time"
	"github.com/google/uuid"
)

// InferenceServiceState represents deployment states
type InferenceServiceState string

const (
	ISStateDeployed   InferenceServiceState = "DEPLOYED"
	ISStateUndeployed InferenceServiceState = "UNDEPLOYED"
)

// ServeModelState represents serve states
type ServeModelState string

const (
	ServeModelPending ServeModelState = "PENDING"
	ServeModelRunning ServeModelState = "RUNNING"
	ServeModelFailed  ServeModelState = "FAILED"
)

// ServingEnvironment represents a K8s namespace for model deployments
type ServingEnvironment struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProjectID   uuid.UUID `json:"project_id"`
	Name        string    `json:"name"` // Maps to K8s namespace
	Description string    `json:"description"`
	ExternalID  string    `json:"external_id,omitempty"`
}

// InferenceService tracks a deployed model on KServe
type InferenceService struct {
	ID                   uuid.UUID             `json:"id"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	ProjectID            uuid.UUID             `json:"project_id"`
	Name                 string                `json:"name"`
	ExternalID           string                `json:"external_id,omitempty"` // K8s resource UID
	ServingEnvironmentID uuid.UUID             `json:"serving_environment_id"`
	RegisteredModelID    uuid.UUID             `json:"registered_model_id"`
	ModelVersionID       *uuid.UUID            `json:"model_version_id,omitempty"`
	DesiredState         InferenceServiceState `json:"desired_state"`
	CurrentState         InferenceServiceState `json:"current_state"`
	Runtime              string                `json:"runtime"`
	URL                  string                `json:"url,omitempty"`
	LastError            string                `json:"last_error,omitempty"`
	Labels               map[string]string     `json:"labels"`
}

// ServeModel links InferenceService to the ModelVersion being served
// FIX: Added ProjectID for tenant isolation (required for multi-tenancy)
type ServeModel struct {
	ID                 uuid.UUID       `json:"id"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	ProjectID          uuid.UUID       `json:"project_id"` // FIX: Required for tenant isolation
	InferenceServiceID uuid.UUID       `json:"inference_service_id"`
	ModelVersionID     uuid.UUID       `json:"model_version_id"`
	LastKnownState     ServeModelState `json:"last_known_state"`
}
```

### 1.2 Add Repository Interfaces

**File**: `internal/domain/repository.go` (append)

```go
// --- Serving Environment ---

type ServingEnvironmentRepository interface {
	Create(ctx context.Context, env *ServingEnvironment) error
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*ServingEnvironment, error)
	GetByName(ctx context.Context, projectID uuid.UUID, name string) (*ServingEnvironment, error)
	Update(ctx context.Context, projectID uuid.UUID, env *ServingEnvironment) error
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	List(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*ServingEnvironment, int, error)
}

// --- Inference Service ---

type InferenceServiceFilter struct {
	ProjectID            uuid.UUID
	ServingEnvironmentID *uuid.UUID
	RegisteredModelID    *uuid.UUID
	ModelVersionID       *uuid.UUID
	State                string
	SortBy               string
	Order                string
	Limit                int
	Offset               int
}

type InferenceServiceRepository interface {
	Create(ctx context.Context, isvc *InferenceService) error
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*InferenceService, error)
	GetByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*InferenceService, error)
	GetByName(ctx context.Context, projectID, envID uuid.UUID, name string) (*InferenceService, error)
	Update(ctx context.Context, projectID uuid.UUID, isvc *InferenceService) error
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	List(ctx context.Context, filter InferenceServiceFilter) ([]*InferenceService, int, error)
	CountByModel(ctx context.Context, projectID, modelID uuid.UUID) (int, error)
}

// --- Serve Model ---
// FIX: All methods require projectID for tenant isolation

type ServeModelRepository interface {
	Create(ctx context.Context, sm *ServeModel) error
	GetByID(ctx context.Context, projectID, id uuid.UUID) (*ServeModel, error)
	Update(ctx context.Context, projectID uuid.UUID, sm *ServeModel) error
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	ListByInferenceService(ctx context.Context, projectID, isvcID uuid.UUID) ([]*ServeModel, error)
}
```

### 1.3 Add Domain Errors

**File**: `internal/domain/errors.go` (append)

```go
var (
	ErrServingEnvNotFound           = errors.New("serving environment not found")
	ErrServingEnvNameConflict       = errors.New("serving environment name already exists")
	ErrInferenceServiceNotFound     = errors.New("inference service not found")
	ErrInferenceServiceNameConflict = errors.New("inference service name already exists in environment")
	ErrServeModelNotFound           = errors.New("serve model not found")
	ErrModelHasActiveDeployments    = errors.New("model has active deployments and cannot be deleted")
)
```

### 1.4 Database Migration (UP)

**File**: `migrations/20260211153600_add_serving_tables.up.sql`

```sql
-- ============================================================================
-- Serving Environment: represents a K8s namespace for deployments
-- ============================================================================
CREATE TABLE IF NOT EXISTS serving_environment (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    external_id VARCHAR(255),
    CONSTRAINT uq_serving_env_project_name UNIQUE(project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_serving_env_project ON serving_environment(project_id);
CREATE INDEX IF NOT EXISTS idx_serving_env_name ON serving_environment(name);

-- ============================================================================
-- Inference Service: tracks deployed models
-- ON DELETE RESTRICT for registered_model prevents deletion if deployments exist
-- ON DELETE CASCADE for serving_environment cleans up when env is deleted
-- ============================================================================
CREATE TABLE IF NOT EXISTS inference_service (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    external_id VARCHAR(255),
    serving_environment_id UUID NOT NULL REFERENCES serving_environment(id) ON DELETE CASCADE,
    registered_model_id UUID NOT NULL REFERENCES registered_model(id) ON DELETE RESTRICT,
    model_version_id UUID REFERENCES model_version(id) ON DELETE SET NULL,
    desired_state VARCHAR(50) NOT NULL DEFAULT 'DEPLOYED',
    current_state VARCHAR(50) NOT NULL DEFAULT 'UNDEPLOYED',
    runtime VARCHAR(100) DEFAULT 'kserve',
    url TEXT,
    last_error TEXT,
    labels JSONB DEFAULT '{}',
    CONSTRAINT uq_isvc_env_name UNIQUE(project_id, serving_environment_id, name)
);

CREATE INDEX IF NOT EXISTS idx_isvc_project ON inference_service(project_id);
CREATE INDEX IF NOT EXISTS idx_isvc_env ON inference_service(serving_environment_id);
CREATE INDEX IF NOT EXISTS idx_isvc_model ON inference_service(registered_model_id);
CREATE INDEX IF NOT EXISTS idx_isvc_version ON inference_service(model_version_id);
CREATE INDEX IF NOT EXISTS idx_isvc_external ON inference_service(external_id) WHERE external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_isvc_state ON inference_service(current_state);

-- ============================================================================
-- Serve Model: tracks which version is actively served (history)
-- FIX: Added project_id for tenant isolation
-- ============================================================================
CREATE TABLE IF NOT EXISTS serve_model (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    inference_service_id UUID NOT NULL REFERENCES inference_service(id) ON DELETE CASCADE,
    model_version_id UUID NOT NULL REFERENCES model_version(id) ON DELETE RESTRICT,
    last_known_state VARCHAR(50) DEFAULT 'PENDING'
);

CREATE INDEX IF NOT EXISTS idx_serve_model_project ON serve_model(project_id);
CREATE INDEX IF NOT EXISTS idx_serve_model_isvc ON serve_model(inference_service_id);
CREATE INDEX IF NOT EXISTS idx_serve_model_version ON serve_model(model_version_id);
```

### 1.5 Database Migration (DOWN)

**File**: `migrations/20260211153600_add_serving_tables.down.sql`

```sql
-- FIX: Added down migration for safe rollback
-- Order matters: drop tables with foreign keys first

DROP TABLE IF EXISTS serve_model CASCADE;
DROP TABLE IF EXISTS inference_service CASCADE;
DROP TABLE IF EXISTS serving_environment CASCADE;
```

## Checklist

- [ ] Create `internal/domain/serving.go` with entities
- [ ] Append repository interfaces to `internal/domain/repository.go`
- [ ] Append errors to `internal/domain/errors.go`
- [ ] Create `migrations/20260211153600_add_serving_tables.up.sql`
- [ ] Create `migrations/20260211153600_add_serving_tables.down.sql`
- [ ] Run migration: `migrate -path migrations -database "$DATABASE_URL" up`
- [ ] Verify tables created: `psql -c "\dt *serving*"`
- [ ] Test rollback: `migrate -path migrations -database "$DATABASE_URL" down 1`

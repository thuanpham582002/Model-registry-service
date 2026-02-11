# Phase 1: Database & Domain

## Objective

Create database tables and domain entities for traffic configuration (canary, A/B testing).

---

## 1.1 Migrations

### Traffic Tables

**File**: `migrations/000003_add_traffic_tables.up.sql`

```sql
-- Traffic Configuration
CREATE TABLE traffic_config (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    inference_service_id UUID NOT NULL REFERENCES inference_service(id) ON DELETE CASCADE,
    strategy VARCHAR(50) NOT NULL DEFAULT 'canary',
    ai_gateway_route_name VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    CONSTRAINT uq_traffic_config_isvc UNIQUE(inference_service_id)
);

CREATE INDEX idx_traffic_config_project ON traffic_config(project_id);
CREATE INDEX idx_traffic_config_status ON traffic_config(status);

COMMENT ON TABLE traffic_config IS 'Traffic management configuration for inference services';
COMMENT ON COLUMN traffic_config.strategy IS 'canary, ab_test, shadow, blue_green';
COMMENT ON COLUMN traffic_config.ai_gateway_route_name IS 'Name of AIGatewayRoute CR in K8s';

-- Traffic Variants
CREATE TABLE traffic_variant (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    traffic_config_id UUID NOT NULL REFERENCES traffic_config(id) ON DELETE CASCADE,
    variant_name VARCHAR(100) NOT NULL,
    model_version_id UUID NOT NULL REFERENCES model_version(id),
    weight INT NOT NULL DEFAULT 0 CHECK (weight >= 0 AND weight <= 100),
    kserve_isvc_name VARCHAR(255),
    kserve_revision VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',

    CONSTRAINT uq_traffic_variant_name UNIQUE(traffic_config_id, variant_name)
);

CREATE INDEX idx_traffic_variant_config ON traffic_variant(traffic_config_id);
CREATE INDEX idx_traffic_variant_version ON traffic_variant(model_version_id);

COMMENT ON TABLE traffic_variant IS 'Traffic variants (stable, canary, shadow) within a config';
COMMENT ON COLUMN traffic_variant.variant_name IS 'stable, canary, shadow, variant_a, variant_b';
COMMENT ON COLUMN traffic_variant.weight IS 'Traffic percentage 0-100';
COMMENT ON COLUMN traffic_variant.status IS 'pending, active, promoting, draining, inactive';
```

**File**: `migrations/000003_add_traffic_tables.down.sql`

```sql
DROP TABLE IF EXISTS traffic_variant;
DROP TABLE IF EXISTS traffic_config;
```

### Virtual Model Tables

**File**: `migrations/000004_add_virtual_model_tables.up.sql`

```sql
-- Virtual Model (model name abstraction)
CREATE TABLE virtual_model (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    ai_gateway_route_name VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    CONSTRAINT uq_virtual_model_name UNIQUE(project_id, name)
);

CREATE INDEX idx_virtual_model_project ON virtual_model(project_id);
CREATE INDEX idx_virtual_model_name ON virtual_model(name);

COMMENT ON TABLE virtual_model IS 'Virtual model names that map to multiple backends';
COMMENT ON COLUMN virtual_model.name IS 'Virtual model name (e.g., claude-4-sonnet)';
COMMENT ON COLUMN virtual_model.ai_gateway_route_name IS 'Name of AIGatewayRoute CR in K8s';

-- Virtual Model Backend Mapping
CREATE TABLE virtual_model_backend (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    virtual_model_id UUID NOT NULL REFERENCES virtual_model(id) ON DELETE CASCADE,
    ai_service_backend_name VARCHAR(255) NOT NULL,
    ai_service_backend_namespace VARCHAR(255),
    model_name_override VARCHAR(255),
    weight INT NOT NULL DEFAULT 1 CHECK (weight >= 0 AND weight <= 100),
    priority INT NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    CONSTRAINT uq_virtual_model_backend UNIQUE(virtual_model_id, ai_service_backend_name, model_name_override)
);

CREATE INDEX idx_virtual_model_backend_vm ON virtual_model_backend(virtual_model_id);

COMMENT ON TABLE virtual_model_backend IS 'Backend mappings for virtual models';
COMMENT ON COLUMN virtual_model_backend.ai_service_backend_name IS 'AIServiceBackend name in K8s';
COMMENT ON COLUMN virtual_model_backend.model_name_override IS 'Override model name sent to upstream (null = use virtual name)';
COMMENT ON COLUMN virtual_model_backend.weight IS 'Traffic weight 0-100';
COMMENT ON COLUMN virtual_model_backend.priority IS '0 = primary, 1+ = fallback';
```

**File**: `migrations/000004_add_virtual_model_tables.down.sql`

```sql
DROP TABLE IF EXISTS virtual_model_backend;
DROP TABLE IF EXISTS virtual_model;
```

---

## 1.2 Domain Entities

**File**: `internal/core/domain/traffic.go`

```go
package domain

import (
	"time"
	"github.com/google/uuid"
)

// TrafficStrategy defines the traffic management strategy
type TrafficStrategy string

const (
	TrafficStrategyCanary    TrafficStrategy = "canary"
	TrafficStrategyABTest    TrafficStrategy = "ab_test"
	TrafficStrategyShadow    TrafficStrategy = "shadow"
	TrafficStrategyBlueGreen TrafficStrategy = "blue_green"
)

// VariantStatus defines the status of a traffic variant
type VariantStatus string

const (
	VariantStatusPending   VariantStatus = "pending"
	VariantStatusActive    VariantStatus = "active"
	VariantStatusPromoting VariantStatus = "promoting"
	VariantStatusDraining  VariantStatus = "draining"
	VariantStatusInactive  VariantStatus = "inactive"
)

// TrafficConfig represents traffic management configuration
type TrafficConfig struct {
	ID                  uuid.UUID       `json:"id"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	ProjectID           uuid.UUID       `json:"project_id"`
	InferenceServiceID  uuid.UUID       `json:"inference_service_id"`
	Strategy            TrafficStrategy `json:"strategy"`
	AIGatewayRouteName  string          `json:"ai_gateway_route_name"`
	Status              string          `json:"status"`

	// Computed
	Variants             []*TrafficVariant `json:"variants,omitempty"`
	InferenceServiceName string           `json:"inference_service_name,omitempty"`
}

// GetVariant returns variant by name
func (c *TrafficConfig) GetVariant(name string) *TrafficVariant {
	for _, v := range c.Variants {
		if v.VariantName == name {
			return v
		}
	}
	return nil
}

// GetActiveVariants returns all active variants
func (c *TrafficConfig) GetActiveVariants() []*TrafficVariant {
	var active []*TrafficVariant
	for _, v := range c.Variants {
		if v.Status == VariantStatusActive && v.Weight > 0 {
			active = append(active, v)
		}
	}
	return active
}

// TotalWeight returns sum of all variant weights
func (c *TrafficConfig) TotalWeight() int {
	total := 0
	for _, v := range c.Variants {
		if v.Status == VariantStatusActive {
			total += v.Weight
		}
	}
	return total
}

// ValidateWeights checks if total weights <= 100
func (c *TrafficConfig) ValidateWeights() error {
	if c.TotalWeight() > 100 {
		return ErrWeightSumExceeds100
	}
	return nil
}

// HasVariant checks if variant exists
func (c *TrafficConfig) HasVariant(name string) bool {
	return c.GetVariant(name) != nil
}

// NewTrafficConfig creates a new TrafficConfig
func NewTrafficConfig(projectID, isvcID uuid.UUID, strategy TrafficStrategy) (*TrafficConfig, error) {
	if projectID == uuid.Nil {
		return nil, ErrMissingProjectID
	}
	if isvcID == uuid.Nil {
		return nil, ErrInvalidInferenceServiceID
	}

	now := time.Now()
	return &TrafficConfig{
		ID:                 uuid.New(),
		CreatedAt:          now,
		UpdatedAt:          now,
		ProjectID:          projectID,
		InferenceServiceID: isvcID,
		Strategy:           strategy,
		Status:             "active",
	}, nil
}

// TrafficVariant represents a traffic variant (stable, canary, etc.)
type TrafficVariant struct {
	ID              uuid.UUID     `json:"id"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	TrafficConfigID uuid.UUID     `json:"traffic_config_id"`
	VariantName     string        `json:"variant_name"`
	ModelVersionID  uuid.UUID     `json:"model_version_id"`
	Weight          int           `json:"weight"`
	KServeISVCName  string        `json:"kserve_isvc_name"`
	KServeRevision  string        `json:"kserve_revision"`
	Status          VariantStatus `json:"status"`

	// Computed
	ModelVersionName string `json:"model_version_name,omitempty"`
}

// NewTrafficVariant creates a new TrafficVariant
func NewTrafficVariant(configID, versionID uuid.UUID, name string, weight int) (*TrafficVariant, error) {
	if name == "" {
		return nil, ErrInvalidVariantName
	}
	if weight < 0 || weight > 100 {
		return nil, ErrInvalidTrafficWeight
	}

	now := time.Now()
	return &TrafficVariant{
		ID:              uuid.New(),
		CreatedAt:       now,
		UpdatedAt:       now,
		TrafficConfigID: configID,
		VariantName:     name,
		ModelVersionID:  versionID,
		Weight:          weight,
		Status:          VariantStatusPending,
	}, nil
}

// SetWeight updates the weight
func (v *TrafficVariant) SetWeight(weight int) error {
	if weight < 0 || weight > 100 {
		return ErrInvalidTrafficWeight
	}
	v.Weight = weight
	v.UpdatedAt = time.Now()
	return nil
}

// Activate marks the variant as active
func (v *TrafficVariant) Activate() {
	v.Status = VariantStatusActive
	v.UpdatedAt = time.Now()
}

// Deactivate marks the variant as inactive
func (v *TrafficVariant) Deactivate() {
	v.Status = VariantStatusInactive
	v.Weight = 0
	v.UpdatedAt = time.Now()
}
```

**File**: `internal/core/domain/virtual_model.go`

```go
package domain

import (
	"time"
	"github.com/google/uuid"
)

// VirtualModel represents a virtual model name that maps to multiple backends
type VirtualModel struct {
	ID                 uuid.UUID              `json:"id"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	ProjectID          uuid.UUID              `json:"project_id"`
	Name               string                 `json:"name"`
	Description        string                 `json:"description,omitempty"`
	AIGatewayRouteName string                 `json:"ai_gateway_route_name,omitempty"`
	Status             string                 `json:"status"`

	// Computed
	Backends           []*VirtualModelBackend `json:"backends,omitempty"`
}

// NewVirtualModel creates a new VirtualModel
func NewVirtualModel(projectID uuid.UUID, name string) (*VirtualModel, error) {
	if projectID == uuid.Nil {
		return nil, ErrMissingProjectID
	}
	if name == "" {
		return nil, ErrInvalidVirtualModelName
	}

	now := time.Now()
	return &VirtualModel{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		ProjectID: projectID,
		Name:      name,
		Status:    "active",
	}, nil
}

// GetBackend returns backend by AI service backend name
func (vm *VirtualModel) GetBackend(backendName string) *VirtualModelBackend {
	for _, b := range vm.Backends {
		if b.AIServiceBackendName == backendName {
			return b
		}
	}
	return nil
}

// GetActiveBackends returns all active backends sorted by priority
func (vm *VirtualModel) GetActiveBackends() []*VirtualModelBackend {
	var active []*VirtualModelBackend
	for _, b := range vm.Backends {
		if b.Status == "active" {
			active = append(active, b)
		}
	}
	return active
}

// TotalWeight returns sum of all backend weights
func (vm *VirtualModel) TotalWeight() int {
	total := 0
	for _, b := range vm.Backends {
		if b.Status == "active" {
			total += b.Weight
		}
	}
	return total
}

// VirtualModelBackend represents a backend mapping for a virtual model
type VirtualModelBackend struct {
	ID                      uuid.UUID `json:"id"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	VirtualModelID          uuid.UUID `json:"virtual_model_id"`
	AIServiceBackendName    string    `json:"ai_service_backend_name"`
	AIServiceBackendNamespace string  `json:"ai_service_backend_namespace,omitempty"`
	ModelNameOverride       *string   `json:"model_name_override,omitempty"` // nil = use virtual model name
	Weight                  int       `json:"weight"`
	Priority                int       `json:"priority"` // 0 = primary, 1+ = fallback
	Status                  string    `json:"status"`
}

// NewVirtualModelBackend creates a new VirtualModelBackend
func NewVirtualModelBackend(vmID uuid.UUID, backendName string, modelOverride *string, weight, priority int) (*VirtualModelBackend, error) {
	if backendName == "" {
		return nil, ErrInvalidBackendName
	}
	if weight < 0 || weight > 100 {
		return nil, ErrInvalidTrafficWeight
	}
	if priority < 0 {
		return nil, ErrInvalidPriority
	}

	now := time.Now()
	return &VirtualModelBackend{
		ID:                   uuid.New(),
		CreatedAt:            now,
		UpdatedAt:            now,
		VirtualModelID:       vmID,
		AIServiceBackendName: backendName,
		ModelNameOverride:    modelOverride,
		Weight:               weight,
		Priority:             priority,
		Status:               "active",
	}, nil
}

// GetEffectiveModelName returns the model name to use (override or virtual model name)
func (b *VirtualModelBackend) GetEffectiveModelName(virtualModelName string) string {
	if b.ModelNameOverride != nil && *b.ModelNameOverride != "" {
		return *b.ModelNameOverride
	}
	return virtualModelName
}
```

---

## 1.3 Domain Errors

**File**: `internal/core/domain/errors.go` (append)

```go
// Traffic errors
var (
	ErrTrafficConfigNotFound  = errors.New("traffic config not found")
	ErrTrafficVariantNotFound = errors.New("traffic variant not found")
	ErrInvalidVariantName     = errors.New("invalid variant name")
	ErrInvalidTrafficWeight   = errors.New("traffic weight must be between 0 and 100")
	ErrWeightSumExceeds100    = errors.New("total variant weights cannot exceed 100")
	ErrVariantAlreadyExists   = errors.New("variant already exists")
	ErrCanaryAlreadyExists    = errors.New("canary variant already exists")
	ErrNoStableVariant        = errors.New("no stable variant configured")
	ErrCannotPromoteInactive  = errors.New("cannot promote inactive variant")
	ErrCannotPromoteStable    = errors.New("cannot promote stable variant to itself")
	ErrCannotDeleteStable     = errors.New("cannot delete stable variant")
)

// Virtual Model errors
var (
	ErrVirtualModelNotFound    = errors.New("virtual model not found")
	ErrInvalidVirtualModelName = errors.New("invalid virtual model name")
	ErrVirtualModelExists      = errors.New("virtual model already exists")
	ErrBackendNotFound         = errors.New("backend not found")
	ErrInvalidBackendName      = errors.New("invalid backend name")
	ErrInvalidPriority         = errors.New("priority must be >= 0")
	ErrBackendAlreadyExists    = errors.New("backend already exists for this virtual model")
)

// Metrics errors
var (
	ErrMetricNotFound        = errors.New("metric not found")
	ErrInvalidTimeRange      = errors.New("invalid time range")
	ErrPrometheusQueryFailed = errors.New("prometheus query failed")
)
```

---

## Checklist

- [ ] Create migration 000003 (traffic tables)
- [ ] Create migration 000004 (virtual model tables)
- [ ] Create `internal/core/domain/traffic.go`
- [ ] Create `internal/core/domain/virtual_model.go`
- [ ] Update `internal/core/domain/errors.go`
- [ ] Run migrations locally
- [ ] Verify table creation

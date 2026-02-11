-- Virtual Model Tables
-- Enables model name virtualization for multi-provider routing

-- ============================================================================
-- Virtual Model
-- ============================================================================
-- Represents a virtual model name that maps to multiple backends

CREATE TABLE IF NOT EXISTS virtual_model (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    ai_gateway_route_name VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    CONSTRAINT uq_virtual_model_project_name UNIQUE (project_id, name)
);

CREATE INDEX idx_virtual_model_project_id ON virtual_model(project_id);
CREATE INDEX idx_virtual_model_name ON virtual_model(name);
CREATE INDEX idx_virtual_model_status ON virtual_model(status);
CREATE INDEX idx_virtual_model_created_at ON virtual_model(created_at DESC);

COMMENT ON TABLE virtual_model IS 'Virtual model names that map to multiple AI backends';
COMMENT ON COLUMN virtual_model.name IS 'Virtual model name exposed to clients (e.g., claude-4-sonnet)';
COMMENT ON COLUMN virtual_model.ai_gateway_route_name IS 'Name of AIGatewayRoute CR in K8s';
COMMENT ON COLUMN virtual_model.status IS 'Status: active, inactive, archived';

-- ============================================================================
-- Virtual Model Backend
-- ============================================================================
-- Maps a virtual model to AI service backends with model name override

CREATE TABLE IF NOT EXISTS virtual_model_backend (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    virtual_model_id UUID NOT NULL,
    ai_service_backend_name VARCHAR(255) NOT NULL,
    ai_service_backend_namespace VARCHAR(255) NOT NULL DEFAULT '',
    model_name_override VARCHAR(255), -- NULL means use virtual model name
    weight INT NOT NULL DEFAULT 1,
    priority INT NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    CONSTRAINT fk_virtual_model_backend_vm
        FOREIGN KEY (virtual_model_id)
        REFERENCES virtual_model(id)
        ON DELETE CASCADE,
    CONSTRAINT uq_virtual_model_backend UNIQUE (virtual_model_id, ai_service_backend_name, COALESCE(model_name_override, '')),
    CONSTRAINT chk_virtual_model_backend_weight CHECK (weight >= 0 AND weight <= 100),
    CONSTRAINT chk_virtual_model_backend_priority CHECK (priority >= 0)
);

CREATE INDEX idx_virtual_model_backend_vm_id ON virtual_model_backend(virtual_model_id);
CREATE INDEX idx_virtual_model_backend_status ON virtual_model_backend(status);
CREATE INDEX idx_virtual_model_backend_priority ON virtual_model_backend(priority);

COMMENT ON TABLE virtual_model_backend IS 'Backend mappings for virtual models';
COMMENT ON COLUMN virtual_model_backend.ai_service_backend_name IS 'AIServiceBackend CR name in K8s';
COMMENT ON COLUMN virtual_model_backend.ai_service_backend_namespace IS 'AIServiceBackend namespace (empty = same as route)';
COMMENT ON COLUMN virtual_model_backend.model_name_override IS 'Model name sent to upstream (NULL = use virtual model name)';
COMMENT ON COLUMN virtual_model_backend.weight IS 'Traffic weight 0-100 for load balancing';
COMMENT ON COLUMN virtual_model_backend.priority IS 'Priority level: 0 = primary, 1+ = fallback tiers';
COMMENT ON COLUMN virtual_model_backend.status IS 'Status: active, inactive, draining';

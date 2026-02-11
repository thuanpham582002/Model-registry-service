-- Traffic Management Tables
-- Enables canary deployments, A/B testing, and traffic splitting

-- ============================================================================
-- Traffic Configuration
-- ============================================================================
-- Represents traffic management configuration for an inference service

CREATE TABLE IF NOT EXISTS traffic_config (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    inference_service_id UUID NOT NULL,
    strategy VARCHAR(50) NOT NULL DEFAULT 'canary',
    ai_gateway_route_name VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'active',

    CONSTRAINT fk_traffic_config_inference_service
        FOREIGN KEY (inference_service_id)
        REFERENCES inference_service(id)
        ON DELETE CASCADE,
    CONSTRAINT uq_traffic_config_isvc UNIQUE (inference_service_id)
);

CREATE INDEX idx_traffic_config_project_id ON traffic_config(project_id);
CREATE INDEX idx_traffic_config_status ON traffic_config(status);
CREATE INDEX idx_traffic_config_created_at ON traffic_config(created_at DESC);

COMMENT ON TABLE traffic_config IS 'Traffic management configuration for inference services';
COMMENT ON COLUMN traffic_config.strategy IS 'Traffic strategy: canary, ab_test, shadow, blue_green';
COMMENT ON COLUMN traffic_config.ai_gateway_route_name IS 'Name of AIGatewayRoute CR in K8s';
COMMENT ON COLUMN traffic_config.status IS 'Config status: active, paused, archived';

-- ============================================================================
-- Traffic Variant
-- ============================================================================
-- Represents a traffic variant (stable, canary, variant_a, etc.)

CREATE TABLE IF NOT EXISTS traffic_variant (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    traffic_config_id UUID NOT NULL,
    variant_name VARCHAR(100) NOT NULL,
    model_version_id UUID NOT NULL,
    weight INT NOT NULL DEFAULT 0,
    kserve_isvc_name VARCHAR(255) NOT NULL DEFAULT '',
    kserve_revision VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',

    CONSTRAINT fk_traffic_variant_config
        FOREIGN KEY (traffic_config_id)
        REFERENCES traffic_config(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_traffic_variant_model_version
        FOREIGN KEY (model_version_id)
        REFERENCES model_version(id)
        ON DELETE RESTRICT,
    CONSTRAINT uq_traffic_variant_name UNIQUE (traffic_config_id, variant_name),
    CONSTRAINT chk_traffic_variant_weight CHECK (weight >= 0 AND weight <= 100)
);

CREATE INDEX idx_traffic_variant_config_id ON traffic_variant(traffic_config_id);
CREATE INDEX idx_traffic_variant_model_version_id ON traffic_variant(model_version_id);
CREATE INDEX idx_traffic_variant_status ON traffic_variant(status);

COMMENT ON TABLE traffic_variant IS 'Traffic variants within a traffic config (stable, canary, variant_a, etc.)';
COMMENT ON COLUMN traffic_variant.variant_name IS 'Variant identifier: stable, canary, shadow, variant_a, variant_b, etc.';
COMMENT ON COLUMN traffic_variant.weight IS 'Traffic percentage 0-100';
COMMENT ON COLUMN traffic_variant.kserve_isvc_name IS 'Name of KServe InferenceService for this variant';
COMMENT ON COLUMN traffic_variant.kserve_revision IS 'KServe revision name if using revisions';
COMMENT ON COLUMN traffic_variant.status IS 'Variant status: pending, active, promoting, draining, inactive';

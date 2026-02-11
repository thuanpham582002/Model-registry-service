-- Serving Tables for KServe Integration
-- Enables click-to-deploy functionality

-- ============================================================================
-- Serving Environment
-- ============================================================================
-- Represents a deployment namespace/environment (maps to K8s namespace)

CREATE TABLE IF NOT EXISTS serving_environment (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    external_id VARCHAR(255) NOT NULL DEFAULT '', -- K8s namespace if different from name

    CONSTRAINT uq_serving_environment_project_name UNIQUE (project_id, name)
);

CREATE INDEX idx_serving_environment_project_id ON serving_environment(project_id);
CREATE INDEX idx_serving_environment_created_at ON serving_environment(created_at DESC);

COMMENT ON TABLE serving_environment IS 'Deployment environments for model serving (maps to K8s namespaces)';
COMMENT ON COLUMN serving_environment.external_id IS 'K8s namespace name if different from env name';

-- ============================================================================
-- Inference Service
-- ============================================================================
-- Represents a deployed model endpoint (maps to KServe InferenceService CR)

CREATE TABLE IF NOT EXISTS inference_service (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    external_id VARCHAR(255) NOT NULL DEFAULT '', -- K8s resource UID
    serving_environment_id UUID NOT NULL,
    registered_model_id UUID NOT NULL,
    model_version_id UUID, -- nullable: can deploy default version
    desired_state VARCHAR(50) NOT NULL DEFAULT 'DEPLOYED',
    current_state VARCHAR(50) NOT NULL DEFAULT 'UNDEPLOYED',
    runtime VARCHAR(50) NOT NULL DEFAULT 'kserve',
    url VARCHAR(500) NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    labels JSONB NOT NULL DEFAULT '{}',

    CONSTRAINT fk_inference_service_serving_environment
        FOREIGN KEY (serving_environment_id)
        REFERENCES serving_environment(id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_inference_service_registered_model
        FOREIGN KEY (registered_model_id)
        REFERENCES registered_model(id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_inference_service_model_version
        FOREIGN KEY (model_version_id)
        REFERENCES model_version(id)
        ON DELETE SET NULL,
    CONSTRAINT uq_inference_service_env_name UNIQUE (serving_environment_id, name)
);

CREATE INDEX idx_inference_service_project_id ON inference_service(project_id);
CREATE INDEX idx_inference_service_serving_environment_id ON inference_service(serving_environment_id);
CREATE INDEX idx_inference_service_registered_model_id ON inference_service(registered_model_id);
CREATE INDEX idx_inference_service_model_version_id ON inference_service(model_version_id);
CREATE INDEX idx_inference_service_desired_state ON inference_service(desired_state);
CREATE INDEX idx_inference_service_current_state ON inference_service(current_state);
CREATE INDEX idx_inference_service_created_at ON inference_service(created_at DESC);
CREATE INDEX idx_inference_service_external_id ON inference_service(external_id) WHERE external_id != '';

COMMENT ON TABLE inference_service IS 'Deployed model endpoints (maps to KServe InferenceService CRs)';
COMMENT ON COLUMN inference_service.external_id IS 'K8s resource UID for reconciliation';
COMMENT ON COLUMN inference_service.desired_state IS 'Target state: DEPLOYED or UNDEPLOYED';
COMMENT ON COLUMN inference_service.current_state IS 'Actual state from K8s cluster';
COMMENT ON COLUMN inference_service.runtime IS 'Serving runtime: kserve, triton, etc.';
COMMENT ON COLUMN inference_service.url IS 'Inference endpoint URL when deployed';

-- ============================================================================
-- Serve Model
-- ============================================================================
-- Links InferenceService to ModelVersion (for multi-model endpoints)

CREATE TABLE IF NOT EXISTS serve_model (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    inference_service_id UUID NOT NULL,
    model_version_id UUID NOT NULL,
    last_known_state VARCHAR(50) NOT NULL DEFAULT 'PENDING',

    CONSTRAINT fk_serve_model_inference_service
        FOREIGN KEY (inference_service_id)
        REFERENCES inference_service(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_serve_model_model_version
        FOREIGN KEY (model_version_id)
        REFERENCES model_version(id)
        ON DELETE RESTRICT,
    CONSTRAINT uq_serve_model_isvc_version UNIQUE (inference_service_id, model_version_id)
);

CREATE INDEX idx_serve_model_project_id ON serve_model(project_id);
CREATE INDEX idx_serve_model_inference_service_id ON serve_model(inference_service_id);
CREATE INDEX idx_serve_model_model_version_id ON serve_model(model_version_id);
CREATE INDEX idx_serve_model_state ON serve_model(last_known_state);

COMMENT ON TABLE serve_model IS 'Links inference services to model versions';
COMMENT ON COLUMN serve_model.last_known_state IS 'State: PENDING, RUNNING, or FAILED';

-- Model Registry Service - Initial Schema
-- SYNC pattern: Go MR stores owner_email, region_name locally (no JOINs to CMP)

-- Core tables
CREATE TABLE IF NOT EXISTS registered_model (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    owner_id UUID,
    owner_email VARCHAR(255) NOT NULL DEFAULT '',
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    region_id UUID NOT NULL,
    region_name VARCHAR(100) NOT NULL DEFAULT '',
    model_type VARCHAR(50) NOT NULL DEFAULT 'CUSTOMTRAIN',
    model_size BIGINT NOT NULL DEFAULT 0,
    state VARCHAR(50) NOT NULL DEFAULT 'LIVE',
    deployment_status VARCHAR(50) NOT NULL DEFAULT 'UNDEPLOYED',
    tags JSONB NOT NULL DEFAULT '{"frameworks":[],"architectures":[],"tasks":[],"subjects":[]}',
    labels JSONB NOT NULL DEFAULT '{}',
    parent_model_id UUID,

    CONSTRAINT uq_registered_model_project_name UNIQUE (project_id, name),
    CONSTRAINT fk_registered_model_parent
        FOREIGN KEY (parent_model_id)
        REFERENCES registered_model(id)
        ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS model_version (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    registered_model_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT false,
    state VARCHAR(50) NOT NULL DEFAULT 'LIVE',
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_by_id UUID,
    updated_by_id UUID,
    created_by_email VARCHAR(255) NOT NULL DEFAULT '',
    updated_by_email VARCHAR(255) NOT NULL DEFAULT '',
    artifact_type VARCHAR(50) NOT NULL DEFAULT 'model-artifact',
    model_framework VARCHAR(100) NOT NULL DEFAULT '',
    model_framework_version VARCHAR(50) NOT NULL DEFAULT '',
    container_image VARCHAR(500) NOT NULL DEFAULT '',
    model_catalog_name VARCHAR(200) NOT NULL DEFAULT '',
    uri VARCHAR(500) NOT NULL DEFAULT '',
    access_key VARCHAR(500) NOT NULL DEFAULT '',
    secret_key VARCHAR(500) NOT NULL DEFAULT '',
    labels JSONB NOT NULL DEFAULT '{}',
    prebuilt_container_id UUID,

    CONSTRAINT fk_model_version_registered_model
        FOREIGN KEY (registered_model_id)
        REFERENCES registered_model(id)
        ON DELETE CASCADE,
    CONSTRAINT uq_model_version_model_name
        UNIQUE (registered_model_id, name)
);

-- Indexes for registered_model
CREATE INDEX idx_registered_model_project_id ON registered_model(project_id);
CREATE INDEX idx_registered_model_region_id ON registered_model(region_id);
CREATE INDEX idx_registered_model_owner_id ON registered_model(owner_id);
CREATE INDEX idx_registered_model_state ON registered_model(state);
CREATE INDEX idx_registered_model_created_at ON registered_model(created_at DESC);
CREATE INDEX idx_registered_model_name ON registered_model(name);

-- Indexes for model_version
CREATE INDEX idx_model_version_registered_model_id ON model_version(registered_model_id);
CREATE INDEX idx_model_version_created_by_id ON model_version(created_by_id);
CREATE INDEX idx_model_version_updated_by_id ON model_version(updated_by_id);
CREATE INDEX idx_model_version_state ON model_version(state);
CREATE INDEX idx_model_version_status ON model_version(status);
CREATE INDEX idx_model_version_created_at ON model_version(created_at DESC);
CREATE INDEX idx_model_version_is_default ON model_version(registered_model_id, is_default) WHERE is_default = true;

-- Comments
COMMENT ON TABLE registered_model IS 'Registered ML models scoped to projects';
COMMENT ON TABLE model_version IS 'Versions of registered models with artifact metadata';
COMMENT ON COLUMN registered_model.owner_email IS 'Denormalized owner email (synced from CMP)';
COMMENT ON COLUMN registered_model.region_name IS 'Denormalized region name (synced from CMP)';
COMMENT ON COLUMN model_version.created_by_email IS 'Denormalized creator email (synced from CMP)';
COMMENT ON COLUMN model_version.updated_by_email IS 'Denormalized updater email (synced from CMP)';

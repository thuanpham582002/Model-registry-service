-- Stub tables for LEFT JOINs (normally managed by CMP Django)
CREATE TABLE IF NOT EXISTS tenant_user (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS organization_region (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL DEFAULT ''
);

-- Core model registry tables
CREATE TABLE IF NOT EXISTS model_registry_registered_model (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    project_id UUID NOT NULL,
    owner_id UUID,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    region_id UUID NOT NULL,
    model_type VARCHAR(50) NOT NULL DEFAULT 'CUSTOMTRAIN',
    model_size BIGINT NOT NULL DEFAULT 0,
    state VARCHAR(50) NOT NULL DEFAULT 'LIVE',
    deployment_status VARCHAR(50) NOT NULL DEFAULT 'UNDEPLOYED',
    tags JSONB NOT NULL DEFAULT '{"frameworks":[],"architectures":[],"tasks":[],"subjects":[]}',
    labels JSONB NOT NULL DEFAULT '{}',
    parent_model_id UUID,
    UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS model_registry_model_version (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    registered_model_id UUID NOT NULL REFERENCES model_registry_registered_model(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT false,
    state VARCHAR(50) NOT NULL DEFAULT 'LIVE',
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_by_id UUID,
    updated_by_id UUID,
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
    UNIQUE (registered_model_id, name)
);

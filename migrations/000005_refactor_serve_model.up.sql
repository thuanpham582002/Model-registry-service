-- Refactor: Use serve_model for multi-model serving
-- Remove model_version_id from inference_service (use serve_model junction table instead)

-- Step 1: Migrate existing data to serve_model
INSERT INTO serve_model (id, created_at, updated_at, project_id, inference_service_id, model_version_id, last_known_state)
SELECT
    gen_random_uuid(),
    NOW(),
    NOW(),
    project_id,
    id,
    model_version_id,
    CASE current_state
        WHEN 'DEPLOYED' THEN 'RUNNING'
        WHEN 'UNDEPLOYED' THEN 'PENDING'
        ELSE 'PENDING'
    END
FROM inference_service
WHERE model_version_id IS NOT NULL
ON CONFLICT (inference_service_id, model_version_id) DO NOTHING;

-- Step 2: Drop the foreign key constraint
ALTER TABLE inference_service DROP CONSTRAINT IF EXISTS fk_inference_service_model_version;

-- Step 3: Drop the index
DROP INDEX IF EXISTS idx_inference_service_model_version_id;

-- Step 4: Remove the column
ALTER TABLE inference_service DROP COLUMN IF EXISTS model_version_id;

-- Step 5: Update comments
COMMENT ON TABLE serve_model IS 'Junction table linking inference services to model versions (supports multi-model endpoints)';

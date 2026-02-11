-- Rollback: Add model_version_id back to inference_service

-- Step 1: Add the column back
ALTER TABLE inference_service ADD COLUMN IF NOT EXISTS model_version_id UUID;

-- Step 2: Restore data from serve_model (pick first/oldest entry per inference_service)
UPDATE inference_service AS isvc
SET model_version_id = sm.model_version_id
FROM (
    SELECT DISTINCT ON (inference_service_id)
        inference_service_id,
        model_version_id
    FROM serve_model
    ORDER BY inference_service_id, created_at ASC
) AS sm
WHERE isvc.id = sm.inference_service_id;

-- Step 3: Recreate the foreign key
ALTER TABLE inference_service
ADD CONSTRAINT fk_inference_service_model_version
    FOREIGN KEY (model_version_id)
    REFERENCES model_version(id)
    ON DELETE SET NULL;

-- Step 4: Recreate the index
CREATE INDEX idx_inference_service_model_version_id ON inference_service(model_version_id);

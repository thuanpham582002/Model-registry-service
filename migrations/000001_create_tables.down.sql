-- Rollback: Drop all model registry tables

DROP TABLE IF EXISTS model_version CASCADE;
DROP TABLE IF EXISTS registered_model CASCADE;

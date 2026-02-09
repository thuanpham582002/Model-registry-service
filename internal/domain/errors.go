package domain

import "errors"

var (
	ErrModelNotFound       = errors.New("registered model not found")
	ErrModelNameConflict   = errors.New("model with this name already exists in the project")
	ErrVersionNotFound     = errors.New("model version not found")
	ErrVersionNameConflict = errors.New("version with this name already exists for this model")
	ErrArtifactNotFound    = errors.New("model artifact not found")
	ErrInvalidModelName    = errors.New("model name is required")
	ErrMissingProjectID    = errors.New("project ID is required (X-Project-ID header)")
	ErrMissingRegionID     = errors.New("region ID is required")
	ErrCannotDeleteModel   = errors.New("cannot delete model: must be archived with no READY versions")
)

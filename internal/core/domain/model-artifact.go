package domain

// ModelArtifact is a virtual entity projected from ModelVersion.
// Artifact API endpoints query the model_version table and project artifact-specific columns.
// This struct is used for artifact-specific responses.
type ModelArtifact struct {
	ModelVersion
}

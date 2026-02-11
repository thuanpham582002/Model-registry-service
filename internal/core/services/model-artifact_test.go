package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"
	"model-registry-service/internal/testutil"
)

func TestModelArtifactService_Create(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	svc := NewModelArtifactService(versionRepo, modelRepo)

	projectID := uuid.New()
	modelID := uuid.New()
	parentModel := &domain.RegisteredModel{ID: modelID}
	returnedVersion := &domain.ModelVersion{
		ID: uuid.New(), RegisteredModelID: modelID, Name: "artifact-v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		State: domain.ModelStateLive, Status: domain.VersionStatusPending,
		ArtifactType: domain.ArtifactTypeModel, Labels: map[string]string{},
	}

	modelRepo.On("GetByID", mock.Anything, projectID, modelID).Return(parentModel, nil)
	versionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returnedVersion, nil)

	result, err := svc.Create(context.Background(), projectID, modelID, "artifact-v1", "desc",
		"model-artifact", "tf", "2.0", "", "s3://b/m", "", "", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "artifact-v1", result.Name)
}

func TestModelArtifactService_Get(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	svc := NewModelArtifactService(versionRepo, modelRepo)

	projectID := uuid.New()
	id := uuid.New()
	expected := &domain.ModelVersion{ID: id, Name: "v1"}
	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(expected, nil)

	result, err := svc.Get(context.Background(), projectID, id)
	assert.NoError(t, err)
	assert.Equal(t, "v1", result.Name)
}

func TestModelArtifactService_List(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	svc := NewModelArtifactService(versionRepo, modelRepo)

	projectID := uuid.New()
	filter := ports.VersionListFilter{Limit: 10}
	versions := []*domain.ModelVersion{{ID: uuid.New()}}
	versionRepo.On("List", mock.Anything, mock.AnythingOfType("ports.VersionListFilter")).Return(versions, 1, nil)

	result, total, err := svc.List(context.Background(), projectID, filter)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
}

func TestModelArtifactService_Update(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	svc := NewModelArtifactService(versionRepo, modelRepo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.ModelVersion{
		ID: id, ArtifactType: domain.ArtifactTypeModel,
		Labels: map[string]string{},
	}

	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	versionRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)

	result, err := svc.Update(context.Background(), projectID, id, map[string]interface{}{
		"artifact_type": "doc-artifact",
	})
	assert.NoError(t, err)
	assert.Equal(t, domain.ArtifactTypeDoc, result.ArtifactType)
}

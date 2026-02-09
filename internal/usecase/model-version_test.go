package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"model-registry-service/internal/domain"
	"model-registry-service/internal/testutil"
)

func TestModelVersionUseCase_Create(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	uc := NewModelVersionUseCase(versionRepo, modelRepo)

	projectID := uuid.New()
	modelID := uuid.New()
	versionID := uuid.New()

	parentModel := &domain.RegisteredModel{ID: modelID, Name: "m1"}
	returnedVersion := &domain.ModelVersion{
		ID: versionID, RegisteredModelID: modelID, Name: "v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		State: domain.ModelStateLive, Status: domain.VersionStatusPending,
		ArtifactType: domain.ArtifactTypeModel, Labels: map[string]string{},
	}

	modelRepo.On("GetByID", mock.Anything, projectID, modelID).Return(parentModel, nil)
	versionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returnedVersion, nil)

	version, err := uc.Create(context.Background(), projectID, modelID, "v1", "desc", false,
		"model-artifact", "pytorch", "2.0", "", "", "s3://bucket/model", "", "",
		nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "v1", version.Name)
}

func TestModelVersionUseCase_Create_ModelNotFound(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	uc := NewModelVersionUseCase(versionRepo, modelRepo)

	projectID := uuid.New()
	modelID := uuid.New()
	modelRepo.On("GetByID", mock.Anything, projectID, modelID).Return(nil, domain.ErrModelNotFound)

	_, err := uc.Create(context.Background(), projectID, modelID, "v1", "desc", false,
		"", "pytorch", "2.0", "", "", "s3://bucket/model", "", "",
		nil, nil, nil)
	assert.ErrorIs(t, err, domain.ErrModelNotFound)
}

func TestModelVersionUseCase_Get(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	uc := NewModelVersionUseCase(versionRepo, modelRepo)

	projectID := uuid.New()
	id := uuid.New()
	expected := &domain.ModelVersion{ID: id, Name: "v1"}
	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(expected, nil)

	version, err := uc.Get(context.Background(), projectID, id)
	assert.NoError(t, err)
	assert.Equal(t, "v1", version.Name)
}

func TestModelVersionUseCase_ListByModel(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	uc := NewModelVersionUseCase(versionRepo, modelRepo)

	projectID := uuid.New()
	modelID := uuid.New()
	filter := domain.VersionListFilter{Limit: 10}
	versions := []*domain.ModelVersion{{ID: uuid.New(), Name: "v1"}}

	versionRepo.On("ListByModel", mock.Anything, modelID, mock.AnythingOfType("domain.VersionListFilter")).Return(versions, 1, nil)

	result, total, err := uc.ListByModel(context.Background(), projectID, modelID, filter)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
}

func TestModelVersionUseCase_Update(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	uc := NewModelVersionUseCase(versionRepo, modelRepo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.ModelVersion{
		ID: id, Name: "v1", State: domain.ModelStateLive,
		Status: domain.VersionStatusPending, Labels: map[string]string{},
	}

	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	versionRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)

	updated, err := uc.Update(context.Background(), projectID, id, map[string]interface{}{
		"status": "READY",
	})
	assert.NoError(t, err)
	assert.Equal(t, domain.VersionStatusReady, updated.Status)
}

func TestModelVersionUseCase_Find(t *testing.T) {
	versionRepo := new(testutil.MockModelVersionRepo)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	uc := NewModelVersionUseCase(versionRepo, modelRepo)

	projectID := uuid.New()
	expected := &domain.ModelVersion{ID: uuid.New(), Name: "v1"}
	versionRepo.On("FindByParams", mock.Anything, projectID, "v1", "", (*uuid.UUID)(nil)).Return(expected, nil)

	version, err := uc.Find(context.Background(), projectID, "v1", "", nil)
	assert.NoError(t, err)
	assert.Equal(t, "v1", version.Name)
}

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

func TestRegisteredModelUseCase_Create(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	projectID := uuid.New()
	regionID := uuid.New()
	modelID := uuid.New()

	returnedModel := &domain.RegisteredModel{
		ID:               modelID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProjectID:        projectID,
		Name:             "test-model",
		Slug:             "test-model",
		RegionID:         regionID,
		ModelType:        domain.ModelTypeCustomTrain,
		State:            domain.ModelStateLive,
		DeploymentStatus: domain.DeploymentStatusUndeployed,
		Tags:             domain.Tags{Frameworks: []string{}, Architectures: []string{}, Tasks: []string{}, Subjects: []string{}},
		Labels:           map[string]string{},
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)
	repo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returnedModel, nil)

	model, err := uc.Create(context.Background(), projectID, nil, "test-model", "desc", regionID, "", domain.Tags{}, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "test-model", model.Name)
	repo.AssertExpectations(t)
}

func TestRegisteredModelUseCase_Create_EmptyName(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	_, err := uc.Create(context.Background(), uuid.New(), nil, "", "desc", uuid.New(), "", domain.Tags{}, nil, nil)
	assert.ErrorIs(t, err, domain.ErrInvalidModelName)
}

func TestRegisteredModelUseCase_Create_NameConflict(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RegisteredModel")).Return(domain.ErrModelNameConflict)

	_, err := uc.Create(context.Background(), uuid.New(), nil, "dup", "desc", uuid.New(), "", domain.Tags{}, nil, nil)
	assert.ErrorIs(t, err, domain.ErrModelNameConflict)
}

func TestRegisteredModelUseCase_Get(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	projectID := uuid.New()
	id := uuid.New()
	expected := &domain.RegisteredModel{ID: id, Name: "m1"}
	repo.On("GetByID", mock.Anything, projectID, id).Return(expected, nil)

	model, err := uc.Get(context.Background(), projectID, id)
	assert.NoError(t, err)
	assert.Equal(t, "m1", model.Name)
}

func TestRegisteredModelUseCase_Get_NotFound(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	projectID := uuid.New()
	id := uuid.New()
	repo.On("GetByID", mock.Anything, projectID, id).Return(nil, domain.ErrModelNotFound)

	_, err := uc.Get(context.Background(), projectID, id)
	assert.ErrorIs(t, err, domain.ErrModelNotFound)
}

func TestRegisteredModelUseCase_List(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	projectID := uuid.New()
	filter := domain.ListFilter{ProjectID: projectID, Limit: 10}
	models := []*domain.RegisteredModel{{ID: uuid.New(), Name: "m1"}}

	repo.On("List", mock.Anything, filter).Return(models, 1, nil)

	result, total, err := uc.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
}

func TestRegisteredModelUseCase_List_DefaultLimit(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	filter := domain.ListFilter{ProjectID: uuid.New(), Limit: 0}
	expectedFilter := filter
	expectedFilter.Limit = 20

	repo.On("List", mock.Anything, expectedFilter).Return([]*domain.RegisteredModel{}, 0, nil)

	_, _, err := uc.List(context.Background(), filter)
	assert.NoError(t, err)
}

func TestRegisteredModelUseCase_Update(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{
		ID: id, Name: "old", State: domain.ModelStateLive,
		Tags: domain.Tags{}, Labels: map[string]string{},
	}

	repo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	repo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)

	updated, err := uc.Update(context.Background(), projectID, id, map[string]interface{}{"name": "new"})
	assert.NoError(t, err)
	assert.Equal(t, "new", updated.Name)
}

func TestRegisteredModelUseCase_Delete_NotArchived(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{ID: id, State: domain.ModelStateLive}
	repo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)

	err := uc.Delete(context.Background(), projectID, id)
	assert.ErrorIs(t, err, domain.ErrCannotDeleteModel)
}

func TestRegisteredModelUseCase_Delete_Success(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	uc := NewRegisteredModelUseCase(repo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{ID: id, State: domain.ModelStateArchived}
	repo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	repo.On("Delete", mock.Anything, projectID, id).Return(nil)

	err := uc.Delete(context.Background(), projectID, id)
	assert.NoError(t, err)
}

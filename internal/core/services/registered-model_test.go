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

func TestRegisteredModelService_Create(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	projectID := uuid.New()
	modelID := uuid.New()

	returnedModel := &domain.RegisteredModel{
		ID:               modelID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProjectID:        projectID,
		Name:             "test-model",
		Slug:             "test-model",
		ModelType:        domain.ModelTypeCustomTrain,
		State:            domain.ModelStateLive,
		DeploymentStatus: domain.DeploymentStatusUndeployed,
		Tags:             domain.Tags{Frameworks: []string{}, Architectures: []string{}, Tasks: []string{}, Subjects: []string{}},
		Labels:           map[string]string{},
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)
	repo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returnedModel, nil)

	model, err := svc.Create(context.Background(), projectID, nil, "test-model", "desc", "owner@test.com", "", domain.Tags{}, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "test-model", model.Name)
	repo.AssertExpectations(t)
}

func TestRegisteredModelService_Create_EmptyName(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	_, err := svc.Create(context.Background(), uuid.New(), nil, "", "desc", "", "", domain.Tags{}, nil, nil)
	assert.ErrorIs(t, err, domain.ErrInvalidModelName)
}

func TestRegisteredModelService_Create_NameConflict(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RegisteredModel")).Return(domain.ErrModelNameConflict)

	_, err := svc.Create(context.Background(), uuid.New(), nil, "dup", "desc", "", "", domain.Tags{}, nil, nil)
	assert.ErrorIs(t, err, domain.ErrModelNameConflict)
}

func TestRegisteredModelService_Get(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	projectID := uuid.New()
	id := uuid.New()
	expected := &domain.RegisteredModel{ID: id, Name: "m1"}
	repo.On("GetByID", mock.Anything, projectID, id).Return(expected, nil)

	model, err := svc.Get(context.Background(), projectID, id)
	assert.NoError(t, err)
	assert.Equal(t, "m1", model.Name)
}

func TestRegisteredModelService_Get_NotFound(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	projectID := uuid.New()
	id := uuid.New()
	repo.On("GetByID", mock.Anything, projectID, id).Return(nil, domain.ErrModelNotFound)

	_, err := svc.Get(context.Background(), projectID, id)
	assert.ErrorIs(t, err, domain.ErrModelNotFound)
}

func TestRegisteredModelService_List(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	projectID := uuid.New()
	filter := ports.ListFilter{ProjectID: projectID, Limit: 10}
	models := []*domain.RegisteredModel{{ID: uuid.New(), Name: "m1"}}

	repo.On("List", mock.Anything, filter).Return(models, 1, nil)

	result, total, err := svc.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
}

func TestRegisteredModelService_List_DefaultLimit(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	filter := ports.ListFilter{ProjectID: uuid.New(), Limit: 0}
	expectedFilter := filter
	expectedFilter.Limit = 20

	repo.On("List", mock.Anything, expectedFilter).Return([]*domain.RegisteredModel{}, 0, nil)

	_, _, err := svc.List(context.Background(), filter)
	assert.NoError(t, err)
}

func TestRegisteredModelService_Update(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{
		ID: id, Name: "old", State: domain.ModelStateLive,
		Tags: domain.Tags{}, Labels: map[string]string{},
	}

	repo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	repo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)

	updated, err := svc.Update(context.Background(), projectID, id, map[string]interface{}{"name": "new"})
	assert.NoError(t, err)
	assert.Equal(t, "new", updated.Name)
}

func TestRegisteredModelService_Delete_NotArchived(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{ID: id, State: domain.ModelStateLive}
	repo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)

	err := svc.Delete(context.Background(), projectID, id)
	assert.ErrorIs(t, err, domain.ErrCannotDeleteModel)
}

func TestRegisteredModelService_Delete_Success(t *testing.T) {
	repo := new(testutil.MockRegisteredModelRepo)
	svc := NewRegisteredModelService(repo)

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{ID: id, State: domain.ModelStateArchived}
	repo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	repo.On("Delete", mock.Anything, projectID, id).Return(nil)

	err := svc.Delete(context.Background(), projectID, id)
	assert.NoError(t, err)
}

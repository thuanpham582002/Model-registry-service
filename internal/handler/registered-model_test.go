package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"model-registry-service/internal/domain"
	"model-registry-service/internal/testutil"
	"model-registry-service/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupModelRouter() (*Handler, *testutil.MockRegisteredModelRepo, *testutil.MockModelVersionRepo, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	modelRepo := new(testutil.MockRegisteredModelRepo)
	versionRepo := new(testutil.MockModelVersionRepo)

	modelUC := usecase.NewRegisteredModelUseCase(modelRepo)
	versionUC := usecase.NewModelVersionUseCase(versionRepo, modelRepo)
	artifactUC := usecase.NewModelArtifactUseCase(versionRepo, modelRepo)

	h := New(modelUC, versionUC, artifactUC)
	r := gin.New()
	api := r.Group("/api/v1/model-registry")
	h.RegisterRoutes(api)

	return h, modelRepo, versionRepo, r
}

func TestListModels(t *testing.T) {
	_, modelRepo, _, r := setupModelRouter()

	projectID := uuid.New()
	models := []*domain.RegisteredModel{
		{
			ID: uuid.New(), Name: "m1", ProjectID: projectID,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			ModelType: domain.ModelTypeCustomTrain, State: domain.ModelStateLive,
			DeploymentStatus: domain.DeploymentStatusUndeployed,
			Tags: domain.Tags{Frameworks: []string{}, Architectures: []string{}, Tasks: []string{}, Subjects: []string{}},
			Labels: map[string]string{},
		},
	}
	modelRepo.On("List", mock.Anything, mock.AnythingOfType("domain.ListFilter")).Return(models, 1, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models?limit=10&offset=0", nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(1), resp["total"])
}

func TestListModels_MissingProjectID(t *testing.T) {
	_, _, _, r := setupModelRouter()

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetModel(t *testing.T) {
	_, modelRepo, _, r := setupModelRouter()

	projectID := uuid.New()
	id := uuid.New()
	model := &domain.RegisteredModel{
		ID: id, Name: "m1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		ModelType: domain.ModelTypeCustomTrain, State: domain.ModelStateLive,
		DeploymentStatus: domain.DeploymentStatusUndeployed,
		Tags: domain.Tags{Frameworks: []string{}, Architectures: []string{}, Tasks: []string{}, Subjects: []string{}},
		Labels: map[string]string{},
	}
	modelRepo.On("GetByID", mock.Anything, projectID, id).Return(model, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/"+id.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetModel_NotFound(t *testing.T) {
	_, modelRepo, _, r := setupModelRouter()

	projectID := uuid.New()
	id := uuid.New()
	modelRepo.On("GetByID", mock.Anything, projectID, id).Return(nil, domain.ErrModelNotFound)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/"+id.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetModel_InvalidID(t *testing.T) {
	_, _, _, r := setupModelRouter()

	projectID := uuid.New()
	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/not-a-uuid", nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateModel(t *testing.T) {
	_, modelRepo, _, r := setupModelRouter()

	projectID := uuid.New()
	regionID := uuid.New()
	returnedModel := &domain.RegisteredModel{
		ID: uuid.New(), Name: "new-model", ProjectID: projectID, RegionID: regionID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		ModelType: domain.ModelTypeCustomTrain, State: domain.ModelStateLive,
		DeploymentStatus: domain.DeploymentStatusUndeployed,
		Tags: domain.Tags{Frameworks: []string{}, Architectures: []string{}, Tasks: []string{}, Subjects: []string{}},
		Labels: map[string]string{},
	}

	modelRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)
	modelRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returnedModel, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":      "new-model",
		"region_id": regionID.String(),
	})

	req, _ := http.NewRequest("POST", "/api/v1/model-registry/models", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateModel_MissingName(t *testing.T) {
	_, _, _, r := setupModelRouter()

	projectID := uuid.New()
	body, _ := json.Marshal(map[string]interface{}{
		"region_id": uuid.New().String(),
	})

	req, _ := http.NewRequest("POST", "/api/v1/model-registry/models", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateModel(t *testing.T) {
	_, modelRepo, _, r := setupModelRouter()

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{
		ID: id, Name: "old",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		ModelType: domain.ModelTypeCustomTrain, State: domain.ModelStateLive,
		DeploymentStatus: domain.DeploymentStatusUndeployed,
		Tags: domain.Tags{Frameworks: []string{}, Architectures: []string{}, Tasks: []string{}, Subjects: []string{}},
		Labels: map[string]string{},
	}

	modelRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	modelRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"name": "updated"})
	req, _ := http.NewRequest("PATCH", "/api/v1/model-registry/models/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteModel(t *testing.T) {
	_, modelRepo, _, r := setupModelRouter()

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{ID: id, State: domain.ModelStateArchived}
	modelRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	modelRepo.On("Delete", mock.Anything, projectID, id).Return(nil)

	req, _ := http.NewRequest("DELETE", "/api/v1/model-registry/models/"+id.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteModel_NotArchived(t *testing.T) {
	_, modelRepo, _, r := setupModelRouter()

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{ID: id, State: domain.ModelStateLive}
	modelRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)

	req, _ := http.NewRequest("DELETE", "/api/v1/model-registry/models/"+id.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

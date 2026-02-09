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

func setupArtifactRouter() (*testutil.MockRegisteredModelRepo, *testutil.MockModelVersionRepo, *gin.Engine) {
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

	return modelRepo, versionRepo, r
}

func TestGetModelArtifact(t *testing.T) {
	_, versionRepo, r := setupArtifactRouter()

	projectID := uuid.New()
	id := uuid.New()
	version := &domain.ModelVersion{
		ID: id, Name: "v1", ArtifactType: domain.ArtifactTypeModel,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Labels: map[string]string{},
	}
	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(version, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model_artifacts/"+id.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListModelArtifacts(t *testing.T) {
	_, versionRepo, r := setupArtifactRouter()

	projectID := uuid.New()
	versions := []*domain.ModelVersion{
		{ID: uuid.New(), Name: "v1", CreatedAt: time.Now(), UpdatedAt: time.Now(), Labels: map[string]string{}},
	}
	versionRepo.On("List", mock.Anything, mock.AnythingOfType("domain.VersionListFilter")).Return(versions, 1, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model_artifacts", nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateModelArtifact(t *testing.T) {
	modelRepo, versionRepo, r := setupArtifactRouter()

	projectID := uuid.New()
	modelID := uuid.New()
	parentModel := &domain.RegisteredModel{ID: modelID}
	returnedVersion := &domain.ModelVersion{
		ID: uuid.New(), RegisteredModelID: modelID, Name: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		ArtifactType: domain.ArtifactTypeModel, Labels: map[string]string{},
	}

	modelRepo.On("GetByID", mock.Anything, projectID, modelID).Return(parentModel, nil)
	versionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returnedVersion, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"registered_model_id":     modelID.String(),
		"name":                    "a1",
		"model_framework":         "tf",
		"model_framework_version": "2.0",
		"uri":                     "s3://bucket/artifact",
	})

	req, _ := http.NewRequest("POST", "/api/v1/model-registry/model_artifacts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestUpdateModelArtifact(t *testing.T) {
	_, versionRepo, r := setupArtifactRouter()

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.ModelVersion{
		ID: id, ArtifactType: domain.ArtifactTypeModel,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Labels: map[string]string{},
	}

	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	versionRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"artifact_type": "doc-artifact"})
	req, _ := http.NewRequest("PATCH", "/api/v1/model-registry/model_artifacts/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFindModelArtifact(t *testing.T) {
	_, versionRepo, r := setupArtifactRouter()

	projectID := uuid.New()
	version := &domain.ModelVersion{
		ID: uuid.New(), Name: "v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Labels: map[string]string{},
	}
	versionRepo.On("FindByParams", mock.Anything, projectID, "v1", "", (*uuid.UUID)(nil)).Return(version, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model_artifact?name=v1", nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

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

func setupVersionRouter() (*testutil.MockRegisteredModelRepo, *testutil.MockModelVersionRepo, *gin.Engine) {
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

func TestListModelVersions(t *testing.T) {
	_, versionRepo, r := setupVersionRouter()

	projectID := uuid.New()
	modelID := uuid.New()
	versions := []*domain.ModelVersion{
		{
			ID: uuid.New(), RegisteredModelID: modelID, Name: "v1",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			State: domain.ModelStateLive, Status: domain.VersionStatusReady,
			Labels: map[string]string{},
		},
	}

	versionRepo.On("ListByModel", mock.Anything, modelID, mock.AnythingOfType("domain.VersionListFilter")).Return(versions, 1, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/"+modelID.String()+"/versions", nil)
	req.Header.Set("Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(1), resp["total"])
}

func TestGetModelVersion(t *testing.T) {
	_, versionRepo, r := setupVersionRouter()

	projectID := uuid.New()
	modelID := uuid.New()
	versionID := uuid.New()
	version := &domain.ModelVersion{
		ID: versionID, RegisteredModelID: modelID, Name: "v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		State: domain.ModelStateLive, Status: domain.VersionStatusReady,
		Labels: map[string]string{},
	}

	versionRepo.On("GetByModelAndVersion", mock.Anything, projectID, modelID, versionID).Return(version, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/"+modelID.String()+"/versions/"+versionID.String(), nil)
	req.Header.Set("Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateModelVersion(t *testing.T) {
	modelRepo, versionRepo, r := setupVersionRouter()

	projectID := uuid.New()
	modelID := uuid.New()
	parentModel := &domain.RegisteredModel{ID: modelID, Name: "m1"}
	returnedVersion := &domain.ModelVersion{
		ID: uuid.New(), RegisteredModelID: modelID, Name: "v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		State: domain.ModelStateLive, Status: domain.VersionStatusPending,
		ArtifactType: domain.ArtifactTypeModel, Labels: map[string]string{},
	}

	modelRepo.On("GetByID", mock.Anything, projectID, modelID).Return(parentModel, nil)
	versionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returnedVersion, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":                    "v1",
		"model_framework":        "pytorch",
		"model_framework_version": "2.0",
		"uri":                     "s3://bucket/model",
	})

	req, _ := http.NewRequest("POST", "/api/v1/model-registry/models/"+modelID.String()+"/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGetModelVersionDirect(t *testing.T) {
	_, versionRepo, r := setupVersionRouter()

	projectID := uuid.New()
	id := uuid.New()
	version := &domain.ModelVersion{
		ID: id, Name: "v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		State: domain.ModelStateLive, Status: domain.VersionStatusReady,
		Labels: map[string]string{},
	}

	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(version, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model_versions/"+id.String(), nil)
	req.Header.Set("Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateModelVersionDirect(t *testing.T) {
	_, versionRepo, r := setupVersionRouter()

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.ModelVersion{
		ID: id, Name: "v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		State: domain.ModelStateLive, Status: domain.VersionStatusPending,
		Labels: map[string]string{},
	}

	versionRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	versionRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"status": "READY"})
	req, _ := http.NewRequest("PATCH", "/api/v1/model-registry/model_versions/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAllModelVersions(t *testing.T) {
	_, versionRepo, r := setupVersionRouter()

	projectID := uuid.New()
	versions := []*domain.ModelVersion{
		{ID: uuid.New(), Name: "v1", CreatedAt: time.Now(), UpdatedAt: time.Now(), Labels: map[string]string{}},
	}
	versionRepo.On("List", mock.Anything, mock.AnythingOfType("domain.VersionListFilter")).Return(versions, 1, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model_versions", nil)
	req.Header.Set("Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFindModelVersion(t *testing.T) {
	_, versionRepo, r := setupVersionRouter()

	projectID := uuid.New()
	version := &domain.ModelVersion{
		ID: uuid.New(), Name: "v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Labels: map[string]string{},
	}

	versionRepo.On("FindByParams", mock.Anything, projectID, "v1", "", (*uuid.UUID)(nil)).Return(version, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model_version?name=v1", nil)
	req.Header.Set("Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

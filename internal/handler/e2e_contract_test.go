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
	"github.com/stretchr/testify/require"
)

// setupE2ERouter creates a full handler with mock repos for contract tests.
func setupE2ERouter() (*testutil.MockRegisteredModelRepo, *testutil.MockModelVersionRepo, *gin.Engine) {
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

// ---------------------------------------------------------------------------
// Helper: assert JSON field exists and has expected type
// ---------------------------------------------------------------------------

func assertFieldString(t *testing.T, resp map[string]interface{}, key string) {
	t.Helper()
	val, ok := resp[key]
	assert.True(t, ok, "response missing field %q", key)
	if ok {
		_, isStr := val.(string)
		assert.True(t, isStr, "field %q should be string, got %T", key, val)
	}
}

func assertFieldNumber(t *testing.T, resp map[string]interface{}, key string) {
	t.Helper()
	val, ok := resp[key]
	assert.True(t, ok, "response missing field %q", key)
	if ok {
		_, isNum := val.(float64)
		assert.True(t, isNum, "field %q should be number, got %T", key, val)
	}
}

func assertFieldBool(t *testing.T, resp map[string]interface{}, key string) {
	t.Helper()
	val, ok := resp[key]
	assert.True(t, ok, "response missing field %q", key)
	if ok {
		_, isBool := val.(bool)
		assert.True(t, isBool, "field %q should be bool, got %T", key, val)
	}
}

func assertFieldMap(t *testing.T, resp map[string]interface{}, key string) {
	t.Helper()
	val, ok := resp[key]
	assert.True(t, ok, "response missing field %q", key)
	if ok && val != nil {
		_, isMap := val.(map[string]interface{})
		assert.True(t, isMap, "field %q should be object/map, got %T", key, val)
	}
}

func assertFieldArray(t *testing.T, resp map[string]interface{}, key string) {
	t.Helper()
	val, ok := resp[key]
	assert.True(t, ok, "response missing field %q", key)
	if ok {
		_, isArr := val.([]interface{})
		assert.True(t, isArr, "field %q should be array, got %T", key, val)
	}
}

// assertModelResponseFields checks all fields the SDK from_dict() expects.
func assertModelResponseFields(t *testing.T, resp map[string]interface{}) {
	t.Helper()
	assertFieldString(t, resp, "id")
	assertFieldString(t, resp, "name")
	assertFieldString(t, resp, "state")
	assertFieldString(t, resp, "description")
	assertFieldString(t, resp, "slug")
	assertFieldString(t, resp, "project_id")
	assertFieldString(t, resp, "region_id")
	assertFieldString(t, resp, "model_type")
	assertFieldNumber(t, resp, "model_size")
	assertFieldString(t, resp, "deployment_status")
	assertFieldString(t, resp, "created_at")
	assertFieldString(t, resp, "updated_at")
	assertFieldNumber(t, resp, "version_count")

	// tags: object with frameworks, architectures, tasks, subjects
	tags, ok := resp["tags"]
	assert.True(t, ok, "response missing field 'tags'")
	if ok && tags != nil {
		tagsMap, isMap := tags.(map[string]interface{})
		assert.True(t, isMap, "tags should be object")
		if isMap {
			assertFieldArray(t, tagsMap, "frameworks")
			assertFieldArray(t, tagsMap, "architectures")
			assertFieldArray(t, tagsMap, "tasks")
			assertFieldArray(t, tagsMap, "subjects")
		}
	}

	// labels: map
	assertFieldMap(t, resp, "labels")
}

// assertVersionResponseFields checks all fields the SDK from_dict() expects.
func assertVersionResponseFields(t *testing.T, resp map[string]interface{}) {
	t.Helper()
	assertFieldString(t, resp, "id")
	assertFieldString(t, resp, "name")
	assertFieldString(t, resp, "registered_model_id")
	assertFieldString(t, resp, "state")
	assertFieldString(t, resp, "description")
	assertFieldBool(t, resp, "is_default")
	assertFieldString(t, resp, "status")
	assertFieldString(t, resp, "artifact_type")
	assertFieldString(t, resp, "model_framework")
	assertFieldString(t, resp, "model_framework_version")
	assertFieldString(t, resp, "container_image")
	assertFieldString(t, resp, "uri")
	assertFieldMap(t, resp, "labels")
	assertFieldString(t, resp, "created_at")
	assertFieldString(t, resp, "updated_at")
}

// assertArtifactResponseFields checks all fields the SDK from_dict() expects.
func assertArtifactResponseFields(t *testing.T, resp map[string]interface{}) {
	t.Helper()
	assertFieldString(t, resp, "id")
	assertFieldString(t, resp, "name")
	assertFieldString(t, resp, "uri")
	assertFieldString(t, resp, "artifact_type")
	assertFieldString(t, resp, "model_framework")
	assertFieldString(t, resp, "model_framework_version")
	assertFieldString(t, resp, "registered_model_id")
	assertFieldMap(t, resp, "labels")
	assertFieldString(t, resp, "created_at")
	assertFieldString(t, resp, "updated_at")
}

// assertListResponseFields checks pagination envelope fields.
func assertListResponseFields(t *testing.T, resp map[string]interface{}) {
	t.Helper()
	assertFieldArray(t, resp, "items")
	assertFieldNumber(t, resp, "total")
	assertFieldNumber(t, resp, "page_size")
	assertFieldNumber(t, resp, "next_offset")
}

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

func fixtureModel(projectID, regionID uuid.UUID) *domain.RegisteredModel {
	return &domain.RegisteredModel{
		ID:               uuid.New(),
		Name:             "test-model",
		Slug:             "test-model",
		Description:      "A test model",
		ProjectID:        projectID,
		RegionID:         regionID,
		ModelType:        domain.ModelTypeCustomTrain,
		ModelSize:        1024,
		State:            domain.ModelStateLive,
		DeploymentStatus: domain.DeploymentStatusUndeployed,
		Tags:             domain.Tags{Frameworks: []string{"pytorch"}, Architectures: []string{"transformer"}, Tasks: []string{"classification"}, Subjects: []string{"nlp"}},
		Labels:           map[string]string{"env": "test"},
		VersionCount:     0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func fixtureVersion(modelID uuid.UUID) *domain.ModelVersion {
	return &domain.ModelVersion{
		ID:                    uuid.New(),
		RegisteredModelID:     modelID,
		Name:                  "v1",
		Description:           "first version",
		IsDefault:             false,
		State:                 domain.ModelStateLive,
		Status:                domain.VersionStatusPending,
		ArtifactType:          domain.ArtifactTypeModel,
		ModelFramework:        "pytorch",
		ModelFrameworkVersion: "2.0",
		ContainerImage:        "registry/img:latest",
		URI:                   "s3://bucket/model",
		Labels:                map[string]string{"stage": "dev"},
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

// ===========================================================================
// RegisteredModel E2E contract tests
// ===========================================================================

func TestE2E_CreateModel(t *testing.T) {
	modelRepo, _, r := setupE2ERouter()

	projectID := uuid.New()
	regionID := uuid.New()
	returned := fixtureModel(projectID, regionID)

	modelRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)
	modelRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returned, nil)

	// SDK payload format
	body, _ := json.Marshal(map[string]interface{}{
		"name":       "test-model",
		"region_id":  regionID.String(),
		"model_type": "CUSTOMTRAIN",
		"tags": map[string]interface{}{
			"frameworks":    []string{"pytorch"},
			"architectures": []string{"transformer"},
			"tasks":         []string{"classification"},
			"subjects":      []string{"nlp"},
		},
		"labels": map[string]string{"env": "test"},
	})

	req, _ := http.NewRequest("POST", "/api/v1/model-registry/models", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertModelResponseFields(t, resp)

	// Verify specific values
	assert.Equal(t, "test-model", resp["name"])
	assert.Equal(t, "LIVE", resp["state"])
}

func TestE2E_GetModel(t *testing.T) {
	modelRepo, _, r := setupE2ERouter()

	projectID := uuid.New()
	regionID := uuid.New()
	model := fixtureModel(projectID, regionID)

	modelRepo.On("GetByID", mock.Anything, projectID, model.ID).Return(model, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/"+model.ID.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertModelResponseFields(t, resp)
	assert.Equal(t, model.ID.String(), resp["id"])
}

func TestE2E_ListModels(t *testing.T) {
	modelRepo, _, r := setupE2ERouter()

	projectID := uuid.New()
	regionID := uuid.New()
	models := []*domain.RegisteredModel{fixtureModel(projectID, regionID)}

	modelRepo.On("List", mock.Anything, mock.AnythingOfType("domain.ListFilter")).Return(models, 1, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models?limit=10&offset=0", nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertListResponseFields(t, resp)

	items := resp["items"].([]interface{})
	require.Len(t, items, 1)
	assertModelResponseFields(t, items[0].(map[string]interface{}))
	assert.Equal(t, float64(1), resp["total"])
	assert.Equal(t, float64(10), resp["page_size"])
	assert.Equal(t, float64(1), resp["next_offset"])
}

func TestE2E_FindModel(t *testing.T) {
	modelRepo, _, r := setupE2ERouter()

	projectID := uuid.New()
	regionID := uuid.New()
	model := fixtureModel(projectID, regionID)

	modelRepo.On("GetByParams", mock.Anything, projectID, "test-model", "").Return(model, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model?name=test-model", nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertModelResponseFields(t, resp)
	assert.Equal(t, "test-model", resp["name"])
}

func TestE2E_UpdateModel(t *testing.T) {
	modelRepo, _, r := setupE2ERouter()

	projectID := uuid.New()
	regionID := uuid.New()
	existing := fixtureModel(projectID, regionID)
	updated := fixtureModel(projectID, regionID)
	updated.ID = existing.ID
	updated.Name = "updated-model"
	updated.Description = "updated desc"

	modelRepo.On("GetByID", mock.Anything, projectID, existing.ID).Return(existing, nil).Once()
	modelRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.RegisteredModel")).Return(nil)
	modelRepo.On("GetByID", mock.Anything, projectID, existing.ID).Return(updated, nil)

	// SDK update payload
	body, _ := json.Marshal(map[string]interface{}{
		"name":        "updated-model",
		"description": "updated desc",
	})

	req, _ := http.NewRequest("PATCH", "/api/v1/model-registry/models/"+existing.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertModelResponseFields(t, resp)
	assert.Equal(t, "updated-model", resp["name"])
}

func TestE2E_DeleteModel(t *testing.T) {
	modelRepo, _, r := setupE2ERouter()

	projectID := uuid.New()
	id := uuid.New()
	existing := &domain.RegisteredModel{ID: id, State: domain.ModelStateArchived}
	modelRepo.On("GetByID", mock.Anything, projectID, id).Return(existing, nil)
	modelRepo.On("Delete", mock.Anything, projectID, id).Return(nil)

	req, _ := http.NewRequest("DELETE", "/api/v1/model-registry/models/"+id.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "deleted", resp["status"])
}

// ===========================================================================
// ModelVersion E2E contract tests
// ===========================================================================

func TestE2E_CreateVersion(t *testing.T) {
	modelRepo, versionRepo, r := setupE2ERouter()

	projectID := uuid.New()
	modelID := uuid.New()
	parentModel := &domain.RegisteredModel{ID: modelID, Name: "m1"}
	returned := fixtureVersion(modelID)

	modelRepo.On("GetByID", mock.Anything, projectID, modelID).Return(parentModel, nil)
	versionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returned, nil)

	// SDK payload format
	body, _ := json.Marshal(map[string]interface{}{
		"name":                    "v1",
		"model_framework":        "pytorch",
		"model_framework_version": "2.0",
		"uri":                     "s3://bucket/model",
	})

	req, _ := http.NewRequest("POST", "/api/v1/model-registry/models/"+modelID.String()+"/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertVersionResponseFields(t, resp)

	assert.Equal(t, modelID.String(), resp["registered_model_id"])
	assert.Equal(t, "PENDING", resp["status"])
	assert.Equal(t, "model-artifact", resp["artifact_type"])
}

func TestE2E_GetVersion(t *testing.T) {
	_, versionRepo, r := setupE2ERouter()

	projectID := uuid.New()
	modelID := uuid.New()
	version := fixtureVersion(modelID)

	versionRepo.On("GetByModelAndVersion", mock.Anything, projectID, modelID, version.ID).Return(version, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/"+modelID.String()+"/versions/"+version.ID.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertVersionResponseFields(t, resp)
	assert.Equal(t, version.ID.String(), resp["id"])
}

func TestE2E_ListVersions(t *testing.T) {
	_, versionRepo, r := setupE2ERouter()

	projectID := uuid.New()
	modelID := uuid.New()
	versions := []*domain.ModelVersion{fixtureVersion(modelID)}

	versionRepo.On("ListByModel", mock.Anything, modelID, mock.AnythingOfType("domain.VersionListFilter")).Return(versions, 1, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/models/"+modelID.String()+"/versions?limit=10", nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertListResponseFields(t, resp)

	items := resp["items"].([]interface{})
	require.Len(t, items, 1)
	assertVersionResponseFields(t, items[0].(map[string]interface{}))
}

func TestE2E_UpdateVersion(t *testing.T) {
	_, versionRepo, r := setupE2ERouter()

	projectID := uuid.New()
	modelID := uuid.New()
	existing := fixtureVersion(modelID)
	updated := fixtureVersion(modelID)
	updated.ID = existing.ID
	updated.Status = domain.VersionStatusReady

	versionRepo.On("GetByID", mock.Anything, projectID, existing.ID).Return(existing, nil).Once()
	versionRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, existing.ID).Return(updated, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"status": "READY",
	})

	req, _ := http.NewRequest("PATCH", "/api/v1/model-registry/models/"+modelID.String()+"/versions/"+existing.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertVersionResponseFields(t, resp)
}

// ===========================================================================
// ModelArtifact E2E contract tests
// ===========================================================================

func TestE2E_CreateArtifact(t *testing.T) {
	modelRepo, versionRepo, r := setupE2ERouter()

	projectID := uuid.New()
	modelID := uuid.New()
	parentModel := &domain.RegisteredModel{ID: modelID}
	returned := fixtureVersion(modelID)
	returned.ArtifactType = domain.ArtifactTypeModel

	modelRepo.On("GetByID", mock.Anything, projectID, modelID).Return(parentModel, nil)
	versionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, mock.AnythingOfType("uuid.UUID")).Return(returned, nil)

	// SDK payload format
	body, _ := json.Marshal(map[string]interface{}{
		"registered_model_id":     modelID.String(),
		"name":                    "a1",
		"uri":                     "s3://bucket/artifact",
		"model_framework":        "tensorflow",
		"model_framework_version": "2.12",
	})

	req, _ := http.NewRequest("POST", "/api/v1/model-registry/model_artifacts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertArtifactResponseFields(t, resp)

	assert.Equal(t, modelID.String(), resp["registered_model_id"])
}

func TestE2E_GetArtifact(t *testing.T) {
	_, versionRepo, r := setupE2ERouter()

	projectID := uuid.New()
	modelID := uuid.New()
	version := fixtureVersion(modelID)
	version.ArtifactType = domain.ArtifactTypeModel

	versionRepo.On("GetByID", mock.Anything, projectID, version.ID).Return(version, nil)

	req, _ := http.NewRequest("GET", "/api/v1/model-registry/model_artifacts/"+version.ID.String(), nil)
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertArtifactResponseFields(t, resp)
	assert.Equal(t, version.ID.String(), resp["id"])
}

func TestE2E_UpdateArtifact(t *testing.T) {
	_, versionRepo, r := setupE2ERouter()

	projectID := uuid.New()
	modelID := uuid.New()
	existing := fixtureVersion(modelID)
	existing.ArtifactType = domain.ArtifactTypeModel
	updated := fixtureVersion(modelID)
	updated.ID = existing.ID
	updated.ArtifactType = domain.ArtifactTypeDoc

	versionRepo.On("GetByID", mock.Anything, projectID, existing.ID).Return(existing, nil).Once()
	versionRepo.On("Update", mock.Anything, projectID, mock.AnythingOfType("*domain.ModelVersion")).Return(nil)
	versionRepo.On("GetByID", mock.Anything, projectID, existing.ID).Return(updated, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"artifact_type": "doc-artifact",
	})

	req, _ := http.NewRequest("PATCH", "/api/v1/model-registry/model_artifacts/"+existing.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-ID", projectID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertArtifactResponseFields(t, resp)
}

package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type DeployService struct {
	envRepo        output.ServingEnvironmentRepository
	isvcRepo       output.InferenceServiceRepository
	serveModelRepo output.ServeModelRepository
	modelRepo      output.RegisteredModelRepository
	versionRepo    output.ModelVersionRepository
	kserve         output.KServeClient
}

func NewDeployService(
	envRepo output.ServingEnvironmentRepository,
	isvcRepo output.InferenceServiceRepository,
	serveModelRepo output.ServeModelRepository,
	modelRepo output.RegisteredModelRepository,
	versionRepo output.ModelVersionRepository,
	kserve output.KServeClient,
) *DeployService {
	return &DeployService{
		envRepo:        envRepo,
		isvcRepo:       isvcRepo,
		serveModelRepo: serveModelRepo,
		modelRepo:      modelRepo,
		versionRepo:    versionRepo,
		kserve:         kserve,
	}
}

type DeployRequest struct {
	ProjectID            uuid.UUID
	RegisteredModelID    uuid.UUID
	ModelVersionIDs      []uuid.UUID // Support multiple versions for multi-model serving
	ServingEnvironmentID uuid.UUID
	Name                 string
	Labels               map[string]string
}

type DeployResult struct {
	InferenceService *domain.InferenceService
	ServedModels     []*domain.ServeModel
	Status           string // PENDING, DEPLOYED, FAILED
	Message          string
}

func (s *DeployService) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	// 1. Validate serving environment exists
	env, err := s.envRepo.GetByID(ctx, req.ProjectID, req.ServingEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("get serving environment: %w", err)
	}

	// 2. Get model
	model, err := s.modelRepo.GetByID(ctx, req.ProjectID, req.RegisteredModelID)
	if err != nil {
		return nil, fmt.Errorf("get model: %w", err)
	}

	// 3. Resolve versions (use provided list or resolve default)
	versions, err := s.resolveVersions(ctx, req.ProjectID, req.RegisteredModelID, req.ModelVersionIDs, model)
	if err != nil {
		return nil, err
	}

	// 4. Validate all versions are ready
	for _, version := range versions {
		if version.Status != domain.VersionStatusReady {
			return nil, fmt.Errorf("version %s: %w", version.Name, domain.ErrVersionNotReady)
		}
	}

	// 5. Generate name if not provided
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s-%s", model.Slug, versions[0].ID.String()[:8])
	}

	// 6. Create InferenceService entity (without model_version_id)
	isvc, err := domain.NewInferenceService(req.ProjectID, name, env.ID, model.ID)
	if err != nil {
		return nil, err
	}
	if req.Labels != nil {
		isvc.Labels = req.Labels
	}

	// 7. Save inference service to database
	if err := s.isvcRepo.Create(ctx, isvc); err != nil {
		return nil, fmt.Errorf("create inference service: %w", err)
	}

	// 8. Create ServeModel entries for each version
	var servedModels []*domain.ServeModel
	for _, version := range versions {
		sm, err := domain.NewServeModel(req.ProjectID, isvc.ID, version.ID)
		if err != nil {
			return nil, fmt.Errorf("create serve model: %w", err)
		}
		if err := s.serveModelRepo.Create(ctx, sm); err != nil {
			return nil, fmt.Errorf("save serve model: %w", err)
		}
		servedModels = append(servedModels, sm)
	}

	// Fetch with joined fields
	isvc, _ = s.isvcRepo.GetByID(ctx, req.ProjectID, isvc.ID)
	isvc.ServedModels = servedModels

	// 9. Deploy to KServe (if available) - deploy first version as primary
	if s.kserve != nil && s.kserve.IsAvailable() {
		namespace := env.ExternalID
		if namespace == "" {
			namespace = env.Name
		}

		// For multi-model, KServe may need different handling
		// Currently deploy with first version as primary
		deployment, err := s.kserve.Deploy(ctx, namespace, isvc, versions[0])
		if err != nil {
			isvc.MarkFailed(err.Error())
			s.isvcRepo.Update(ctx, req.ProjectID, isvc)

			// Mark all serve models as failed
			for _, sm := range servedModels {
				sm.SetState(domain.ServeStateFailed)
				s.serveModelRepo.Update(ctx, req.ProjectID, sm)
			}

			return &DeployResult{
				InferenceService: isvc,
				ServedModels:     servedModels,
				Status:           "FAILED",
				Message:          err.Error(),
			}, nil
		}

		isvc.SetExternalID(deployment.ExternalID)
		s.isvcRepo.Update(ctx, req.ProjectID, isvc)
	}

	return &DeployResult{
		InferenceService: isvc,
		ServedModels:     servedModels,
		Status:           "PENDING",
		Message:          "Deployment initiated",
	}, nil
}

// AddModelVersion adds a model version to an existing inference service
func (s *DeployService) AddModelVersion(ctx context.Context, projectID, isvcID, versionID uuid.UUID) (*domain.ServeModel, error) {
	// Validate inference service exists
	isvc, err := s.isvcRepo.GetByID(ctx, projectID, isvcID)
	if err != nil {
		return nil, err
	}

	// Validate version exists and is ready
	version, err := s.versionRepo.GetByID(ctx, projectID, versionID)
	if err != nil {
		return nil, err
	}
	if version.Status != domain.VersionStatusReady {
		return nil, domain.ErrVersionNotReady
	}

	// Create ServeModel
	sm, err := domain.NewServeModel(projectID, isvc.ID, versionID)
	if err != nil {
		return nil, err
	}
	if err := s.serveModelRepo.Create(ctx, sm); err != nil {
		return nil, fmt.Errorf("create serve model: %w", err)
	}

	return s.serveModelRepo.GetByID(ctx, projectID, sm.ID)
}

// RemoveModelVersion removes a model version from an inference service
func (s *DeployService) RemoveModelVersion(ctx context.Context, projectID, serveModelID uuid.UUID) error {
	return s.serveModelRepo.Delete(ctx, projectID, serveModelID)
}

// GetServedModels returns all model versions served by an inference service
func (s *DeployService) GetServedModels(ctx context.Context, projectID, isvcID uuid.UUID) ([]*domain.ServeModel, error) {
	return s.serveModelRepo.FindByInferenceService(ctx, projectID, isvcID)
}

func (s *DeployService) Undeploy(ctx context.Context, projectID, isvcID uuid.UUID) error {
	// 1. Get inference service
	isvc, err := s.isvcRepo.GetByID(ctx, projectID, isvcID)
	if err != nil {
		return err
	}

	// 2. Get environment for namespace
	env, err := s.envRepo.GetByID(ctx, projectID, isvc.ServingEnvironmentID)
	if err != nil {
		return err
	}

	// 3. Delete from KServe
	if s.kserve != nil && s.kserve.IsAvailable() {
		namespace := env.ExternalID
		if namespace == "" {
			namespace = env.Name
		}
		// Ignore error - might already be deleted
		_ = s.kserve.Undeploy(ctx, namespace, isvc.Name)
	}

	// 4. Update state
	isvc.Undeploy()
	isvc.MarkUndeployed()
	return s.isvcRepo.Update(ctx, projectID, isvc)
}

func (s *DeployService) SyncStatus(ctx context.Context, projectID, isvcID uuid.UUID) (*domain.InferenceService, error) {
	isvc, err := s.isvcRepo.GetByID(ctx, projectID, isvcID)
	if err != nil {
		return nil, err
	}

	if s.kserve == nil || !s.kserve.IsAvailable() {
		return isvc, nil
	}

	env, err := s.envRepo.GetByID(ctx, projectID, isvc.ServingEnvironmentID)
	if err != nil {
		return nil, err
	}

	namespace := env.ExternalID
	if namespace == "" {
		namespace = env.Name
	}

	status, err := s.kserve.GetStatus(ctx, namespace, isvc.Name)
	if err != nil {
		return nil, err
	}

	if status.Ready {
		isvc.MarkDeployed(status.URL)

		// Update serve model states to running
		servedModels, _ := s.serveModelRepo.FindByInferenceService(ctx, projectID, isvcID)
		for _, sm := range servedModels {
			sm.SetState(domain.ServeStateRunning)
			s.serveModelRepo.Update(ctx, projectID, sm)
		}
	} else if status.Error != "" {
		isvc.MarkFailed(status.Error)
	}
	isvc.UpdatedAt = time.Now()

	if err := s.isvcRepo.Update(ctx, projectID, isvc); err != nil {
		return nil, err
	}

	// Load served models
	isvc, _ = s.isvcRepo.GetByID(ctx, projectID, isvcID)
	isvc.ServedModels, _ = s.serveModelRepo.FindByInferenceService(ctx, projectID, isvcID)

	return isvc, nil
}

func (s *DeployService) resolveVersions(
	ctx context.Context,
	projectID, modelID uuid.UUID,
	versionIDs []uuid.UUID,
	model *domain.RegisteredModel,
) ([]*domain.ModelVersion, error) {
	// If specific versions provided, use them
	if len(versionIDs) > 0 {
		var versions []*domain.ModelVersion
		for _, vID := range versionIDs {
			v, err := s.versionRepo.GetByID(ctx, projectID, vID)
			if err != nil {
				return nil, fmt.Errorf("get version %s: %w", vID.String(), err)
			}
			versions = append(versions, v)
		}
		return versions, nil
	}

	// Try default version from model
	if model.DefaultVersion != nil {
		return []*domain.ModelVersion{model.DefaultVersion}, nil
	}

	// Try latest version from model
	if model.LatestVersion != nil {
		return []*domain.ModelVersion{model.LatestVersion}, nil
	}

	return nil, fmt.Errorf("no model version available for deployment")
}

// IsKServeAvailable checks if KServe integration is enabled
func (s *DeployService) IsKServeAvailable() bool {
	return s.kserve != nil && s.kserve.IsAvailable()
}

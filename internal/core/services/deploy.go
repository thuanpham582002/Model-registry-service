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
	envRepo     output.ServingEnvironmentRepository
	isvcRepo    output.InferenceServiceRepository
	modelRepo   output.RegisteredModelRepository
	versionRepo output.ModelVersionRepository
	kserve      output.KServeClient
}

func NewDeployService(
	envRepo output.ServingEnvironmentRepository,
	isvcRepo output.InferenceServiceRepository,
	modelRepo output.RegisteredModelRepository,
	versionRepo output.ModelVersionRepository,
	kserve output.KServeClient,
) *DeployService {
	return &DeployService{
		envRepo:     envRepo,
		isvcRepo:    isvcRepo,
		modelRepo:   modelRepo,
		versionRepo: versionRepo,
		kserve:      kserve,
	}
}

type DeployRequest struct {
	ProjectID            uuid.UUID
	RegisteredModelID    uuid.UUID
	ModelVersionID       *uuid.UUID
	ServingEnvironmentID uuid.UUID
	Name                 string
	Labels               map[string]string
}

type DeployResult struct {
	InferenceService *domain.InferenceService
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

	// 3. Get version (specified or default/latest)
	version, err := s.resolveVersion(ctx, req.ProjectID, req.RegisteredModelID, req.ModelVersionID, model)
	if err != nil {
		return nil, err
	}

	// 4. Validate version is ready
	if version.Status != domain.VersionStatusReady {
		return nil, domain.ErrVersionNotReady
	}

	// 5. Generate name if not provided
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s-%s", model.Slug, version.ID.String()[:8])
	}

	// 6. Create InferenceService entity
	isvc, err := domain.NewInferenceService(req.ProjectID, name, env.ID, model.ID, &version.ID)
	if err != nil {
		return nil, err
	}
	if req.Labels != nil {
		isvc.Labels = req.Labels
	}

	// 7. Save to database
	if err := s.isvcRepo.Create(ctx, isvc); err != nil {
		return nil, fmt.Errorf("create inference service: %w", err)
	}

	// Fetch with joined fields
	isvc, _ = s.isvcRepo.GetByID(ctx, req.ProjectID, isvc.ID)

	// 8. Deploy to KServe (if available)
	if s.kserve != nil && s.kserve.IsAvailable() {
		namespace := env.ExternalID
		if namespace == "" {
			namespace = env.Name
		}

		deployment, err := s.kserve.Deploy(ctx, namespace, isvc, version)
		if err != nil {
			isvc.MarkFailed(err.Error())
			s.isvcRepo.Update(ctx, req.ProjectID, isvc)
			return &DeployResult{
				InferenceService: isvc,
				Status:           "FAILED",
				Message:          err.Error(),
			}, nil
		}

		isvc.SetExternalID(deployment.ExternalID)
		s.isvcRepo.Update(ctx, req.ProjectID, isvc)
	}

	return &DeployResult{
		InferenceService: isvc,
		Status:           "PENDING",
		Message:          "Deployment initiated",
	}, nil
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
	} else if status.Error != "" {
		isvc.MarkFailed(status.Error)
	}
	isvc.UpdatedAt = time.Now()

	if err := s.isvcRepo.Update(ctx, projectID, isvc); err != nil {
		return nil, err
	}

	return s.isvcRepo.GetByID(ctx, projectID, isvcID)
}

func (s *DeployService) resolveVersion(
	ctx context.Context,
	projectID, modelID uuid.UUID,
	versionID *uuid.UUID,
	model *domain.RegisteredModel,
) (*domain.ModelVersion, error) {
	if versionID != nil {
		return s.versionRepo.GetByID(ctx, projectID, *versionID)
	}

	// Try default version from model
	if model.DefaultVersion != nil {
		return model.DefaultVersion, nil
	}

	// Try latest version from model
	if model.LatestVersion != nil {
		return model.LatestVersion, nil
	}

	return nil, fmt.Errorf("no model version available for deployment")
}

// IsKServeAvailable checks if KServe integration is enabled
func (s *DeployService) IsKServeAvailable() bool {
	return s.kserve != nil && s.kserve.IsAvailable()
}

package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/ports/output"
)

type RegisteredModelService struct {
	repo ports.RegisteredModelRepository
}

func NewRegisteredModelService(repo ports.RegisteredModelRepository) *RegisteredModelService {
	return &RegisteredModelService{repo: repo}
}

func (s *RegisteredModelService) Create(ctx context.Context, projectID uuid.UUID, ownerID *uuid.UUID, name, description string, ownerEmail, modelType string, tags domain.Tags, labels map[string]string, parentModelID *uuid.UUID) (*domain.RegisteredModel, error) {
	if name == "" {
		return nil, domain.ErrInvalidModelName
	}

	mt := domain.ModelType(modelType)
	if mt == "" {
		mt = domain.ModelTypeCustomTrain
	}

	now := time.Now()
	model := &domain.RegisteredModel{
		ID:               uuid.New(),
		CreatedAt:        now,
		UpdatedAt:        now,
		ProjectID:        projectID,
		OwnerID:          ownerID,
		OwnerEmail:       ownerEmail,
		Name:             name,
		Slug:             generateSlug(name),
		Description:      description,
		ModelType:        mt,
		State:            domain.ModelStateLive,
		DeploymentStatus: domain.DeploymentStatusUndeployed,
		Tags:             tags,
		Labels:           labels,
		ParentModelID:    parentModelID,
	}

	if model.Labels == nil {
		model.Labels = make(map[string]string)
	}
	if model.Tags.Frameworks == nil {
		model.Tags.Frameworks = []string{}
	}
	if model.Tags.Architectures == nil {
		model.Tags.Architectures = []string{}
	}
	if model.Tags.Tasks == nil {
		model.Tags.Tasks = []string{}
	}
	if model.Tags.Subjects == nil {
		model.Tags.Subjects = []string{}
	}

	if err := s.repo.Create(ctx, model); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, model.ID)
}

func (s *RegisteredModelService) Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.RegisteredModel, error) {
	return s.repo.GetByID(ctx, projectID, id)
}

func (s *RegisteredModelService) GetByParams(ctx context.Context, projectID uuid.UUID, name, externalID string) (*domain.RegisteredModel, error) {
	return s.repo.GetByParams(ctx, projectID, name, externalID)
}

func (s *RegisteredModelService) List(ctx context.Context, filter ports.ListFilter) ([]*domain.RegisteredModel, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.List(ctx, filter)
}

func (s *RegisteredModelService) Update(ctx context.Context, projectID uuid.UUID, id uuid.UUID, updates map[string]interface{}) (*domain.RegisteredModel, error) {
	model, err := s.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["name"]; ok && v != nil {
		model.Name = v.(string)
	}
	if v, ok := updates["description"]; ok && v != nil {
		model.Description = v.(string)
	}
	if v, ok := updates["model_type"]; ok && v != nil {
		model.ModelType = domain.ModelType(v.(string))
	}
	if v, ok := updates["model_size"]; ok && v != nil {
		model.ModelSize = v.(int64)
	}
	if v, ok := updates["state"]; ok && v != nil {
		model.State = domain.ModelState(v.(string))
	}
	if v, ok := updates["deployment_status"]; ok && v != nil {
		model.DeploymentStatus = domain.DeploymentStatus(v.(string))
	}
	if v, ok := updates["tags"]; ok && v != nil {
		model.Tags = v.(domain.Tags)
	}
	if v, ok := updates["labels"]; ok && v != nil {
		model.Labels = v.(map[string]string)
	}

	if err := s.repo.Update(ctx, projectID, model); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, projectID, id)
}

func (s *RegisteredModelService) Delete(ctx context.Context, projectID uuid.UUID, id uuid.UUID) error {
	model, err := s.repo.GetByID(ctx, projectID, id)
	if err != nil {
		return err
	}

	if model.State != domain.ModelStateArchived {
		return domain.ErrCannotDeleteModel
	}

	return s.repo.Delete(ctx, projectID, id)
}

func generateSlug(name string) string {
	slug := ""
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			slug += string(ch)
		} else if ch >= 'A' && ch <= 'Z' {
			slug += string(ch + 32)
		} else if ch == ' ' || ch == '_' {
			slug += "-"
		}
	}
	if len(slug) > 60 {
		slug = slug[:60]
	}
	return slug
}

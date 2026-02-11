package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type inferenceServiceRepo struct {
	pool *pgxpool.Pool
}

// NewInferenceServiceRepository creates a new InferenceServiceRepository
func NewInferenceServiceRepository(pool *pgxpool.Pool) output.InferenceServiceRepository {
	return &inferenceServiceRepo{pool: pool}
}

func (r *inferenceServiceRepo) Create(ctx context.Context, isvc *domain.InferenceService) error {
	labelsJSON, err := json.Marshal(isvc.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		INSERT INTO inference_service
			(id, created_at, updated_at, project_id, name, external_id,
			 serving_environment_id, registered_model_id,
			 desired_state, current_state, runtime, url, last_error, labels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.pool.Exec(ctx, query,
		isvc.ID, isvc.CreatedAt, isvc.UpdatedAt,
		isvc.ProjectID, isvc.Name, isvc.ExternalID,
		isvc.ServingEnvironmentID, isvc.RegisteredModelID,
		string(isvc.DesiredState), string(isvc.CurrentState),
		isvc.Runtime, isvc.URL, isvc.LastError, labelsJSON,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrInferenceServiceNameConflict
		}
		return fmt.Errorf("create inference service: %w", err)
	}
	return nil
}

func (r *inferenceServiceRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.InferenceService, error) {
	query := `
		SELECT
			i.id, i.created_at, i.updated_at, i.project_id, i.name, i.external_id,
			i.serving_environment_id, i.registered_model_id,
			i.desired_state, i.current_state, i.runtime, i.url, i.last_error, i.labels,
			se.name AS serving_environment_name,
			rm.name AS registered_model_name
		FROM inference_service i
		JOIN serving_environment se ON se.id = i.serving_environment_id
		JOIN registered_model rm ON rm.id = i.registered_model_id
		WHERE i.id = $1 AND i.project_id = $2
	`

	isvc, err := r.scanIsvc(r.pool.QueryRow(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInferenceServiceNotFound
		}
		return nil, fmt.Errorf("get inference service by id: %w", err)
	}
	return isvc, nil
}

func (r *inferenceServiceRepo) GetByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*domain.InferenceService, error) {
	query := `
		SELECT
			i.id, i.created_at, i.updated_at, i.project_id, i.name, i.external_id,
			i.serving_environment_id, i.registered_model_id,
			i.desired_state, i.current_state, i.runtime, i.url, i.last_error, i.labels,
			se.name AS serving_environment_name,
			rm.name AS registered_model_name
		FROM inference_service i
		JOIN serving_environment se ON se.id = i.serving_environment_id
		JOIN registered_model rm ON rm.id = i.registered_model_id
		WHERE i.external_id = $1 AND i.project_id = $2
	`

	isvc, err := r.scanIsvc(r.pool.QueryRow(ctx, query, externalID, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInferenceServiceNotFound
		}
		return nil, fmt.Errorf("get inference service by external id: %w", err)
	}
	return isvc, nil
}

func (r *inferenceServiceRepo) GetByName(ctx context.Context, projectID, envID uuid.UUID, name string) (*domain.InferenceService, error) {
	query := `
		SELECT
			i.id, i.created_at, i.updated_at, i.project_id, i.name, i.external_id,
			i.serving_environment_id, i.registered_model_id,
			i.desired_state, i.current_state, i.runtime, i.url, i.last_error, i.labels,
			se.name AS serving_environment_name,
			rm.name AS registered_model_name
		FROM inference_service i
		JOIN serving_environment se ON se.id = i.serving_environment_id
		JOIN registered_model rm ON rm.id = i.registered_model_id
		WHERE i.serving_environment_id = $1 AND i.name = $2 AND i.project_id = $3
	`

	isvc, err := r.scanIsvc(r.pool.QueryRow(ctx, query, envID, name, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInferenceServiceNotFound
		}
		return nil, fmt.Errorf("get inference service by name: %w", err)
	}
	return isvc, nil
}

func (r *inferenceServiceRepo) Update(ctx context.Context, projectID uuid.UUID, isvc *domain.InferenceService) error {
	labelsJSON, err := json.Marshal(isvc.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		UPDATE inference_service
		SET name = $1, external_id = $2,
			desired_state = $3, current_state = $4, runtime = $5,
			url = $6, last_error = $7, labels = $8, updated_at = NOW()
		WHERE id = $9 AND project_id = $10
	`

	result, err := r.pool.Exec(ctx, query,
		isvc.Name, isvc.ExternalID,
		string(isvc.DesiredState), string(isvc.CurrentState), isvc.Runtime,
		isvc.URL, isvc.LastError, labelsJSON,
		isvc.ID, projectID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrInferenceServiceNameConflict
		}
		return fmt.Errorf("update inference service: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrInferenceServiceNotFound
	}
	return nil
}

func (r *inferenceServiceRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	query := `DELETE FROM inference_service WHERE id = $1 AND project_id = $2`

	result, err := r.pool.Exec(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete inference service: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrInferenceServiceNotFound
	}
	return nil
}

func (r *inferenceServiceRepo) List(ctx context.Context, filter output.InferenceServiceFilter) ([]*domain.InferenceService, int, error) {
	conditions := []string{"i.project_id = $1"}
	args := []interface{}{filter.ProjectID}
	argPos := 2

	if filter.ServingEnvironmentID != nil {
		conditions = append(conditions, fmt.Sprintf("i.serving_environment_id = $%d", argPos))
		args = append(args, *filter.ServingEnvironmentID)
		argPos++
	}
	if filter.RegisteredModelID != nil {
		conditions = append(conditions, fmt.Sprintf("i.registered_model_id = $%d", argPos))
		args = append(args, *filter.RegisteredModelID)
		argPos++
	}
	if filter.DesiredState != "" {
		conditions = append(conditions, fmt.Sprintf("i.desired_state = $%d", argPos))
		args = append(args, filter.DesiredState)
		argPos++
	}
	if filter.CurrentState != "" {
		conditions = append(conditions, fmt.Sprintf("i.current_state = $%d", argPos))
		args = append(args, filter.CurrentState)
		argPos++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM inference_service i WHERE %s
	`, whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count inference services: %w", err)
	}

	// Order
	orderBy := "i.created_at DESC"
	if filter.SortBy != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf("i.%s %s", filter.SortBy, dir)
	}

	query := fmt.Sprintf(`
		SELECT
			i.id, i.created_at, i.updated_at, i.project_id, i.name, i.external_id,
			i.serving_environment_id, i.registered_model_id,
			i.desired_state, i.current_state, i.runtime, i.url, i.last_error, i.labels,
			se.name AS serving_environment_name,
			rm.name AS registered_model_name
		FROM inference_service i
		JOIN serving_environment se ON se.id = i.serving_environment_id
		JOIN registered_model rm ON rm.id = i.registered_model_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argPos, argPos+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list inference services: %w", err)
	}
	defer rows.Close()

	var isvcs []*domain.InferenceService
	for rows.Next() {
		isvc, err := r.scanIsvcFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan inference service row: %w", err)
		}
		isvcs = append(isvcs, isvc)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate inference service rows: %w", err)
	}

	return isvcs, total, nil
}

func (r *inferenceServiceRepo) CountByModel(ctx context.Context, projectID, modelID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM inference_service
		WHERE project_id = $1 AND registered_model_id = $2 AND current_state = 'DEPLOYED'
	`
	var count int
	err := r.pool.QueryRow(ctx, query, projectID, modelID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count inference services by model: %w", err)
	}
	return count, nil
}

func (r *inferenceServiceRepo) CountByEnvironment(ctx context.Context, projectID, envID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM inference_service
		WHERE project_id = $1 AND serving_environment_id = $2
	`
	var count int
	err := r.pool.QueryRow(ctx, query, projectID, envID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count inference services by environment: %w", err)
	}
	return count, nil
}

func (r *inferenceServiceRepo) scanIsvc(row pgx.Row) (*domain.InferenceService, error) {
	isvc := &domain.InferenceService{}
	var labelsJSON []byte
	var desiredState, currentState string

	err := row.Scan(
		&isvc.ID, &isvc.CreatedAt, &isvc.UpdatedAt,
		&isvc.ProjectID, &isvc.Name, &isvc.ExternalID,
		&isvc.ServingEnvironmentID, &isvc.RegisteredModelID,
		&desiredState, &currentState, &isvc.Runtime, &isvc.URL, &isvc.LastError, &labelsJSON,
		&isvc.ServingEnvironmentName, &isvc.RegisteredModelName,
	)
	if err != nil {
		return nil, err
	}

	isvc.DesiredState = domain.InferenceServiceState(desiredState)
	isvc.CurrentState = domain.InferenceServiceState(currentState)

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &isvc.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}
	if isvc.Labels == nil {
		isvc.Labels = make(map[string]string)
	}

	return isvc, nil
}

func (r *inferenceServiceRepo) scanIsvcFromRows(rows pgx.Rows) (*domain.InferenceService, error) {
	isvc := &domain.InferenceService{}
	var labelsJSON []byte
	var desiredState, currentState string

	err := rows.Scan(
		&isvc.ID, &isvc.CreatedAt, &isvc.UpdatedAt,
		&isvc.ProjectID, &isvc.Name, &isvc.ExternalID,
		&isvc.ServingEnvironmentID, &isvc.RegisteredModelID,
		&desiredState, &currentState, &isvc.Runtime, &isvc.URL, &isvc.LastError, &labelsJSON,
		&isvc.ServingEnvironmentName, &isvc.RegisteredModelName,
	)
	if err != nil {
		return nil, err
	}

	isvc.DesiredState = domain.InferenceServiceState(desiredState)
	isvc.CurrentState = domain.InferenceServiceState(currentState)

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &isvc.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}
	if isvc.Labels == nil {
		isvc.Labels = make(map[string]string)
	}

	return isvc, nil
}

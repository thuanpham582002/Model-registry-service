package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
)

type serveModelRepo struct {
	pool *pgxpool.Pool
}

// NewServeModelRepository creates a new ServeModelRepository
func NewServeModelRepository(pool *pgxpool.Pool) output.ServeModelRepository {
	return &serveModelRepo{pool: pool}
}

func (r *serveModelRepo) Create(ctx context.Context, sm *domain.ServeModel) error {
	query := `
		INSERT INTO serve_model
			(id, created_at, updated_at, project_id, inference_service_id, model_version_id, last_known_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		sm.ID, sm.CreatedAt, sm.UpdatedAt,
		sm.ProjectID, sm.InferenceServiceID, sm.ModelVersionID,
		string(sm.LastKnownState),
	)
	if err != nil {
		return fmt.Errorf("create serve model: %w", err)
	}
	return nil
}

func (r *serveModelRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServeModel, error) {
	query := `
		SELECT
			sm.id, sm.created_at, sm.updated_at, sm.project_id,
			sm.inference_service_id, sm.model_version_id, sm.last_known_state,
			isvc.name AS inference_service_name,
			mv.name AS model_version_name
		FROM serve_model sm
		JOIN inference_service isvc ON isvc.id = sm.inference_service_id
		JOIN model_version mv ON mv.id = sm.model_version_id
		WHERE sm.id = $1 AND sm.project_id = $2
	`

	sm, err := r.scanServeModel(r.pool.QueryRow(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrServeModelNotFound
		}
		return nil, fmt.Errorf("get serve model by id: %w", err)
	}
	return sm, nil
}

func (r *serveModelRepo) Update(ctx context.Context, projectID uuid.UUID, sm *domain.ServeModel) error {
	query := `
		UPDATE serve_model
		SET last_known_state = $1, updated_at = NOW()
		WHERE id = $2 AND project_id = $3
	`

	result, err := r.pool.Exec(ctx, query,
		string(sm.LastKnownState), sm.ID, projectID,
	)
	if err != nil {
		return fmt.Errorf("update serve model: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrServeModelNotFound
	}
	return nil
}

func (r *serveModelRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	query := `DELETE FROM serve_model WHERE id = $1 AND project_id = $2`

	result, err := r.pool.Exec(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete serve model: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrServeModelNotFound
	}
	return nil
}

func (r *serveModelRepo) List(ctx context.Context, filter output.ServeModelFilter) ([]*domain.ServeModel, int, error) {
	conditions := []string{"sm.project_id = $1"}
	args := []interface{}{filter.ProjectID}
	argPos := 2

	if filter.InferenceServiceID != nil {
		conditions = append(conditions, fmt.Sprintf("sm.inference_service_id = $%d", argPos))
		args = append(args, *filter.InferenceServiceID)
		argPos++
	}
	if filter.ModelVersionID != nil {
		conditions = append(conditions, fmt.Sprintf("sm.model_version_id = $%d", argPos))
		args = append(args, *filter.ModelVersionID)
		argPos++
	}
	if filter.State != "" {
		conditions = append(conditions, fmt.Sprintf("sm.last_known_state = $%d", argPos))
		args = append(args, filter.State)
		argPos++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM serve_model sm WHERE %s`, whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count serve models: %w", err)
	}

	// Order
	orderBy := "sm.created_at DESC"
	if filter.SortBy != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf("sm.%s %s", filter.SortBy, dir)
	}

	query := fmt.Sprintf(`
		SELECT
			sm.id, sm.created_at, sm.updated_at, sm.project_id,
			sm.inference_service_id, sm.model_version_id, sm.last_known_state,
			isvc.name AS inference_service_name,
			mv.name AS model_version_name
		FROM serve_model sm
		JOIN inference_service isvc ON isvc.id = sm.inference_service_id
		JOIN model_version mv ON mv.id = sm.model_version_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argPos, argPos+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list serve models: %w", err)
	}
	defer rows.Close()

	var serveModels []*domain.ServeModel
	for rows.Next() {
		sm, err := r.scanServeModelFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan serve model row: %w", err)
		}
		serveModels = append(serveModels, sm)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate serve model rows: %w", err)
	}

	return serveModels, total, nil
}

func (r *serveModelRepo) FindByInferenceService(ctx context.Context, projectID, isvcID uuid.UUID) ([]*domain.ServeModel, error) {
	query := `
		SELECT
			sm.id, sm.created_at, sm.updated_at, sm.project_id,
			sm.inference_service_id, sm.model_version_id, sm.last_known_state,
			isvc.name AS inference_service_name,
			mv.name AS model_version_name
		FROM serve_model sm
		JOIN inference_service isvc ON isvc.id = sm.inference_service_id
		JOIN model_version mv ON mv.id = sm.model_version_id
		WHERE sm.inference_service_id = $1 AND sm.project_id = $2
		ORDER BY sm.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, isvcID, projectID)
	if err != nil {
		return nil, fmt.Errorf("find serve models by inference service: %w", err)
	}
	defer rows.Close()

	var serveModels []*domain.ServeModel
	for rows.Next() {
		sm, err := r.scanServeModelFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan serve model row: %w", err)
		}
		serveModels = append(serveModels, sm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serve model rows: %w", err)
	}

	return serveModels, nil
}

func (r *serveModelRepo) FindByModelVersion(ctx context.Context, projectID, versionID uuid.UUID) ([]*domain.ServeModel, error) {
	query := `
		SELECT
			sm.id, sm.created_at, sm.updated_at, sm.project_id,
			sm.inference_service_id, sm.model_version_id, sm.last_known_state,
			isvc.name AS inference_service_name,
			mv.name AS model_version_name
		FROM serve_model sm
		JOIN inference_service isvc ON isvc.id = sm.inference_service_id
		JOIN model_version mv ON mv.id = sm.model_version_id
		WHERE sm.model_version_id = $1 AND sm.project_id = $2
		ORDER BY sm.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, versionID, projectID)
	if err != nil {
		return nil, fmt.Errorf("find serve models by model version: %w", err)
	}
	defer rows.Close()

	var serveModels []*domain.ServeModel
	for rows.Next() {
		sm, err := r.scanServeModelFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan serve model row: %w", err)
		}
		serveModels = append(serveModels, sm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serve model rows: %w", err)
	}

	return serveModels, nil
}

func (r *serveModelRepo) scanServeModel(row pgx.Row) (*domain.ServeModel, error) {
	sm := &domain.ServeModel{}
	var state string

	err := row.Scan(
		&sm.ID, &sm.CreatedAt, &sm.UpdatedAt, &sm.ProjectID,
		&sm.InferenceServiceID, &sm.ModelVersionID, &state,
		&sm.InferenceServiceName, &sm.ModelVersionName,
	)
	if err != nil {
		return nil, err
	}

	sm.LastKnownState = domain.ServeModelState(state)
	return sm, nil
}

func (r *serveModelRepo) scanServeModelFromRows(rows pgx.Rows) (*domain.ServeModel, error) {
	sm := &domain.ServeModel{}
	var state string

	err := rows.Scan(
		&sm.ID, &sm.CreatedAt, &sm.UpdatedAt, &sm.ProjectID,
		&sm.InferenceServiceID, &sm.ModelVersionID, &state,
		&sm.InferenceServiceName, &sm.ModelVersionName,
	)
	if err != nil {
		return nil, err
	}

	sm.LastKnownState = domain.ServeModelState(state)
	return sm, nil
}

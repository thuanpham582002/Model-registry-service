# Phase 2: Repository Layer (Revised)

## Objective
Implement PostgreSQL repository for ServingEnvironment, InferenceService, and ServeModel with proper error handling.

## Tasks

### 2.1 ServingEnvironment Repository

**File**: `internal/repository/serving-env-repo.go`

```go
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"model-registry-service/internal/domain"
)

type servingEnvRepo struct {
	pool *pgxpool.Pool
}

func NewServingEnvironmentRepository(pool *pgxpool.Pool) domain.ServingEnvironmentRepository {
	return &servingEnvRepo{pool: pool}
}

func (r *servingEnvRepo) Create(ctx context.Context, env *domain.ServingEnvironment) error {
	query := `
		INSERT INTO serving_environment (id, project_id, name, description, external_id)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, env.ID, env.ProjectID, env.Name, env.Description, env.ExternalID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrServingEnvNameConflict
		}
		return fmt.Errorf("create serving environment: %w", err)
	}
	return nil
}

func (r *servingEnvRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServingEnvironment, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, COALESCE(external_id, '') as external_id
		FROM serving_environment
		WHERE id = $1 AND project_id = $2
	`
	env := &domain.ServingEnvironment{}
	err := r.pool.QueryRow(ctx, query, id, projectID).Scan(
		&env.ID, &env.CreatedAt, &env.UpdatedAt, &env.ProjectID,
		&env.Name, &env.Description, &env.ExternalID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrServingEnvNotFound
		}
		return nil, fmt.Errorf("get serving environment: %w", err)
	}
	return env, nil
}

func (r *servingEnvRepo) GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.ServingEnvironment, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, COALESCE(external_id, '') as external_id
		FROM serving_environment
		WHERE project_id = $1 AND name = $2
	`
	env := &domain.ServingEnvironment{}
	err := r.pool.QueryRow(ctx, query, projectID, name).Scan(
		&env.ID, &env.CreatedAt, &env.UpdatedAt, &env.ProjectID,
		&env.Name, &env.Description, &env.ExternalID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrServingEnvNotFound
		}
		return nil, fmt.Errorf("get serving environment by name: %w", err)
	}
	return env, nil
}

func (r *servingEnvRepo) Update(ctx context.Context, projectID uuid.UUID, env *domain.ServingEnvironment) error {
	query := `
		UPDATE serving_environment
		SET name = $1, description = $2, external_id = $3, updated_at = NOW()
		WHERE id = $4 AND project_id = $5
	`
	result, err := r.pool.Exec(ctx, query, env.Name, env.Description, env.ExternalID, env.ID, projectID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrServingEnvNameConflict
		}
		return fmt.Errorf("update serving environment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrServingEnvNotFound
	}
	return nil
}

func (r *servingEnvRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	query := `DELETE FROM serving_environment WHERE id = $1 AND project_id = $2`
	result, err := r.pool.Exec(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete serving environment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrServingEnvNotFound
	}
	return nil
}

func (r *servingEnvRepo) List(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*domain.ServingEnvironment, int, error) {
	// Count
	var total int
	countQuery := `SELECT COUNT(*) FROM serving_environment WHERE project_id = $1`
	if err := r.pool.QueryRow(ctx, countQuery, projectID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count serving environments: %w", err)
	}

	// List
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, COALESCE(external_id, '') as external_id
		FROM serving_environment
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, projectID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list serving environments: %w", err)
	}
	defer rows.Close()

	var envs []*domain.ServingEnvironment
	for rows.Next() {
		env := &domain.ServingEnvironment{}
		// FIX: Proper error handling for scan
		if err := rows.Scan(
			&env.ID, &env.CreatedAt, &env.UpdatedAt, &env.ProjectID,
			&env.Name, &env.Description, &env.ExternalID,
		); err != nil {
			return nil, 0, fmt.Errorf("scan serving environment: %w", err)
		}
		envs = append(envs, env)
	}

	// FIX: Check rows.Err() after iteration
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate serving environments: %w", err)
	}

	return envs, total, nil
}
```

### 2.2 InferenceService Repository

**File**: `internal/repository/inference-service-repo.go`

```go
package repository

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

	"model-registry-service/internal/domain"
)

type inferenceServiceRepo struct {
	pool *pgxpool.Pool
}

func NewInferenceServiceRepository(pool *pgxpool.Pool) domain.InferenceServiceRepository {
	return &inferenceServiceRepo{pool: pool}
}

func (r *inferenceServiceRepo) Create(ctx context.Context, isvc *domain.InferenceService) error {
	labelsJSON, err := json.Marshal(isvc.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		INSERT INTO inference_service
			(id, project_id, name, external_id, serving_environment_id,
			 registered_model_id, model_version_id, desired_state, current_state,
			 runtime, url, last_error, labels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err = r.pool.Exec(ctx, query,
		isvc.ID, isvc.ProjectID, isvc.Name, isvc.ExternalID,
		isvc.ServingEnvironmentID, isvc.RegisteredModelID, isvc.ModelVersionID,
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
		SELECT id, created_at, updated_at, project_id, name, COALESCE(external_id, '') as external_id,
			   serving_environment_id, registered_model_id, model_version_id,
			   desired_state, current_state, runtime, COALESCE(url, '') as url,
			   COALESCE(last_error, '') as last_error, COALESCE(labels, '{}') as labels
		FROM inference_service
		WHERE id = $1 AND project_id = $2
	`
	return r.scanOne(r.pool.QueryRow(ctx, query, id, projectID))
}

func (r *inferenceServiceRepo) GetByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*domain.InferenceService, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, COALESCE(external_id, '') as external_id,
			   serving_environment_id, registered_model_id, model_version_id,
			   desired_state, current_state, runtime, COALESCE(url, '') as url,
			   COALESCE(last_error, '') as last_error, COALESCE(labels, '{}') as labels
		FROM inference_service
		WHERE external_id = $1 AND project_id = $2
	`
	return r.scanOne(r.pool.QueryRow(ctx, query, externalID, projectID))
}

func (r *inferenceServiceRepo) GetByName(ctx context.Context, projectID, envID uuid.UUID, name string) (*domain.InferenceService, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, COALESCE(external_id, '') as external_id,
			   serving_environment_id, registered_model_id, model_version_id,
			   desired_state, current_state, runtime, COALESCE(url, '') as url,
			   COALESCE(last_error, '') as last_error, COALESCE(labels, '{}') as labels
		FROM inference_service
		WHERE project_id = $1 AND serving_environment_id = $2 AND name = $3
	`
	return r.scanOne(r.pool.QueryRow(ctx, query, projectID, envID, name))
}

func (r *inferenceServiceRepo) Update(ctx context.Context, projectID uuid.UUID, isvc *domain.InferenceService) error {
	labelsJSON, err := json.Marshal(isvc.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		UPDATE inference_service
		SET name = $1, external_id = $2, model_version_id = $3,
			desired_state = $4, current_state = $5, runtime = $6,
			url = $7, last_error = $8, labels = $9, updated_at = NOW()
		WHERE id = $10 AND project_id = $11
	`
	result, err := r.pool.Exec(ctx, query,
		isvc.Name, isvc.ExternalID, isvc.ModelVersionID,
		string(isvc.DesiredState), string(isvc.CurrentState), isvc.Runtime,
		isvc.URL, isvc.LastError, labelsJSON, isvc.ID, projectID,
	)
	if err != nil {
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

func (r *inferenceServiceRepo) List(ctx context.Context, filter domain.InferenceServiceFilter) ([]*domain.InferenceService, int, error) {
	conditions := []string{"project_id = $1"}
	args := []interface{}{filter.ProjectID}
	argPos := 2

	if filter.ServingEnvironmentID != nil {
		conditions = append(conditions, fmt.Sprintf("serving_environment_id = $%d", argPos))
		args = append(args, *filter.ServingEnvironmentID)
		argPos++
	}
	if filter.RegisteredModelID != nil {
		conditions = append(conditions, fmt.Sprintf("registered_model_id = $%d", argPos))
		args = append(args, *filter.RegisteredModelID)
		argPos++
	}
	if filter.ModelVersionID != nil {
		conditions = append(conditions, fmt.Sprintf("model_version_id = $%d", argPos))
		args = append(args, *filter.ModelVersionID)
		argPos++
	}
	if filter.State != "" {
		conditions = append(conditions, fmt.Sprintf("current_state = $%d", argPos))
		args = append(args, filter.State)
		argPos++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM inference_service WHERE %s", whereClause)
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count inference services: %w", err)
	}

	// List
	orderBy := "created_at DESC"
	if filter.SortBy != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf("%s %s", filter.SortBy, dir)
	}

	query := fmt.Sprintf(`
		SELECT id, created_at, updated_at, project_id, name, COALESCE(external_id, '') as external_id,
			   serving_environment_id, registered_model_id, model_version_id,
			   desired_state, current_state, runtime, COALESCE(url, '') as url,
			   COALESCE(last_error, '') as last_error, COALESCE(labels, '{}') as labels
		FROM inference_service
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

	// FIX: Proper error handling with scanMany
	result, err := r.scanMany(rows)
	if err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

func (r *inferenceServiceRepo) CountByModel(ctx context.Context, projectID, modelID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM inference_service
		WHERE project_id = $1 AND registered_model_id = $2 AND current_state = 'DEPLOYED'
	`
	var count int
	err := r.pool.QueryRow(ctx, query, projectID, modelID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count by model: %w", err)
	}
	return count, nil
}

func (r *inferenceServiceRepo) scanOne(row pgx.Row) (*domain.InferenceService, error) {
	isvc := &domain.InferenceService{}
	var labelsJSON []byte

	err := row.Scan(
		&isvc.ID, &isvc.CreatedAt, &isvc.UpdatedAt, &isvc.ProjectID,
		&isvc.Name, &isvc.ExternalID, &isvc.ServingEnvironmentID,
		&isvc.RegisteredModelID, &isvc.ModelVersionID,
		&isvc.DesiredState, &isvc.CurrentState, &isvc.Runtime,
		&isvc.URL, &isvc.LastError, &labelsJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInferenceServiceNotFound
		}
		return nil, fmt.Errorf("scan inference service: %w", err)
	}

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &isvc.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}

	return isvc, nil
}

// FIX: scanMany now returns error for proper error handling
func (r *inferenceServiceRepo) scanMany(rows pgx.Rows) ([]*domain.InferenceService, error) {
	var result []*domain.InferenceService
	for rows.Next() {
		isvc := &domain.InferenceService{}
		var labelsJSON []byte

		// FIX: Check scan error
		if err := rows.Scan(
			&isvc.ID, &isvc.CreatedAt, &isvc.UpdatedAt, &isvc.ProjectID,
			&isvc.Name, &isvc.ExternalID, &isvc.ServingEnvironmentID,
			&isvc.RegisteredModelID, &isvc.ModelVersionID,
			&isvc.DesiredState, &isvc.CurrentState, &isvc.Runtime,
			&isvc.URL, &isvc.LastError, &labelsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan inference service row: %w", err)
		}

		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &isvc.Labels); err != nil {
				return nil, fmt.Errorf("unmarshal labels: %w", err)
			}
		}
		result = append(result, isvc)
	}

	// FIX: Check rows.Err() after iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inference services: %w", err)
	}

	return result, nil
}
```

### 2.3 ServeModel Repository

**File**: `internal/repository/serve-model-repo.go`

```go
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"model-registry-service/internal/domain"
)

type serveModelRepo struct {
	pool *pgxpool.Pool
}

func NewServeModelRepository(pool *pgxpool.Pool) domain.ServeModelRepository {
	return &serveModelRepo{pool: pool}
}

// FIX: ServeModel now includes project_id
func (r *serveModelRepo) Create(ctx context.Context, sm *domain.ServeModel) error {
	query := `
		INSERT INTO serve_model (id, project_id, inference_service_id, model_version_id, last_known_state)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, sm.ID, sm.ProjectID, sm.InferenceServiceID, sm.ModelVersionID, string(sm.LastKnownState))
	if err != nil {
		return fmt.Errorf("create serve model: %w", err)
	}
	return nil
}

// FIX: Added projectID parameter for tenant isolation
func (r *serveModelRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServeModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, inference_service_id, model_version_id, last_known_state
		FROM serve_model WHERE id = $1 AND project_id = $2
	`
	sm := &domain.ServeModel{}
	err := r.pool.QueryRow(ctx, query, id, projectID).Scan(
		&sm.ID, &sm.CreatedAt, &sm.UpdatedAt, &sm.ProjectID,
		&sm.InferenceServiceID, &sm.ModelVersionID, &sm.LastKnownState,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrServeModelNotFound
		}
		return nil, fmt.Errorf("get serve model: %w", err)
	}
	return sm, nil
}

// FIX: Added projectID parameter for tenant isolation
func (r *serveModelRepo) Update(ctx context.Context, projectID uuid.UUID, sm *domain.ServeModel) error {
	query := `
		UPDATE serve_model
		SET last_known_state = $1, updated_at = NOW()
		WHERE id = $2 AND project_id = $3
	`
	result, err := r.pool.Exec(ctx, query, string(sm.LastKnownState), sm.ID, projectID)
	if err != nil {
		return fmt.Errorf("update serve model: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrServeModelNotFound
	}
	return nil
}

// FIX: Added projectID parameter for tenant isolation
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

// FIX: Added projectID parameter and proper error handling
func (r *serveModelRepo) ListByInferenceService(ctx context.Context, projectID, isvcID uuid.UUID) ([]*domain.ServeModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, inference_service_id, model_version_id, last_known_state
		FROM serve_model
		WHERE inference_service_id = $1 AND project_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, isvcID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list serve models: %w", err)
	}
	defer rows.Close()

	var result []*domain.ServeModel
	for rows.Next() {
		sm := &domain.ServeModel{}
		// FIX: Check scan error
		if err := rows.Scan(
			&sm.ID, &sm.CreatedAt, &sm.UpdatedAt, &sm.ProjectID,
			&sm.InferenceServiceID, &sm.ModelVersionID, &sm.LastKnownState,
		); err != nil {
			return nil, fmt.Errorf("scan serve model row: %w", err)
		}
		result = append(result, sm)
	}

	// FIX: Check rows.Err() after iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serve models: %w", err)
	}

	return result, nil
}
```

## Checklist

- [ ] Create `internal/repository/serving-env-repo.go`
- [ ] Create `internal/repository/inference-service-repo.go`
- [ ] Create `internal/repository/serve-model-repo.go`
- [ ] Verify all scan operations have error checking
- [ ] Verify all loops check `rows.Err()` after iteration
- [ ] Write unit tests with mock DB
- [ ] Run integration tests against PostgreSQL

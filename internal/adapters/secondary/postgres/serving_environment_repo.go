package postgres

import (
	"context"
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

type servingEnvironmentRepo struct {
	pool *pgxpool.Pool
}

// NewServingEnvironmentRepository creates a new ServingEnvironmentRepository
func NewServingEnvironmentRepository(pool *pgxpool.Pool) output.ServingEnvironmentRepository {
	return &servingEnvironmentRepo{pool: pool}
}

func (r *servingEnvironmentRepo) Create(ctx context.Context, env *domain.ServingEnvironment) error {
	query := `
		INSERT INTO serving_environment
			(id, created_at, updated_at, project_id, name, description, external_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		env.ID, env.CreatedAt, env.UpdatedAt,
		env.ProjectID, env.Name, env.Description, env.ExternalID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrServingEnvNameConflict
		}
		return fmt.Errorf("create serving environment: %w", err)
	}
	return nil
}

func (r *servingEnvironmentRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.ServingEnvironment, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, external_id
		FROM serving_environment
		WHERE id = $1 AND project_id = $2
	`

	env, err := r.scanEnv(r.pool.QueryRow(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrServingEnvNotFound
		}
		return nil, fmt.Errorf("get serving environment by id: %w", err)
	}
	return env, nil
}

func (r *servingEnvironmentRepo) GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.ServingEnvironment, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, external_id
		FROM serving_environment
		WHERE project_id = $1 AND name = $2
	`

	env, err := r.scanEnv(r.pool.QueryRow(ctx, query, projectID, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrServingEnvNotFound
		}
		return nil, fmt.Errorf("get serving environment by name: %w", err)
	}
	return env, nil
}

func (r *servingEnvironmentRepo) Update(ctx context.Context, projectID uuid.UUID, env *domain.ServingEnvironment) error {
	query := `
		UPDATE serving_environment
		SET name = $1, description = $2, external_id = $3, updated_at = NOW()
		WHERE id = $4 AND project_id = $5
	`

	result, err := r.pool.Exec(ctx, query,
		env.Name, env.Description, env.ExternalID,
		env.ID, projectID,
	)
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

func (r *servingEnvironmentRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	query := `DELETE FROM serving_environment WHERE id = $1 AND project_id = $2`

	result, err := r.pool.Exec(ctx, query, id, projectID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			// Foreign key violation - has inference services
			return domain.ErrServingEnvHasDeployments
		}
		return fmt.Errorf("delete serving environment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrServingEnvNotFound
	}
	return nil
}

func (r *servingEnvironmentRepo) List(ctx context.Context, filter output.ServingEnvironmentFilter) ([]*domain.ServingEnvironment, int, error) {
	conditions := []string{"project_id = $1"}
	args := []interface{}{filter.ProjectID}
	argPos := 2

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argPos))
		args = append(args, "%"+filter.Search+"%")
		argPos++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM serving_environment WHERE %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count serving environments: %w", err)
	}

	// Order
	orderBy := "created_at DESC"
	if filter.SortBy != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf("%s %s", filter.SortBy, dir)
	}

	query := fmt.Sprintf(`
		SELECT id, created_at, updated_at, project_id, name, description, external_id
		FROM serving_environment
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argPos, argPos+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list serving environments: %w", err)
	}
	defer rows.Close()

	var envs []*domain.ServingEnvironment
	for rows.Next() {
		env, err := r.scanEnvFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan serving environment row: %w", err)
		}
		envs = append(envs, env)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate serving environment rows: %w", err)
	}

	return envs, total, nil
}

func (r *servingEnvironmentRepo) scanEnv(row pgx.Row) (*domain.ServingEnvironment, error) {
	env := &domain.ServingEnvironment{}
	err := row.Scan(
		&env.ID, &env.CreatedAt, &env.UpdatedAt,
		&env.ProjectID, &env.Name, &env.Description, &env.ExternalID,
	)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (r *servingEnvironmentRepo) scanEnvFromRows(rows pgx.Rows) (*domain.ServingEnvironment, error) {
	env := &domain.ServingEnvironment{}
	err := rows.Scan(
		&env.ID, &env.CreatedAt, &env.UpdatedAt,
		&env.ProjectID, &env.Name, &env.Description, &env.ExternalID,
	)
	if err != nil {
		return nil, err
	}
	return env, nil
}

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
	ports "model-registry-service/internal/core/ports/output"
)

type trafficConfigRepo struct {
	pool *pgxpool.Pool
}

// NewTrafficConfigRepository creates a new traffic config repository
func NewTrafficConfigRepository(pool *pgxpool.Pool) ports.TrafficConfigRepository {
	return &trafficConfigRepo{pool: pool}
}

func (r *trafficConfigRepo) Create(ctx context.Context, config *domain.TrafficConfig) error {
	query := `
		INSERT INTO traffic_config (id, project_id, inference_service_id, strategy, ai_gateway_route_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		config.ID,
		config.ProjectID,
		config.InferenceServiceID,
		string(config.Strategy),
		nullableString(config.AIGatewayRouteName),
		config.Status,
		config.CreatedAt,
		config.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert traffic_config: %w", err)
	}
	return nil
}

func (r *trafficConfigRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.TrafficConfig, error) {
	query := `
		SELECT tc.id, tc.created_at, tc.updated_at, tc.project_id, tc.inference_service_id,
		       tc.strategy, tc.ai_gateway_route_name, tc.status,
		       COALESCE(isvc.name, '') as inference_service_name
		FROM traffic_config tc
		LEFT JOIN inference_service isvc ON tc.inference_service_id = isvc.id
		WHERE tc.id = $1 AND tc.project_id = $2
	`
	config, err := r.scanConfig(r.pool.QueryRow(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTrafficConfigNotFound
		}
		return nil, fmt.Errorf("get traffic_config by id: %w", err)
	}
	return config, nil
}

func (r *trafficConfigRepo) GetByISVC(ctx context.Context, projectID, isvcID uuid.UUID) (*domain.TrafficConfig, error) {
	query := `
		SELECT tc.id, tc.created_at, tc.updated_at, tc.project_id, tc.inference_service_id,
		       tc.strategy, tc.ai_gateway_route_name, tc.status,
		       COALESCE(isvc.name, '') as inference_service_name
		FROM traffic_config tc
		LEFT JOIN inference_service isvc ON tc.inference_service_id = isvc.id
		WHERE tc.inference_service_id = $1 AND tc.project_id = $2
	`
	config, err := r.scanConfig(r.pool.QueryRow(ctx, query, isvcID, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTrafficConfigNotFound
		}
		return nil, fmt.Errorf("get traffic_config by isvc: %w", err)
	}
	return config, nil
}

func (r *trafficConfigRepo) Update(ctx context.Context, projectID uuid.UUID, config *domain.TrafficConfig) error {
	query := `
		UPDATE traffic_config
		SET strategy = $1, ai_gateway_route_name = $2, status = $3, updated_at = NOW()
		WHERE id = $4 AND project_id = $5
	`
	result, err := r.pool.Exec(ctx, query,
		string(config.Strategy),
		nullableString(config.AIGatewayRouteName),
		config.Status,
		config.ID,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("update traffic_config: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrTrafficConfigNotFound
	}
	return nil
}

func (r *trafficConfigRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	query := `DELETE FROM traffic_config WHERE id = $1 AND project_id = $2`
	result, err := r.pool.Exec(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete traffic_config: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrTrafficConfigNotFound
	}
	return nil
}

func (r *trafficConfigRepo) List(ctx context.Context, filter ports.TrafficConfigFilter) ([]*domain.TrafficConfig, int, error) {
	// Build WHERE clause
	conditions := []string{"tc.project_id = $1"}
	args := []interface{}{filter.ProjectID}
	argIdx := 2

	if filter.InferenceServiceID != nil {
		conditions = append(conditions, fmt.Sprintf("tc.inference_service_id = $%d", argIdx))
		args = append(args, *filter.InferenceServiceID)
		argIdx++
	}
	if filter.Strategy != "" {
		conditions = append(conditions, fmt.Sprintf("tc.strategy = $%d", argIdx))
		args = append(args, filter.Strategy)
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("tc.status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count total
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM traffic_config tc WHERE %s`, whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count traffic_configs: %w", err)
	}

	// Query with pagination
	dataQuery := fmt.Sprintf(`
		SELECT tc.id, tc.created_at, tc.updated_at, tc.project_id, tc.inference_service_id,
		       tc.strategy, tc.ai_gateway_route_name, tc.status,
		       COALESCE(isvc.name, '') as inference_service_name
		FROM traffic_config tc
		LEFT JOIN inference_service isvc ON tc.inference_service_id = isvc.id
		WHERE %s
		ORDER BY tc.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query traffic_configs: %w", err)
	}
	defer rows.Close()

	var configs []*domain.TrafficConfig
	for rows.Next() {
		config, err := r.scanConfigFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan traffic_config: %w", err)
		}
		configs = append(configs, config)
	}

	return configs, total, nil
}

func (r *trafficConfigRepo) scanConfig(row pgx.Row) (*domain.TrafficConfig, error) {
	var config domain.TrafficConfig
	var routeName *string
	var isvcName string

	err := row.Scan(
		&config.ID, &config.CreatedAt, &config.UpdatedAt, &config.ProjectID, &config.InferenceServiceID,
		&config.Strategy, &routeName, &config.Status, &isvcName,
	)
	if err != nil {
		return nil, err
	}

	if routeName != nil {
		config.AIGatewayRouteName = *routeName
	}
	config.InferenceServiceName = isvcName

	return &config, nil
}

func (r *trafficConfigRepo) scanConfigFromRows(rows pgx.Rows) (*domain.TrafficConfig, error) {
	var config domain.TrafficConfig
	var routeName *string
	var isvcName string

	err := rows.Scan(
		&config.ID, &config.CreatedAt, &config.UpdatedAt, &config.ProjectID, &config.InferenceServiceID,
		&config.Strategy, &routeName, &config.Status, &isvcName,
	)
	if err != nil {
		return nil, err
	}

	if routeName != nil {
		config.AIGatewayRouteName = *routeName
	}
	config.InferenceServiceName = isvcName

	return &config, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

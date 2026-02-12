package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"model-registry-service/internal/core/domain"
	ports "model-registry-service/internal/core/ports/output"
)

type virtualModelRepo struct {
	pool *pgxpool.Pool
}

// NewVirtualModelRepository creates a new virtual model repository
func NewVirtualModelRepository(pool *pgxpool.Pool) ports.VirtualModelRepository {
	return &virtualModelRepo{pool: pool}
}

// ============================================================================
// Virtual Model CRUD
// ============================================================================

func (r *virtualModelRepo) Create(ctx context.Context, vm *domain.VirtualModel) error {
	query := `
		INSERT INTO virtual_model (id, project_id, name, description, ai_gateway_route_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		vm.ID,
		vm.ProjectID,
		vm.Name,
		vm.Description,        // NOT NULL column with default
		vm.AIGatewayRouteName, // NOT NULL column with default
		vm.Status,
		vm.CreatedAt,
		vm.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert virtual_model: %w", err)
	}
	return nil
}

func (r *virtualModelRepo) GetByID(ctx context.Context, projectID, id uuid.UUID) (*domain.VirtualModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, ai_gateway_route_name, status
		FROM virtual_model
		WHERE id = $1 AND project_id = $2
	`
	vm, err := r.scanVirtualModel(r.pool.QueryRow(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVirtualModelNotFound
		}
		return nil, fmt.Errorf("get virtual_model by id: %w", err)
	}

	// Load backends
	backends, err := r.ListBackends(ctx, vm.ID)
	if err != nil {
		return nil, err
	}
	vm.Backends = backends

	return vm, nil
}

func (r *virtualModelRepo) GetByName(ctx context.Context, projectID uuid.UUID, name string) (*domain.VirtualModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, ai_gateway_route_name, status
		FROM virtual_model
		WHERE project_id = $1 AND name = $2
	`
	vm, err := r.scanVirtualModel(r.pool.QueryRow(ctx, query, projectID, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVirtualModelNotFound
		}
		return nil, fmt.Errorf("get virtual_model by name: %w", err)
	}

	// Load backends
	backends, err := r.ListBackends(ctx, vm.ID)
	if err != nil {
		return nil, err
	}
	vm.Backends = backends

	return vm, nil
}

func (r *virtualModelRepo) Update(ctx context.Context, projectID uuid.UUID, vm *domain.VirtualModel) error {
	query := `
		UPDATE virtual_model
		SET name = $1, description = $2, ai_gateway_route_name = $3, status = $4, updated_at = NOW()
		WHERE id = $5 AND project_id = $6
	`
	result, err := r.pool.Exec(ctx, query,
		vm.Name,
		vm.Description,        // NOT NULL column
		vm.AIGatewayRouteName, // NOT NULL column
		vm.Status,
		vm.ID,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("update virtual_model: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrVirtualModelNotFound
	}
	return nil
}

func (r *virtualModelRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	// Delete backends first (cascade)
	_, err := r.pool.Exec(ctx, `DELETE FROM virtual_model_backend WHERE virtual_model_id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete virtual_model_backends: %w", err)
	}

	query := `DELETE FROM virtual_model WHERE id = $1 AND project_id = $2`
	result, err := r.pool.Exec(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete virtual_model: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrVirtualModelNotFound
	}
	return nil
}

func (r *virtualModelRepo) List(ctx context.Context, projectID uuid.UUID) ([]*domain.VirtualModel, error) {
	query := `
		SELECT id, created_at, updated_at, project_id, name, description, ai_gateway_route_name, status
		FROM virtual_model
		WHERE project_id = $1
		ORDER BY name
	`
	rows, err := r.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("query virtual_models: %w", err)
	}
	defer rows.Close()

	var models []*domain.VirtualModel
	for rows.Next() {
		vm, err := r.scanVirtualModelFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan virtual_model: %w", err)
		}

		// Load backends for each model
		backends, _ := r.ListBackends(ctx, vm.ID)
		vm.Backends = backends

		models = append(models, vm)
	}

	return models, nil
}

// ============================================================================
// Backend Operations
// ============================================================================

func (r *virtualModelRepo) CreateBackend(ctx context.Context, backend *domain.VirtualModelBackend) error {
	query := `
		INSERT INTO virtual_model_backend (id, virtual_model_id, ai_service_backend_name, ai_service_backend_namespace, model_name_override, weight, priority, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		backend.ID,
		backend.VirtualModelID,
		backend.AIServiceBackendName,
		nullableString(backend.AIServiceBackendNamespace),
		backend.ModelNameOverride,
		backend.Weight,
		backend.Priority,
		backend.Status,
		backend.CreatedAt,
		backend.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert virtual_model_backend: %w", err)
	}
	return nil
}

func (r *virtualModelRepo) UpdateBackend(ctx context.Context, backend *domain.VirtualModelBackend) error {
	query := `
		UPDATE virtual_model_backend
		SET weight = $1, priority = $2, status = $3, updated_at = NOW()
		WHERE id = $4
	`
	result, err := r.pool.Exec(ctx, query,
		backend.Weight,
		backend.Priority,
		backend.Status,
		backend.ID,
	)
	if err != nil {
		return fmt.Errorf("update virtual_model_backend: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrBackendNotFound
	}
	return nil
}

func (r *virtualModelRepo) DeleteBackend(ctx context.Context, backendID uuid.UUID) error {
	query := `DELETE FROM virtual_model_backend WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, backendID)
	if err != nil {
		return fmt.Errorf("delete virtual_model_backend: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrBackendNotFound
	}
	return nil
}

func (r *virtualModelRepo) ListBackends(ctx context.Context, vmID uuid.UUID) ([]*domain.VirtualModelBackend, error) {
	query := `
		SELECT id, created_at, updated_at, virtual_model_id, ai_service_backend_name,
		       ai_service_backend_namespace, model_name_override, weight, priority, status
		FROM virtual_model_backend
		WHERE virtual_model_id = $1
		ORDER BY priority, weight DESC
	`
	rows, err := r.pool.Query(ctx, query, vmID)
	if err != nil {
		return nil, fmt.Errorf("query virtual_model_backends: %w", err)
	}
	defer rows.Close()

	var backends []*domain.VirtualModelBackend
	for rows.Next() {
		backend, err := r.scanBackendFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan virtual_model_backend: %w", err)
		}
		backends = append(backends, backend)
	}

	return backends, nil
}

// ============================================================================
// Scan helpers
// ============================================================================

func (r *virtualModelRepo) scanVirtualModel(row pgx.Row) (*domain.VirtualModel, error) {
	var vm domain.VirtualModel
	var description, routeName *string

	err := row.Scan(
		&vm.ID, &vm.CreatedAt, &vm.UpdatedAt, &vm.ProjectID, &vm.Name,
		&description, &routeName, &vm.Status,
	)
	if err != nil {
		return nil, err
	}

	if description != nil {
		vm.Description = *description
	}
	if routeName != nil {
		vm.AIGatewayRouteName = *routeName
	}

	return &vm, nil
}

func (r *virtualModelRepo) scanVirtualModelFromRows(rows pgx.Rows) (*domain.VirtualModel, error) {
	var vm domain.VirtualModel
	var description, routeName *string

	err := rows.Scan(
		&vm.ID, &vm.CreatedAt, &vm.UpdatedAt, &vm.ProjectID, &vm.Name,
		&description, &routeName, &vm.Status,
	)
	if err != nil {
		return nil, err
	}

	if description != nil {
		vm.Description = *description
	}
	if routeName != nil {
		vm.AIGatewayRouteName = *routeName
	}

	return &vm, nil
}

func (r *virtualModelRepo) scanBackendFromRows(rows pgx.Rows) (*domain.VirtualModelBackend, error) {
	var backend domain.VirtualModelBackend
	var namespace *string

	err := rows.Scan(
		&backend.ID, &backend.CreatedAt, &backend.UpdatedAt, &backend.VirtualModelID, &backend.AIServiceBackendName,
		&namespace, &backend.ModelNameOverride, &backend.Weight, &backend.Priority, &backend.Status,
	)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		backend.AIServiceBackendNamespace = *namespace
	}

	return &backend, nil
}

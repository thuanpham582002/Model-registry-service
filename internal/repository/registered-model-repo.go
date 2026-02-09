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

type registeredModelRepo struct {
	pool *pgxpool.Pool
}

func NewRegisteredModelRepository(pool *pgxpool.Pool) domain.RegisteredModelRepository {
	return &registeredModelRepo{pool: pool}
}

func (r *registeredModelRepo) Create(ctx context.Context, model *domain.RegisteredModel) error {
	tagsJSON, err := json.Marshal(model.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	labelsJSON, err := json.Marshal(model.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		INSERT INTO model_registry_registered_model
			(id, created_at, updated_at, project_id, owner_id, name, slug,
			 description, region_id, model_type, model_size, state,
			 deployment_status, tags, labels, parent_model_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`

	_, err = r.pool.Exec(ctx, query,
		model.ID, model.CreatedAt, model.UpdatedAt,
		model.ProjectID, model.OwnerID, model.Name, model.Slug,
		model.Description, model.RegionID, string(model.ModelType),
		model.ModelSize, string(model.State), string(model.DeploymentStatus),
		tagsJSON, labelsJSON, model.ParentModelID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrModelNameConflict
		}
		return fmt.Errorf("create registered model: %w", err)
	}
	return nil
}

func (r *registeredModelRepo) GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.RegisteredModel, error) {
	query := `
		SELECT
			rm.id, rm.created_at, rm.updated_at, rm.project_id, rm.owner_id,
			rm.name, rm.slug, rm.description, rm.region_id,
			rm.model_type, rm.model_size, rm.state, rm.deployment_status,
			rm.tags, rm.labels, rm.parent_model_id,
			COALESCE(tu.email, '') AS owner_email,
			COALESCE(oreg.name, '') AS region_name,
			(SELECT COUNT(*) FROM model_registry_model_version mv WHERE mv.registered_model_id = rm.id) AS version_count
		FROM model_registry_registered_model rm
		LEFT JOIN tenant_user tu ON tu.id = rm.owner_id
		LEFT JOIN organization_region oreg ON oreg.id = rm.region_id
		WHERE rm.id = $1 AND rm.project_id = $2
	`

	model, err := r.scanModel(r.pool.QueryRow(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrModelNotFound
		}
		return nil, fmt.Errorf("get registered model by id: %w", err)
	}

	if err := r.loadVersionMeta(ctx, model); err != nil {
		return nil, err
	}

	return model, nil
}

func (r *registeredModelRepo) GetByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string) (*domain.RegisteredModel, error) {
	conditions := []string{"rm.project_id = $1"}
	args := []interface{}{projectID}
	argPos := 2

	if name != "" {
		conditions = append(conditions, fmt.Sprintf("rm.name = $%d", argPos))
		args = append(args, name)
		argPos++
	}
	if externalID != "" {
		conditions = append(conditions, fmt.Sprintf("rm.id::text = $%d", argPos))
		args = append(args, externalID)
		argPos++
	}

	if len(conditions) == 1 {
		return nil, domain.ErrModelNotFound
	}

	query := fmt.Sprintf(`
		SELECT
			rm.id, rm.created_at, rm.updated_at, rm.project_id, rm.owner_id,
			rm.name, rm.slug, rm.description, rm.region_id,
			rm.model_type, rm.model_size, rm.state, rm.deployment_status,
			rm.tags, rm.labels, rm.parent_model_id,
			COALESCE(tu.email, '') AS owner_email,
			COALESCE(oreg.name, '') AS region_name,
			(SELECT COUNT(*) FROM model_registry_model_version mv WHERE mv.registered_model_id = rm.id) AS version_count
		FROM model_registry_registered_model rm
		LEFT JOIN tenant_user tu ON tu.id = rm.owner_id
		LEFT JOIN organization_region oreg ON oreg.id = rm.region_id
		WHERE %s
		LIMIT 1
	`, strings.Join(conditions, " AND "))

	model, err := r.scanModel(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrModelNotFound
		}
		return nil, fmt.Errorf("get registered model by params: %w", err)
	}

	if err := r.loadVersionMeta(ctx, model); err != nil {
		return nil, err
	}

	return model, nil
}

func (r *registeredModelRepo) Update(ctx context.Context, projectID uuid.UUID, model *domain.RegisteredModel) error {
	tagsJSON, err := json.Marshal(model.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	labelsJSON, err := json.Marshal(model.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		UPDATE model_registry_registered_model
		SET name=$1, description=$2, model_type=$3, model_size=$4,
			state=$5, deployment_status=$6, tags=$7, labels=$8, updated_at=NOW()
		WHERE id=$9 AND project_id=$10
	`
	result, err := r.pool.Exec(ctx, query,
		model.Name, model.Description, string(model.ModelType),
		model.ModelSize, string(model.State), string(model.DeploymentStatus),
		tagsJSON, labelsJSON, model.ID, projectID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrModelNameConflict
		}
		return fmt.Errorf("update registered model: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrModelNotFound
	}
	return nil
}

func (r *registeredModelRepo) Delete(ctx context.Context, projectID uuid.UUID, id uuid.UUID) error {
	query := `DELETE FROM model_registry_registered_model WHERE id = $1 AND project_id = $2`
	result, err := r.pool.Exec(ctx, query, id, projectID)
	if err != nil {
		return fmt.Errorf("delete registered model: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrModelNotFound
	}
	return nil
}

func (r *registeredModelRepo) List(ctx context.Context, filter domain.ListFilter) ([]*domain.RegisteredModel, int, error) {
	conditions := []string{"rm.project_id = $1"}
	args := []interface{}{filter.ProjectID}
	argPos := 2

	if filter.State != "" {
		conditions = append(conditions, fmt.Sprintf("rm.state = $%d", argPos))
		args = append(args, filter.State)
		argPos++
	}
	if filter.ModelType != "" {
		conditions = append(conditions, fmt.Sprintf("rm.model_type = $%d", argPos))
		args = append(args, filter.ModelType)
		argPos++
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("rm.name ILIKE $%d", argPos))
		args = append(args, "%"+filter.Search+"%")
		argPos++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM model_registry_registered_model rm WHERE %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count registered models: %w", err)
	}

	// Order
	orderBy := "rm.created_at DESC"
	if filter.SortBy != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf("rm.%s %s", filter.SortBy, dir)
	}

	query := fmt.Sprintf(`
		SELECT
			rm.id, rm.created_at, rm.updated_at, rm.project_id, rm.owner_id,
			rm.name, rm.slug, rm.description, rm.region_id,
			rm.model_type, rm.model_size, rm.state, rm.deployment_status,
			rm.tags, rm.labels, rm.parent_model_id,
			COALESCE(tu.email, '') AS owner_email,
			COALESCE(oreg.name, '') AS region_name,
			(SELECT COUNT(*) FROM model_registry_model_version mv WHERE mv.registered_model_id = rm.id) AS version_count
		FROM model_registry_registered_model rm
		LEFT JOIN tenant_user tu ON tu.id = rm.owner_id
		LEFT JOIN organization_region oreg ON oreg.id = rm.region_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argPos, argPos+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list registered models: %w", err)
	}
	defer rows.Close()

	var models []*domain.RegisteredModel
	for rows.Next() {
		m, err := r.scanModelFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan registered model row: %w", err)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate registered model rows: %w", err)
	}

	return models, total, nil
}

// scanModel scans a single RegisteredModel from a pgx.Row.
func (r *registeredModelRepo) scanModel(row pgx.Row) (*domain.RegisteredModel, error) {
	m := &domain.RegisteredModel{}
	var tagsJSON, labelsJSON []byte

	err := row.Scan(
		&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.ProjectID, &m.OwnerID,
		&m.Name, &m.Slug, &m.Description, &m.RegionID,
		&m.ModelType, &m.ModelSize, &m.State, &m.DeploymentStatus,
		&tagsJSON, &labelsJSON, &m.ParentModelID,
		&m.OwnerEmail, &m.RegionName, &m.VersionCount,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(tagsJSON, &m.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &m.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}

	return m, nil
}

// scanModelFromRows scans a RegisteredModel from pgx.Rows (same columns as scanModel).
func (r *registeredModelRepo) scanModelFromRows(rows pgx.Rows) (*domain.RegisteredModel, error) {
	m := &domain.RegisteredModel{}
	var tagsJSON, labelsJSON []byte

	err := rows.Scan(
		&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.ProjectID, &m.OwnerID,
		&m.Name, &m.Slug, &m.Description, &m.RegionID,
		&m.ModelType, &m.ModelSize, &m.State, &m.DeploymentStatus,
		&tagsJSON, &labelsJSON, &m.ParentModelID,
		&m.OwnerEmail, &m.RegionName, &m.VersionCount,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(tagsJSON, &m.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &m.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}

	return m, nil
}

// loadVersionMeta loads latest_version and default_version for a model.
func (r *registeredModelRepo) loadVersionMeta(ctx context.Context, model *domain.RegisteredModel) error {
	versionQuery := `
		SELECT id, created_at, updated_at, registered_model_id, name, description,
			   is_default, state, status, created_by_id, updated_by_id,
			   artifact_type, model_framework, model_framework_version,
			   container_image, model_catalog_name, uri, access_key, secret_key,
			   labels, prebuilt_container_id
		FROM model_registry_model_version
		WHERE registered_model_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	latest, err := scanVersion(r.pool.QueryRow(ctx, versionQuery, model.ID))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load latest version: %w", err)
	}
	if err == nil {
		model.LatestVersion = latest
	}

	defaultQuery := `
		SELECT id, created_at, updated_at, registered_model_id, name, description,
			   is_default, state, status, created_by_id, updated_by_id,
			   artifact_type, model_framework, model_framework_version,
			   container_image, model_catalog_name, uri, access_key, secret_key,
			   labels, prebuilt_container_id
		FROM model_registry_model_version
		WHERE registered_model_id = $1 AND is_default = true
		LIMIT 1
	`
	def, err := scanVersion(r.pool.QueryRow(ctx, defaultQuery, model.ID))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load default version: %w", err)
	}
	if err == nil {
		model.DefaultVersion = def
	}

	return nil
}

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
	"model-registry-service/internal/core/ports/output"
)

type modelVersionRepo struct {
	pool *pgxpool.Pool
}

func NewModelVersionRepository(pool *pgxpool.Pool) ports.ModelVersionRepository {
	return &modelVersionRepo{pool: pool}
}

func (r *modelVersionRepo) Create(ctx context.Context, version *domain.ModelVersion) error {
	labelsJSON, err := json.Marshal(version.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		INSERT INTO model_version
			(id, created_at, updated_at, registered_model_id, name, description,
			 is_default, state, status, created_by_id, updated_by_id,
			 created_by_email, updated_by_email,
			 artifact_type, model_framework, model_framework_version,
			 container_image, model_catalog_name, uri, access_key, secret_key,
			 labels, prebuilt_container_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
	`
	_, err = r.pool.Exec(ctx, query,
		version.ID, version.CreatedAt, version.UpdatedAt,
		version.RegisteredModelID, version.Name, version.Description,
		version.IsDefault, string(version.State), string(version.Status),
		version.CreatedByID, version.UpdatedByID,
		version.CreatedByEmail, version.UpdatedByEmail,
		string(version.ArtifactType), version.ModelFramework, version.ModelFrameworkVersion,
		version.ContainerImage, version.ModelCatalogName, version.URI,
		version.AccessKey, version.SecretKey, labelsJSON, version.PrebuiltContainerID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrVersionNameConflict
		}
		return fmt.Errorf("create model version: %w", err)
	}
	return nil
}

func (r *modelVersionRepo) GetByID(ctx context.Context, projectID uuid.UUID, id uuid.UUID) (*domain.ModelVersion, error) {
	query := `
		SELECT mv.id, mv.created_at, mv.updated_at, mv.registered_model_id,
			   mv.name, mv.description, mv.is_default, mv.state, mv.status,
			   mv.created_by_id, mv.updated_by_id,
			   mv.artifact_type, mv.model_framework, mv.model_framework_version,
			   mv.container_image, mv.model_catalog_name, mv.uri,
			   mv.access_key, mv.secret_key, mv.labels, mv.prebuilt_container_id,
			   COALESCE(mv.created_by_email, '') AS created_by_email,
			   COALESCE(mv.updated_by_email, '') AS updated_by_email
		FROM model_version mv
		JOIN registered_model rm ON rm.id = mv.registered_model_id
		WHERE mv.id = $1 AND rm.project_id = $2
	`
	v, err := scanVersionWithEmails(r.pool.QueryRow(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, fmt.Errorf("get model version by id: %w", err)
	}
	return v, nil
}

func (r *modelVersionRepo) GetByModelAndVersion(ctx context.Context, projectID uuid.UUID, modelID uuid.UUID, versionID uuid.UUID) (*domain.ModelVersion, error) {
	query := `
		SELECT mv.id, mv.created_at, mv.updated_at, mv.registered_model_id,
			   mv.name, mv.description, mv.is_default, mv.state, mv.status,
			   mv.created_by_id, mv.updated_by_id,
			   mv.artifact_type, mv.model_framework, mv.model_framework_version,
			   mv.container_image, mv.model_catalog_name, mv.uri,
			   mv.access_key, mv.secret_key, mv.labels, mv.prebuilt_container_id,
			   COALESCE(mv.created_by_email, '') AS created_by_email,
			   COALESCE(mv.updated_by_email, '') AS updated_by_email
		FROM model_version mv
		JOIN registered_model rm ON rm.id = mv.registered_model_id
		WHERE mv.registered_model_id = $1 AND mv.id = $2 AND rm.project_id = $3
	`
	v, err := scanVersionWithEmails(r.pool.QueryRow(ctx, query, modelID, versionID, projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, fmt.Errorf("get model version by model and version: %w", err)
	}
	return v, nil
}

func (r *modelVersionRepo) Update(ctx context.Context, projectID uuid.UUID, version *domain.ModelVersion) error {
	labelsJSON, err := json.Marshal(version.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	query := `
		UPDATE model_version
		SET name=$1, description=$2, is_default=$3, state=$4, status=$5,
			updated_by_id=$6, artifact_type=$7, model_framework=$8,
			model_framework_version=$9, container_image=$10, uri=$11,
			labels=$12, updated_at=NOW()
		WHERE id=$13
			AND registered_model_id IN (
				SELECT id FROM registered_model WHERE project_id = $14
			)
	`
	result, err := r.pool.Exec(ctx, query,
		version.Name, version.Description, version.IsDefault,
		string(version.State), string(version.Status), version.UpdatedByID,
		string(version.ArtifactType), version.ModelFramework,
		version.ModelFrameworkVersion, version.ContainerImage,
		version.URI, labelsJSON, version.ID, projectID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrVersionNameConflict
		}
		return fmt.Errorf("update model version: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrVersionNotFound
	}
	return nil
}

func (r *modelVersionRepo) List(ctx context.Context, filter ports.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	conditions := []string{}
	args := []interface{}{}
	argPos := 1

	// Project scoping via JOIN
	needProjectJoin := filter.ProjectID != uuid.Nil

	if filter.RegisteredModelID != uuid.Nil {
		conditions = append(conditions, fmt.Sprintf("mv.registered_model_id = $%d", argPos))
		args = append(args, filter.RegisteredModelID)
		argPos++
	}
	if filter.State != "" {
		conditions = append(conditions, fmt.Sprintf("mv.state = $%d", argPos))
		args = append(args, filter.State)
		argPos++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("mv.status = $%d", argPos))
		args = append(args, filter.Status)
		argPos++
	}
	if needProjectJoin {
		conditions = append(conditions, fmt.Sprintf("rm.project_id = $%d", argPos))
		args = append(args, filter.ProjectID)
		argPos++
	}

	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}

	joinClause := ""
	if needProjectJoin {
		joinClause = "JOIN registered_model rm ON rm.id = mv.registered_model_id"
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM model_version mv %s WHERE %s", joinClause, whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count model versions: %w", err)
	}

	orderBy := "mv.created_at DESC"
	if filter.SortBy != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		}
		orderBy = fmt.Sprintf("mv.%s %s", filter.SortBy, dir)
	}

	query := fmt.Sprintf(`
		SELECT mv.id, mv.created_at, mv.updated_at, mv.registered_model_id,
			   mv.name, mv.description, mv.is_default, mv.state, mv.status,
			   mv.created_by_id, mv.updated_by_id,
			   mv.artifact_type, mv.model_framework, mv.model_framework_version,
			   mv.container_image, mv.model_catalog_name, mv.uri,
			   mv.access_key, mv.secret_key, mv.labels, mv.prebuilt_container_id,
			   COALESCE(mv.created_by_email, '') AS created_by_email,
			   COALESCE(mv.updated_by_email, '') AS updated_by_email
		FROM model_version mv
		%s
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, joinClause, whereClause, orderBy, argPos, argPos+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list model versions: %w", err)
	}
	defer rows.Close()

	var versions []*domain.ModelVersion
	for rows.Next() {
		v, err := scanVersionWithEmailsFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan model version row: %w", err)
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate model version rows: %w", err)
	}

	return versions, total, nil
}

func (r *modelVersionRepo) ListByModel(ctx context.Context, modelID uuid.UUID, filter ports.VersionListFilter) ([]*domain.ModelVersion, int, error) {
	if modelID == uuid.Nil {
		return []*domain.ModelVersion{}, 0, nil
	}
	filter.RegisteredModelID = modelID
	return r.List(ctx, filter)
}

func (r *modelVersionRepo) FindByParams(ctx context.Context, projectID uuid.UUID, name string, externalID string, modelID *uuid.UUID) (*domain.ModelVersion, error) {
	conditions := []string{"rm.project_id = $1"}
	args := []interface{}{projectID}
	argPos := 2

	if name != "" {
		conditions = append(conditions, fmt.Sprintf("mv.name = $%d", argPos))
		args = append(args, name)
		argPos++
	}
	if externalID != "" {
		conditions = append(conditions, fmt.Sprintf("mv.id::text = $%d", argPos))
		args = append(args, externalID)
		argPos++
	}
	if modelID != nil {
		conditions = append(conditions, fmt.Sprintf("mv.registered_model_id = $%d", argPos))
		args = append(args, *modelID)
		argPos++
	}

	if len(conditions) == 1 {
		return nil, domain.ErrVersionNotFound
	}

	query := fmt.Sprintf(`
		SELECT mv.id, mv.created_at, mv.updated_at, mv.registered_model_id,
			   mv.name, mv.description, mv.is_default, mv.state, mv.status,
			   mv.created_by_id, mv.updated_by_id,
			   mv.artifact_type, mv.model_framework, mv.model_framework_version,
			   mv.container_image, mv.model_catalog_name, mv.uri,
			   mv.access_key, mv.secret_key, mv.labels, mv.prebuilt_container_id,
			   COALESCE(mv.created_by_email, '') AS created_by_email,
			   COALESCE(mv.updated_by_email, '') AS updated_by_email
		FROM model_version mv
		JOIN registered_model rm ON rm.id = mv.registered_model_id
		WHERE %s
		LIMIT 1
	`, strings.Join(conditions, " AND "))

	v, err := scanVersionWithEmails(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, fmt.Errorf("find model version by params: %w", err)
	}
	return v, nil
}

// scanVersion scans a ModelVersion without email joins (for registered model sub-queries).
func scanVersion(row pgx.Row) (*domain.ModelVersion, error) {
	v := &domain.ModelVersion{}
	var labelsJSON []byte

	err := row.Scan(
		&v.ID, &v.CreatedAt, &v.UpdatedAt, &v.RegisteredModelID,
		&v.Name, &v.Description, &v.IsDefault, &v.State, &v.Status,
		&v.CreatedByID, &v.UpdatedByID,
		&v.ArtifactType, &v.ModelFramework, &v.ModelFrameworkVersion,
		&v.ContainerImage, &v.ModelCatalogName, &v.URI,
		&v.AccessKey, &v.SecretKey, &labelsJSON, &v.PrebuiltContainerID,
	)
	if err != nil {
		return nil, err
	}

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &v.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}
	return v, nil
}

// scanVersionWithEmails scans a ModelVersion with email JOINs from a single row.
func scanVersionWithEmails(row pgx.Row) (*domain.ModelVersion, error) {
	v := &domain.ModelVersion{}
	var labelsJSON []byte

	err := row.Scan(
		&v.ID, &v.CreatedAt, &v.UpdatedAt, &v.RegisteredModelID,
		&v.Name, &v.Description, &v.IsDefault, &v.State, &v.Status,
		&v.CreatedByID, &v.UpdatedByID,
		&v.ArtifactType, &v.ModelFramework, &v.ModelFrameworkVersion,
		&v.ContainerImage, &v.ModelCatalogName, &v.URI,
		&v.AccessKey, &v.SecretKey, &labelsJSON, &v.PrebuiltContainerID,
		&v.CreatedByEmail, &v.UpdatedByEmail,
	)
	if err != nil {
		return nil, err
	}

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &v.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}
	return v, nil
}

// scanVersionWithEmailsFromRows scans a ModelVersion with email JOINs from pgx.Rows.
func scanVersionWithEmailsFromRows(rows pgx.Rows) (*domain.ModelVersion, error) {
	v := &domain.ModelVersion{}
	var labelsJSON []byte

	err := rows.Scan(
		&v.ID, &v.CreatedAt, &v.UpdatedAt, &v.RegisteredModelID,
		&v.Name, &v.Description, &v.IsDefault, &v.State, &v.Status,
		&v.CreatedByID, &v.UpdatedByID,
		&v.ArtifactType, &v.ModelFramework, &v.ModelFrameworkVersion,
		&v.ContainerImage, &v.ModelCatalogName, &v.URI,
		&v.AccessKey, &v.SecretKey, &labelsJSON, &v.PrebuiltContainerID,
		&v.CreatedByEmail, &v.UpdatedByEmail,
	)
	if err != nil {
		return nil, err
	}

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &v.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}
	return v, nil
}

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

type trafficVariantRepo struct {
	pool *pgxpool.Pool
}

// NewTrafficVariantRepository creates a new traffic variant repository
func NewTrafficVariantRepository(pool *pgxpool.Pool) ports.TrafficVariantRepository {
	return &trafficVariantRepo{pool: pool}
}

func (r *trafficVariantRepo) Create(ctx context.Context, variant *domain.TrafficVariant) error {
	query := `
		INSERT INTO traffic_variant (id, traffic_config_id, variant_name, model_version_id, weight, kserve_isvc_name, kserve_revision, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		variant.ID,
		variant.TrafficConfigID,
		variant.VariantName,
		variant.ModelVersionID,
		variant.Weight,
		nullableString(variant.KServeISVCName),
		nullableString(variant.KServeRevision),
		string(variant.Status),
		variant.CreatedAt,
		variant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert traffic_variant: %w", err)
	}
	return nil
}

func (r *trafficVariantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.TrafficVariant, error) {
	query := `
		SELECT tv.id, tv.created_at, tv.updated_at, tv.traffic_config_id, tv.variant_name,
		       tv.model_version_id, tv.weight, tv.kserve_isvc_name, tv.kserve_revision, tv.status,
		       COALESCE(mv.name, '') as model_version_name
		FROM traffic_variant tv
		LEFT JOIN model_version mv ON tv.model_version_id = mv.id
		WHERE tv.id = $1
	`
	variant, err := r.scanVariant(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTrafficVariantNotFound
		}
		return nil, fmt.Errorf("get traffic_variant by id: %w", err)
	}
	return variant, nil
}

func (r *trafficVariantRepo) GetByName(ctx context.Context, configID uuid.UUID, name string) (*domain.TrafficVariant, error) {
	query := `
		SELECT tv.id, tv.created_at, tv.updated_at, tv.traffic_config_id, tv.variant_name,
		       tv.model_version_id, tv.weight, tv.kserve_isvc_name, tv.kserve_revision, tv.status,
		       COALESCE(mv.name, '') as model_version_name
		FROM traffic_variant tv
		LEFT JOIN model_version mv ON tv.model_version_id = mv.id
		WHERE tv.traffic_config_id = $1 AND tv.variant_name = $2
	`
	variant, err := r.scanVariant(r.pool.QueryRow(ctx, query, configID, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTrafficVariantNotFound
		}
		return nil, fmt.Errorf("get traffic_variant by name: %w", err)
	}
	return variant, nil
}

func (r *trafficVariantRepo) Update(ctx context.Context, variant *domain.TrafficVariant) error {
	query := `
		UPDATE traffic_variant
		SET variant_name = $1, weight = $2, kserve_isvc_name = $3, kserve_revision = $4, status = $5, updated_at = NOW()
		WHERE id = $6
	`
	result, err := r.pool.Exec(ctx, query,
		variant.VariantName,
		variant.Weight,
		nullableString(variant.KServeISVCName),
		nullableString(variant.KServeRevision),
		string(variant.Status),
		variant.ID,
	)
	if err != nil {
		return fmt.Errorf("update traffic_variant: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrTrafficVariantNotFound
	}
	return nil
}

func (r *trafficVariantRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM traffic_variant WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete traffic_variant: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrTrafficVariantNotFound
	}
	return nil
}

func (r *trafficVariantRepo) ListByConfig(ctx context.Context, configID uuid.UUID) ([]*domain.TrafficVariant, error) {
	query := `
		SELECT tv.id, tv.created_at, tv.updated_at, tv.traffic_config_id, tv.variant_name,
		       tv.model_version_id, tv.weight, tv.kserve_isvc_name, tv.kserve_revision, tv.status,
		       COALESCE(mv.name, '') as model_version_name
		FROM traffic_variant tv
		LEFT JOIN model_version mv ON tv.model_version_id = mv.id
		WHERE tv.traffic_config_id = $1
		ORDER BY tv.created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, configID)
	if err != nil {
		return nil, fmt.Errorf("query traffic_variants: %w", err)
	}
	defer rows.Close()

	var variants []*domain.TrafficVariant
	for rows.Next() {
		variant, err := r.scanVariantFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan traffic_variant: %w", err)
		}
		variants = append(variants, variant)
	}

	return variants, nil
}

func (r *trafficVariantRepo) DeleteByConfig(ctx context.Context, configID uuid.UUID) error {
	query := `DELETE FROM traffic_variant WHERE traffic_config_id = $1`
	_, err := r.pool.Exec(ctx, query, configID)
	if err != nil {
		return fmt.Errorf("delete traffic_variants by config: %w", err)
	}
	return nil
}

func (r *trafficVariantRepo) scanVariant(row pgx.Row) (*domain.TrafficVariant, error) {
	var variant domain.TrafficVariant
	var isvcName, revision *string
	var versionName string

	err := row.Scan(
		&variant.ID, &variant.CreatedAt, &variant.UpdatedAt, &variant.TrafficConfigID, &variant.VariantName,
		&variant.ModelVersionID, &variant.Weight, &isvcName, &revision, &variant.Status, &versionName,
	)
	if err != nil {
		return nil, err
	}

	if isvcName != nil {
		variant.KServeISVCName = *isvcName
	}
	if revision != nil {
		variant.KServeRevision = *revision
	}
	variant.ModelVersionName = versionName

	return &variant, nil
}

func (r *trafficVariantRepo) scanVariantFromRows(rows pgx.Rows) (*domain.TrafficVariant, error) {
	var variant domain.TrafficVariant
	var isvcName, revision *string
	var versionName string

	err := rows.Scan(
		&variant.ID, &variant.CreatedAt, &variant.UpdatedAt, &variant.TrafficConfigID, &variant.VariantName,
		&variant.ModelVersionID, &variant.Weight, &isvcName, &revision, &variant.Status, &versionName,
	)
	if err != nil {
		return nil, err
	}

	if isvcName != nil {
		variant.KServeISVCName = *isvcName
	}
	if revision != nil {
		variant.KServeRevision = *revision
	}
	variant.ModelVersionName = versionName

	return &variant, nil
}

package postgres

import (
	"context"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FarmRepository persists and queries domain.Farm records in PostgreSQL.
type FarmRepository struct {
	db *pgxpool.Pool
}

// NewFarmRepository constructs a FarmRepository.
func NewFarmRepository(db *pgxpool.Pool) *FarmRepository {
	return &FarmRepository{db: db}
}

// Create inserts a new farm row.
func (r *FarmRepository) Create(ctx context.Context, farm *domain.Farm) error {
	const query = `
		INSERT INTO farms (id, user_id, name, latitude, longitude, location_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		farm.ID,
		farm.UserID,
		farm.Name,
		farm.Latitude,
		farm.Longitude,
		farm.LocationKey,
		farm.CreatedAt,
		farm.UpdatedAt,
	)
	return mapError("create farm", err)
}

// GetByID fetches a farm by primary key.
func (r *FarmRepository) GetByID(ctx context.Context, farmID uuid.UUID) (*domain.Farm, error) {
	const query = `
		SELECT id, user_id, name, latitude, longitude, location_key, created_at, updated_at
		FROM farms
		WHERE id = $1
	`
	var f domain.Farm
	err := r.db.QueryRow(ctx, query, farmID).Scan(
		&f.ID,
		&f.UserID,
		&f.Name,
		&f.Latitude,
		&f.Longitude,
		&f.LocationKey,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		return nil, mapError("get farm by id", err)
	}
	return &f, nil
}

// ListByUserID returns all farms owned by userID ordered newest first.
func (r *FarmRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error) {
	const query = `
		SELECT id, user_id, name, latitude, longitude, location_key, created_at, updated_at
		FROM farms
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	return r.queryFarms(ctx, "list farms by user", query, userID)
}

// ListAlertTargets returns up to limit farms that have at least one enabled alert rule, ordered by
// (location_key, id) after the cursor, each joined with its owner's email and enabled rules.
func (r *FarmRepository) ListAlertTargets(ctx context.Context, after domain.AlertTargetCursor, limit int) ([]*domain.AlertTarget, error) {
	const query = `
		WITH page AS (
			SELECT f.id
			FROM farms f
			WHERE (f.location_key, f.id) > ($1, $2)
			  AND EXISTS (SELECT 1 FROM weather_alerts a WHERE a.farm_id = f.id AND a.is_enabled)
			ORDER BY f.location_key, f.id
			LIMIT $3
		)
		SELECT f.id, f.user_id, f.name, f.latitude, f.longitude, f.location_key, f.created_at, f.updated_at,
		       u.email,
		       a.id, a.metric, a.operator, a.value, a.is_enabled, a.created_at, a.updated_at
		FROM page p
		JOIN farms f ON f.id = p.id
		JOIN users u ON u.id = f.user_id
		JOIN weather_alerts a ON a.farm_id = f.id AND a.is_enabled
		ORDER BY f.location_key, f.id, a.created_at
	`
	rows, err := r.db.Query(ctx, query, after.LocationKey, after.FarmID, limit)
	if err != nil {
		return nil, mapError("list alert targets", err)
	}
	defer rows.Close()

	var targets []*domain.AlertTarget
	for rows.Next() {
		var (
			f     domain.Farm
			email string
			w     domain.Weather
		)
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.Name, &f.Latitude, &f.Longitude, &f.LocationKey, &f.CreatedAt, &f.UpdatedAt,
			&email,
			&w.ID, &w.Metric, &w.Operator, &w.Value, &w.IsEnabled, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("list alert targets scan: %w", err)
		}
		w.FarmID = f.ID
		if n := len(targets); n == 0 || targets[n-1].Farm.ID != f.ID {
			targets = append(targets, &domain.AlertTarget{Farm: f, OwnerEmail: email})
		}
		last := targets[len(targets)-1]
		last.Alerts = append(last.Alerts, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list alert targets rows: %w", err)
	}
	return targets, nil
}

// ReconcileLocationKeys rewrites location_key with domain.NewLocationKey wherever the stored value
// differs (for example rows backfilled by SQL rounding in migration 005). It returns the rows changed.
func (r *FarmRepository) ReconcileLocationKeys(ctx context.Context) (int, error) {
	rows, err := r.db.Query(ctx, `SELECT id, latitude, longitude, location_key FROM farms`)
	if err != nil {
		return 0, mapError("scan location keys", err)
	}
	type fix struct {
		id  uuid.UUID
		key string
	}
	var fixes []fix
	for rows.Next() {
		var (
			id       uuid.UUID
			lat, lon float64
			stored   string
		)
		if err := rows.Scan(&id, &lat, &lon, &stored); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan location key: %w", err)
		}
		if want := domain.NewLocationKey(lat, lon); want != stored {
			fixes = append(fixes, fix{id: id, key: want})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate location keys: %w", err)
	}

	for _, f := range fixes {
		if _, err := r.db.Exec(ctx, `UPDATE farms SET location_key = $1 WHERE id = $2`, f.key, f.id); err != nil {
			return 0, mapError("update location key", err)
		}
	}
	return len(fixes), nil
}

func (r *FarmRepository) queryFarms(ctx context.Context, op, query string, args ...any) ([]*domain.Farm, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(op, err)
	}
	defer rows.Close()

	var farms []*domain.Farm
	for rows.Next() {
		var f domain.Farm
		if err := rows.Scan(
			&f.ID,
			&f.UserID,
			&f.Name,
			&f.Latitude,
			&f.Longitude,
			&f.LocationKey,
			&f.CreatedAt,
			&f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s scan: %w", op, err)
		}
		farms = append(farms, &f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s rows: %w", op, err)
	}
	return farms, nil
}

// Update modifies an existing farm record.
func (r *FarmRepository) Update(ctx context.Context, farm *domain.Farm) error {
	const query = `
		UPDATE farms
		SET name = $1, latitude = $2, longitude = $3, location_key = $4, updated_at = $5
		WHERE id = $6
	`
	res, err := r.db.Exec(ctx, query,
		farm.Name,
		farm.Latitude,
		farm.Longitude,
		farm.LocationKey,
		farm.UpdatedAt,
		farm.ID,
	)
	if err != nil {
		return mapError("update farm", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("update farm: %w", domain.ErrNotFound)
	}
	return nil
}

// Delete removes a farm by ID.
func (r *FarmRepository) Delete(ctx context.Context, farmID uuid.UUID) error {
	const query = `DELETE FROM farms WHERE id = $1`
	res, err := r.db.Exec(ctx, query, farmID)
	if err != nil {
		return mapError("delete farm", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("delete farm: %w", domain.ErrNotFound)
	}
	return nil
}

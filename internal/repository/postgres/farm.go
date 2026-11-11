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

// ListAll returns all farms grouped by location_key for batch forecast evaluation.
func (r *FarmRepository) ListAll(ctx context.Context) ([]*domain.Farm, error) {
	const query = `
		SELECT id, user_id, name, latitude, longitude, location_key, created_at, updated_at
		FROM farms
		ORDER BY location_key, created_at DESC
	`
	return r.queryFarms(ctx, "list all farms", query)
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

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/farm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FarmRepository struct {
	db *pgxpool.Pool
}

func NewFarmRepository(db *pgxpool.Pool) farm.Repository {
	return &FarmRepository{db: db}
}

func (r *FarmRepository) Create(ctx context.Context, farm *domain.Farm) error {
	query := `
		INSERT INTO farms (id, user_id, name, latitude, longitude, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		farm.ID,
		farm.UserID,
		farm.Name,
		farm.Latitude,
		farm.Longitude,
		farm.CreatedAt,
		farm.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert farm into database: %w", err)
	}
	return nil
}

func (r *FarmRepository) GetByID(ctx context.Context, farmID uuid.UUID) (*domain.Farm, error) {
	var f domain.Farm
	query := `
		SELECT id, user_id, name, latitude, longitude, created_at, updated_at
		FROM farms
		WHERE id = $1
	`
	err := r.db.QueryRow(ctx, query, farmID).Scan(
		&f.ID,
		&f.UserID,
		&f.Name,
		&f.Latitude,
		&f.Longitude,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get farm by ID: %w", err)
	}
	return &f, nil
}

func (r *FarmRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error) {
	var farms []*domain.Farm
	query := `
		SELECT id, user_id, name, latitude, longitude, created_at, updated_at
		FROM farms
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query farms by user ID: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var f domain.Farm
		if err := rows.Scan(
			&f.ID,
			&f.UserID,
			&f.Name,
			&f.Latitude,
			&f.Longitude,
			&f.CreatedAt,
			&f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan farm row: %w", err)
		}
		farms = append(farms, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return farms, nil
}

func (r *FarmRepository) Update(ctx context.Context, farm *domain.Farm) error {
	query := `
		UPDATE farms
		SET name = $1, latitude = $2, longitude = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query,
		farm.Name,
		farm.Latitude,
		farm.Longitude,
		time.Now(),
		farm.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update farm in database: %w", err)
	}

	return nil
}

func (r *FarmRepository) Delete(ctx context.Context, farmID uuid.UUID) error {
	query := `
		DELETE FROM farms
		WHERE id = $1
	`
	result, err := r.db.Exec(ctx, query, farmID)
	if err != nil {
		return fmt.Errorf("failed to delete farm from database: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("farm not found for deletion")
	}

	return nil
}

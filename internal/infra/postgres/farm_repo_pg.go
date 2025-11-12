package postgres

import (
	"context"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/farm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FarmRepositoryPG struct {
	db *pgxpool.Pool
}

func NewFarmRepositoryPG(db *pgxpool.Pool) farm.Repository {
	return &FarmRepositoryPG{
		db: db,
	}
}

func (r *FarmRepositoryPG) Create(ctx context.Context, farm *domain.Farm) error {
	query := `
		INSERT INTO farms (id, user_id, name, latitude, longitude, soil_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query, farm.ID, farm.UserID, farm.Name, farm.Latitude, farm.Longitude, farm.SoilType, farm.CreatedAt, farm.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create farm: %w", err)
	}
	return nil
}

func (r *FarmRepositoryPG) GetByID(ctx context.Context, id uuid.UUID) (*domain.Farm, error) {
	query := `
		SELECT id, user_id, name, latitude, longitude, soil_type, created_at, updated_at
		FROM farms
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)

	var farm domain.Farm
	err := row.Scan(&farm.ID, &farm.UserID, &farm.Name, &farm.Latitude, &farm.Longitude, &farm.SoilType, &farm.CreatedAt, &farm.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get farm by id: %w", err)
	}
	return &farm, nil
}

func (r *FarmRepositoryPG) Update(ctx context.Context, farm *domain.Farm) error {
	query := `
		UPDATE farms
		SET name = $1, latitude = $2, longitude = $3, soil_type = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, query, farm.Name, farm.Latitude, farm.Longitude, farm.SoilType, farm.UpdatedAt, farm.ID)
	if err != nil {
		return fmt.Errorf("failed to update farm: %w", err)
	}
	return nil
}

func (r *FarmRepositoryPG) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM farms WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete farm: %w", err)
	}
	return nil
}

func (r *FarmRepositoryPG) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error) {
	query := `
		SELECT id, user_id, name, latitude, longitude, soil_type, created_at, updated_at
		FROM farms
		WHERE user_id = $1
		ORDER BY created_at
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list farms by user id: %w", err)
	}
	defer rows.Close()

	var farms []*domain.Farm
	for rows.Next() {
		var farm domain.Farm
		if err := rows.Scan(&farm.ID, &farm.UserID, &farm.Name, &farm.Latitude, &farm.Longitude, &farm.SoilType, &farm.CreatedAt, &farm.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan farm: %w", err)
		}
		farms = append(farms, &farm)
	}

	return farms, nil
}

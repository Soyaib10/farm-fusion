package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/weather"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WeatherRepository struct {
	db *pgxpool.Pool
}

func NewWeatherRepository(db *pgxpool.Pool) weather.Repository {
	return &WeatherRepository{db: db}
}

func (r *WeatherRepository) Create(ctx context.Context, w *domain.Weather) error {
	query := `
		INSERT INTO weather_alerts (id, farm_id, metric, operator, value, is_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		w.ID,
		w.FarmID,
		w.Metric,
		w.Operator,
		w.Value,
		w.IsEnabled,
		w.CreatedAt,
		w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert weather alert into database: %w", err)
	}
	return nil
}

func (r *WeatherRepository) GetByID(ctx context.Context, weatherID uuid.UUID) (*domain.Weather, error) {
	var w domain.Weather
	query := `
		SELECT id, farm_id, metric, operator, value, is_enabled, created_at, updated_at
		FROM weather_alerts
		WHERE id = $1
	`
	err := r.db.QueryRow(ctx, query, weatherID).Scan(
		&w.ID,
		&w.FarmID,
		&w.Metric,
		&w.Operator,
		&w.Value,
		&w.IsEnabled,
		&w.CreatedAt,
		&w.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get weather alert by ID: %w", err)
	}
	return &w, nil
}

func (r *WeatherRepository) ListByFarmID(ctx context.Context, farmID uuid.UUID) ([]*domain.Weather, error) {
	var weathers []*domain.Weather
	query := `
		SELECT id, farm_id, metric, operator, value, is_enabled, created_at, updated_at
		FROM weather_alerts
		WHERE farm_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to query weather alerts by farm ID: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var w domain.Weather
		if err := rows.Scan(
			&w.ID,
			&w.FarmID,
			&w.Metric,
			&w.Operator,
			&w.Value,
			&w.IsEnabled,
			&w.CreatedAt,
			&w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan weather alert row: %w", err)
		}
		weathers = append(weathers, &w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return weathers, nil
}

func (r *WeatherRepository) Update(ctx context.Context, w *domain.Weather) error {
	query := `
		UPDATE weather_alerts
		SET metric = $1, operator = $2, value = $3, is_enabled = $4, updated_at = $5
		WHERE id = $6
	`
	result, err := r.db.Exec(ctx, query,
		w.Metric,
		w.Operator,
		w.Value,
		w.IsEnabled,
		time.Now(),
		w.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update weather alert in database: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("weather alert not found for update")
	}

	return nil
}

func (r *WeatherRepository) Delete(ctx context.Context, weatherID uuid.UUID) error {
	query := `
		DELETE FROM weather_alerts
		WHERE id = $1
	`
	result, err := r.db.Exec(ctx, query, weatherID)
	if err != nil {
		return fmt.Errorf("failed to delete weather alert from database: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("weather alert not found for deletion")
	}

	return nil
}

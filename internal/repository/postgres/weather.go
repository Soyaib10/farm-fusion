package postgres

import (
	"context"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WeatherRepository persists and queries domain.Weather alert rules in PostgreSQL.
type WeatherRepository struct {
	db *pgxpool.Pool
}

// NewWeatherRepository constructs a WeatherRepository.
func NewWeatherRepository(db *pgxpool.Pool) *WeatherRepository {
	return &WeatherRepository{db: db}
}

// Create inserts a new weather alert rule.
func (r *WeatherRepository) Create(ctx context.Context, w *domain.Weather) error {
	const query = `
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
	return mapError("create weather alert", err)
}

// GetByID fetches a weather alert rule by ID.
func (r *WeatherRepository) GetByID(ctx context.Context, weatherID uuid.UUID) (*domain.Weather, error) {
	const query = `
		SELECT id, farm_id, metric, operator, value, is_enabled, created_at, updated_at
		FROM weather_alerts
		WHERE id = $1
	`
	var w domain.Weather
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
		return nil, mapError("get weather alert by id", err)
	}
	return &w, nil
}

// ListByFarmID returns all weather alert rules configured for farmID.
func (r *WeatherRepository) ListByFarmID(ctx context.Context, farmID uuid.UUID) ([]*domain.Weather, error) {
	const query = `
		SELECT id, farm_id, metric, operator, value, is_enabled, created_at, updated_at
		FROM weather_alerts
		WHERE farm_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, farmID)
	if err != nil {
		return nil, mapError("list weather alerts by farm", err)
	}
	defer rows.Close()

	var rules []*domain.Weather
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
			return nil, fmt.Errorf("scan weather alert: %w", err)
		}
		rules = append(rules, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weather alerts: %w", err)
	}
	return rules, nil
}

// Update modifies an existing weather alert rule.
func (r *WeatherRepository) Update(ctx context.Context, w *domain.Weather) error {
	const query = `
		UPDATE weather_alerts
		SET metric = $1, operator = $2, value = $3, is_enabled = $4, updated_at = $5
		WHERE id = $6
	`
	res, err := r.db.Exec(ctx, query,
		w.Metric,
		w.Operator,
		w.Value,
		w.IsEnabled,
		w.UpdatedAt,
		w.ID,
	)
	if err != nil {
		return mapError("update weather alert", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("update weather alert: %w", domain.ErrNotFound)
	}
	return nil
}

// Delete removes a weather alert rule by ID.
func (r *WeatherRepository) Delete(ctx context.Context, weatherID uuid.UUID) error {
	const query = `DELETE FROM weather_alerts WHERE id = $1`
	res, err := r.db.Exec(ctx, query, weatherID)
	if err != nil {
		return mapError("delete weather alert", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("delete weather alert: %w", domain.ErrNotFound)
	}
	return nil
}

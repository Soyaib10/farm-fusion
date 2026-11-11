package weather

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/google/uuid"
)

// Repository defines weather alert persistence operations needed by the weather service.
type Repository interface {
	Create(ctx context.Context, weather *domain.Weather) error
	GetByID(ctx context.Context, weatherID uuid.UUID) (*domain.Weather, error)
	ListByFarmID(ctx context.Context, farmID uuid.UUID) ([]*domain.Weather, error)
	Update(ctx context.Context, weather *domain.Weather) error
	Delete(ctx context.Context, weatherID uuid.UUID) error
}

// FarmOwnershipChecker verifies that a farm exists and belongs to a given user.
type FarmOwnershipChecker interface {
	GetByID(ctx context.Context, farmID, userID uuid.UUID) (*domain.Farm, error)
}

// CreateInput holds parameters for creating a weather alert threshold on a farm.
type CreateInput struct {
	UserID   uuid.UUID
	FarmID   uuid.UUID
	Metric   string
	Operator string
	Value    float64
}

// UpdateInput holds parameters for updating an existing weather alert threshold.
type UpdateInput struct {
	UserID    uuid.UUID
	FarmID    uuid.UUID
	WeatherID uuid.UUID
	Metric    string
	Operator  string
	Value     float64
	IsEnabled bool
}

// Service manages weather alert thresholds and enforces farm ownership.
type Service struct {
	repo  Repository
	farms FarmOwnershipChecker
}

// NewService constructs a weather alert Service.
func NewService(repo Repository, farms FarmOwnershipChecker) *Service {
	return &Service{repo: repo, farms: farms}
}

func validateRule(rawMetric, rawOperator string, value float64) (string, string, error) {
	metric := normalizeMetric(rawMetric)
	operator := normalizeOperator(rawOperator)

	v := validator.New()
	v.Check(validator.In(metric, domain.AllowedWeatherMetrics...), "metric",
		"must be one of: temperature, humidity, rainfall, wind_speed")
	v.Check(validator.In(operator, domain.AllowedWeatherOperators...), "operator",
		"must be one of: <, <=, >, >=, =")
	v.Check(validator.Finite(value), "value", "must be a finite number")

	if validator.Finite(value) {
		switch metric {
		case "temperature":
			v.Check(validator.Between(value, -90, 70), "value", "temperature must be between -90 and 70 °C")
		case "humidity":
			v.Check(validator.Between(value, 0, 100), "value", "humidity must be between 0 and 100%")
		case "rainfall":
			v.Check(validator.Between(value, 0, 2000), "value", "rainfall must be between 0 and 2000 mm")
		case "wind_speed":
			v.Check(validator.Between(value, 0, 500), "value", "wind_speed must be between 0 and 500 km/h")
		}
	}

	if err := v.Err(); err != nil {
		return "", "", err
	}
	return metric, operator, nil
}

func normalizeMetric(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "temp", "temperature":
		return "temperature"
	case "rain", "rainfall":
		return "rainfall"
	case "wind", "wind_speed":
		return "wind_speed"
	case "humidity":
		return "humidity"
	default:
		return strings.ToLower(strings.TrimSpace(m))
	}
}

func normalizeOperator(op string) string {
	trimmed := strings.TrimSpace(op)
	if trimmed == "==" {
		return "="
	}
	return trimmed
}

// Create validates the rule, checks farm ownership, and creates a weather alert.
func (s *Service) Create(ctx context.Context, input CreateInput) (*domain.Weather, error) {
	metric, operator, err := validateRule(input.Metric, input.Operator, input.Value)
	if err != nil {
		return nil, err
	}

	if _, err := s.farms.GetByID(ctx, input.FarmID, input.UserID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	rule := &domain.Weather{
		ID:        uuid.New(),
		FarmID:    input.FarmID,
		Metric:    metric,
		Operator:  operator,
		Value:     input.Value,
		IsEnabled: true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("create weather alert: %w", err)
	}
	return rule, nil
}

// GetByID retrieves a weather alert after verifying farm ownership and farm association.
func (s *Service) GetByID(ctx context.Context, userID, farmID, weatherID uuid.UUID) (*domain.Weather, error) {
	if _, err := s.farms.GetByID(ctx, farmID, userID); err != nil {
		return nil, err
	}

	rule, err := s.repo.GetByID(ctx, weatherID)
	if err != nil {
		return nil, err
	}
	if rule.FarmID != farmID {
		return nil, domain.ErrNotFound
	}
	return rule, nil
}

// ListByFarm lists all weather alerts for farmID after verifying farm ownership.
func (s *Service) ListByFarm(ctx context.Context, userID, farmID uuid.UUID) ([]*domain.Weather, error) {
	if _, err := s.farms.GetByID(ctx, farmID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListByFarmID(ctx, farmID)
}

// Update validates fields, checks farm ownership, and updates an existing weather alert.
func (s *Service) Update(ctx context.Context, input UpdateInput) (*domain.Weather, error) {
	metric, operator, err := validateRule(input.Metric, input.Operator, input.Value)
	if err != nil {
		return nil, err
	}

	if _, err := s.farms.GetByID(ctx, input.FarmID, input.UserID); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, input.WeatherID)
	if err != nil {
		return nil, err
	}
	if existing.FarmID != input.FarmID {
		return nil, domain.ErrNotFound
	}

	updated := &domain.Weather{
		ID:        input.WeatherID,
		FarmID:    input.FarmID,
		Metric:    metric,
		Operator:  operator,
		Value:     input.Value,
		IsEnabled: input.IsEnabled,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, fmt.Errorf("update weather alert: %w", err)
	}
	return updated, nil
}

// Delete removes a weather alert after verifying farm ownership and farm association.
func (s *Service) Delete(ctx context.Context, userID, farmID, weatherID uuid.UUID) error {
	if _, err := s.farms.GetByID(ctx, farmID, userID); err != nil {
		return err
	}

	existing, err := s.repo.GetByID(ctx, weatherID)
	if err != nil {
		return err
	}
	if existing.FarmID != farmID {
		return domain.ErrNotFound
	}
	return s.repo.Delete(ctx, weatherID)
}

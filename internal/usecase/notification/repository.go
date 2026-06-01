package notification

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type ForecastRepository interface {
	Get(ctx context.Context, locationKey string) (*domain.WeatherForecast, error)
	Set(ctx context.Context, forecast *domain.WeatherForecast) error
}

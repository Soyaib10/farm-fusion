package notification

import (
	"context"
	"errors"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

// ErrCacheMiss is returned by ForecastRepository.Get when no cached entry exists.
var ErrCacheMiss = errors.New("cache miss")

type ForecastRepository interface {
	Get(ctx context.Context, locationKey string) (*domain.WeatherForecast, error)
	Set(ctx context.Context, forecast *domain.WeatherForecast) error
	Close() error
}

type ForecastProvider interface {
	FetchForecast(ctx context.Context, lat, lon float64, locationKey string) (*domain.WeatherForecast, error)
}

type FarmRepository interface {
	ListAll(ctx context.Context) ([]*domain.Farm, error)
}

type WeatherAlertRepository interface {
	ListByFarmID(ctx context.Context, farmID uuid.UUID) ([]*domain.Weather, error)
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type LogRepository interface {
	Create(ctx context.Context, log *domain.NotificationLog) error
}

type EmailSender interface {
	SendWeatherNotification(ctx context.Context, payload *domain.NotificationPayload) (string, error)
}

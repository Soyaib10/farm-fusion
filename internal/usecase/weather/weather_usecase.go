package weather

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type UseCase interface {
	Create(ctx context.Context, cmd CreateWeatherCommand) (*domain.Weather, error)
	GetByID(ctx context.Context, userID, farmID, weatherID uuid.UUID) (*domain.Weather, error)
	ListByFarm(ctx context.Context, userID, farmID uuid.UUID) ([]*domain.Weather, error)
	Update(ctx context.Context, cmd UpdateWeatherCommand) (*domain.Weather, error)
	Delete(ctx context.Context, userID, farmID, weatherID uuid.UUID) error
}

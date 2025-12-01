package weather

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, weather *domain.Weather) error
	GetByID(ctx context.Context, weatherID uuid.UUID) (*domain.Weather, error)
	ListByFarmID(ctx context.Context, farmID uuid.UUID) ([]*domain.Weather, error)
	Update(ctx context.Context, weather *domain.Weather) error
	Delete(ctx context.Context, weatherID uuid.UUID) error
}

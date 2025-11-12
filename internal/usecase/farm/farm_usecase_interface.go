package farm

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type UseCase interface {
	Create(ctx context.Context, userID uuid.UUID, name string, latitude, longitude float64, soilType string) (*domain.Farm, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Farm, error)
	Update(ctx context.Context, userID uuid.UUID, name string, latitude, longitude float64, soilType string) (*domain.Farm, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error)
}

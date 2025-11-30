package farm

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, farm *domain.Farm) error
	GetByID(ctx context.Context, farmID uuid.UUID) (*domain.Farm, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error)
	Update(ctx context.Context, farm *domain.Farm) error
	Delete(ctx context.Context, farmID uuid.UUID) error
}

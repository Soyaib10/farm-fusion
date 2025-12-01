package farm

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type UseCase interface {
	Create(ctx context.Context, userID uuid.UUID, cmd UpsertFarmCommand) (*domain.Farm, error)
	GetByID(ctx context.Context, farmID uuid.UUID, userID uuid.UUID) (*domain.Farm, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error)
	Update(ctx context.Context, farmID uuid.UUID, userID uuid.UUID, cmd UpsertFarmCommand) (*domain.Farm, error)
	Delete(ctx context.Context, farmID uuid.UUID, userID uuid.UUID) error
}

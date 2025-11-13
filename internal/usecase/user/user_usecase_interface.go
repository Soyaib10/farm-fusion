package user

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type UseCase interface {
	Create(ctx context.Context, name, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, id uuid.UUID, name, email string) (*domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Find(ctx context.Context) ([]domain.User, error)
}

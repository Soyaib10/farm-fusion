package user

import (
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type UseCase interface {
	Create(name, email string) (*domain.User, error)
	GetByID(id uuid.UUID) (*domain.User, error)
}
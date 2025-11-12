package user

import (
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type Repository interface {
	Create(user *domain.User) error
	GetByID(id uuid.UUID) (*domain.User, error)
}
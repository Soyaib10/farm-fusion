package user

import "github.com/Soyaib10/farm-fusion/internal/domain"

type Repository interface {
	Create(user *domain.User) error
	GetByID(id string) (*domain.User, error)
}
package user

import "github.com/Soyaib10/farm-fusion/internal/domain"

type UseCase interface {
	Create(name, email string) (*domain.User, error)
	GetByID(id string) (*domain.User, error)
}
package user

import (
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/pkg/helpers"
)

type useCase struct {
	repository Repository
}

func NewUseCase(repository Repository) UseCase {
	return &useCase{
		repository: repository,
	}
}

func (uc *useCase) Create(name, email string) (*domain.User, error) {
	user := &domain.User{
		ID:        helpers.GenerateID(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}

	// validate using domain rules
	if err := user.Validate(); err != nil  {
		return nil, err
	}
    
	if err := uc.repository.Create(user); err !=  nil {
		return nil, err
	}
	return user, nil							
}

func (uc *useCase) GetByID(id string) (*domain.User, error) {
	return uc.repository.GetByID(id)
}

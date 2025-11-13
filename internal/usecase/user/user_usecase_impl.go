package user

import (
	"context"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/pkg/helpers"
	"github.com/google/uuid"
)

type useCase struct {
	repository Repository
}

func NewUseCase(repository Repository) UseCase {
	return &useCase{
		repository: repository,
	}
}

func (uc *useCase) Create(ctx context.Context, name, email string) (*domain.User, error) {
	user := &domain.User{
		ID:        helpers.GenerateID(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}

	// validate using domain rules
	if err := user.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repository.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *useCase) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return uc.repository.GetByID(ctx, id)
}

func (uc *useCase) Update(ctx context.Context, id uuid.UUID, name, email string) (*domain.User, error) {
	user, err := uc.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	user.Name = name
	user.Email = email

	if err := user.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repository.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (uc *useCase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repository.Delete(ctx, id)
}

func (uc *useCase) Find(ctx context.Context) ([]domain.User, error) {
	return uc.repository.Find(ctx)
}

package user

import (
	"context"
	"errors"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type useCase struct {
	repo Repository
}

func New(repo Repository) UseCase {
	return &useCase{repo: repo}
}

func (uc *useCase) Create(ctx context.Context, cmd UpsertUserCommand) (*domain.User, error) {
	if cmd.Name == nil || cmd.Email == nil {
		return nil, errors.New("name and email are required")
	}

	user := &domain.User{
		ID:        uuid.New(),
		Name:      *cmd.Name,
		Email:     *cmd.Email,
		CreatedAt: time.Now().UTC(),
	}

	if err := domain.ValidateUser(user); err != nil {
		return nil, err
	}

	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (uc *useCase) Update(ctx context.Context, id uuid.UUID, cmd UpsertUserCommand) (*domain.User, error) {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if cmd.Name != nil {
		user.Name = *cmd.Name
	}
	if cmd.Email != nil {
		user.Email = *cmd.Email
	}

	if err := domain.ValidateUser(user); err != nil {
		return nil, err
	}

	if err := uc.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (uc *useCase) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return uc.repo.GetByID(ctx, id)
}

func (uc *useCase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *useCase) List(ctx context.Context) ([]*domain.User, error) {
	return uc.repo.List(ctx)
}
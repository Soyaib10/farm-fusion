package user

import (
	"context"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/google/uuid"
)

type useCase struct {
	repo Repository
}

func New(repo Repository) UseCase {
	return &useCase{repo: repo}
}

func (uc *useCase) Create(ctx context.Context, cmd UpsertUserCommand) (*domain.User, error) {
	v := validator.New()

	v.Check(cmd.Name != nil && *cmd.Name != "", "name", "must be provided")
	v.Check(cmd.Name == nil || len(*cmd.Name) <= 500, "name", "must not be more than 500 characters")
	v.Check(cmd.Email != nil && *cmd.Email != "", "email", "must be provided")
	v.Check(cmd.Email == nil || validator.Matches(*cmd.Email, validator.EmailRX), "email", "must be a valid email address")

	if !v.Valid() {
		return nil, v.Errors 
	}

	user := &domain.User{
		ID:        uuid.New(),
		Name:      *cmd.Name,
		Email:     *cmd.Email,
		CreatedAt: time.Now().UTC(),
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

	v := validator.New()

	if cmd.Name != nil {
		v.Check(*cmd.Name != "", "name", "must be provided")
		v.Check(len(*cmd.Name) <= 500, "name", "must not be more than 500 characters")
	}
	if cmd.Email != nil {
		v.Check(*cmd.Email != "", "email", "must be provided")
		v.Check(validator.Matches(*cmd.Email, validator.EmailRX), "email", "must be a valid email address")
	}

	if !v.Valid() {
		return nil, v.Errors
	}

	if cmd.Name != nil {
		user.Name = *cmd.Name
	}
	if cmd.Email != nil {
		user.Email = *cmd.Email
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

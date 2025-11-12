package farm

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

func (uc *useCase) Create(ctx context.Context, userID uuid.UUID, name string, latitude, longitude float64, soilType string) (*domain.Farm, error) {
	farm := &domain.Farm{
		ID:        helpers.GenerateID(),
		UserID:    userID,
		Name:      name,
		Latitude:  latitude,
		Longitude: longitude,
		SoilType:  soilType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := farm.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repository.Create(ctx, farm); err != nil {
		return nil, err
	}

	return farm, nil
}

func (uc *useCase) GetByID(ctx context.Context, id uuid.UUID) (*domain.Farm, error) {
	return uc.repository.GetByID(ctx, id)
}

func (uc *useCase) Update(ctx context.Context, userID uuid.UUID, name string, latitude, longitude float64, soilType string) (*domain.Farm, error) {
	farm := &domain.Farm{
		UserID:    userID,
		Name:      name,
		Latitude:  latitude,
		Longitude: longitude,
		SoilType:  soilType,
		UpdatedAt: time.Now(),
	}

	if err := farm.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repository.Update(ctx, farm); err != nil {
		return nil, err
	}

	return farm, nil
}

func (uc *useCase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repository.Delete(ctx, id)
}

func (uc *useCase) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error) {
	return uc.repository.ListByUserID(ctx, userID)
}

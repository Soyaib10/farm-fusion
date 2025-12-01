package farm

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type useCase struct {
	repo Repository
}

func NewUseCase(repo Repository) UseCase {
	return &useCase{repo: repo}
}

func (uc *useCase) Create(ctx context.Context, userID uuid.UUID, cmd UpsertFarmCommand) (*domain.Farm, error) {
	farm := &domain.Farm{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      *cmd.Name,
		Latitude:  *cmd.Latitude,
		Longitude: *cmd.Longitude,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := uc.repo.Create(ctx, farm); err != nil {
		return nil, fmt.Errorf("failed to create farm: %w", err)
	}

	return farm, nil
}

func (uc *useCase) GetByID(ctx context.Context, farmID uuid.UUID, userID uuid.UUID) (*domain.Farm, error) {
	farm, err := uc.repo.GetByID(ctx, farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to get farm: %w", err)
	}

	if farm.UserID != userID {
		return nil, fmt.Errorf("failed to get farm: not found or access denied")
	}

	return farm, nil
}

func (uc *useCase) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Farm, error) {
	farms, err := uc.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list farms: %w", err)
	}
	return farms, nil
}

func (uc *useCase) Update(ctx context.Context, farmID uuid.UUID, userID uuid.UUID, cmd UpsertFarmCommand) (*domain.Farm, error) {
	// 1. Fetch
	existingFarm, err := uc.repo.GetByID(ctx, farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to get farm for update: %w", err)
	}

	// 2. Authorize
	if existingFarm.UserID != userID {
		return nil, fmt.Errorf("failed to update farm: not found or access denied")
	}

	// 3. Execute
	updatedFarm := &domain.Farm{
		ID:        farmID,
		UserID:    userID,
		Name:      *cmd.Name,
		Latitude:  *cmd.Latitude,
		Longitude: *cmd.Longitude,
		UpdatedAt: time.Now(),
		CreatedAt: existingFarm.CreatedAt,
	}

	if err := uc.repo.Update(ctx, updatedFarm); err != nil {
		return nil, fmt.Errorf("failed to execute farm update: %w", err)
	}
	return updatedFarm, nil
}

func (uc *useCase) Delete(ctx context.Context, farmID uuid.UUID, userID uuid.UUID) error {
	farm, err := uc.repo.GetByID(ctx, farmID)
	if err != nil {
		return fmt.Errorf("failed to get farm for deletion: %w", err)
	}

	if farm.UserID != userID {
		return fmt.Errorf("failed to delete farm: not found or access denied")
	}

	if err := uc.repo.Delete(ctx, farmID); err != nil {
		return fmt.Errorf("failed to execute farm deletion: %w", err)
	}
	return nil
}

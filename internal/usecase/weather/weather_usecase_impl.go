package weather

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	farmUsecase "github.com/Soyaib10/farm-fusion/internal/usecase/farm"
	"github.com/google/uuid"
)

type useCase struct {
	repo   Repository
	farmUC farmUsecase.UseCase
}

func NewUseCase(repo Repository, farmUC farmUsecase.UseCase) UseCase {
	return &useCase{repo: repo, farmUC: farmUC}
}

func (uc *useCase) checkFarmOwnership(ctx context.Context, userID, farmID uuid.UUID) error {
	_, err := uc.farmUC.GetByID(ctx, farmID, userID)
	if err != nil {
		return fmt.Errorf("farm not found or access denied: %w", err)
	}
	return nil
}

func (uc *useCase) Create(ctx context.Context, cmd CreateWeatherCommand) (*domain.Weather, error) {
	if err := uc.checkFarmOwnership(ctx, cmd.UserID, cmd.FarmID); err != nil {
		return nil, fmt.Errorf("cannot create weather alert: %w", err)
	}

	weather := &domain.Weather{
		ID:        uuid.New(),
		FarmID:    cmd.FarmID,
		Metric:    cmd.Metric,
		Operator:  cmd.Operator,
		Value:     cmd.Value,
		IsEnabled: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := uc.repo.Create(ctx, weather); err != nil {
		return nil, fmt.Errorf("failed to create weather alert: %w", err)
	}

	return weather, nil
}

func (uc *useCase) GetByID(ctx context.Context, userID, farmID, weatherID uuid.UUID) (*domain.Weather, error) {
	weather, err := uc.repo.GetByID(ctx, weatherID)
	if err != nil {
		return nil, fmt.Errorf("failed to get weather alert: %w", err)
	}

	if weather.FarmID != farmID {
		return nil, fmt.Errorf("weather alert not found on specified farm")
	}
	if err := uc.checkFarmOwnership(ctx, userID, farmID); err != nil {
		return nil, fmt.Errorf("cannot get weather alert: %w", err)
	}

	return weather, nil
}

func (uc *useCase) ListByFarm(ctx context.Context, userID, farmID uuid.UUID) ([]*domain.Weather, error) {
	if err := uc.checkFarmOwnership(ctx, userID, farmID); err != nil {
		return nil, fmt.Errorf("cannot list weather alerts: %w", err)
	}

	weathers, err := uc.repo.ListByFarmID(ctx, farmID)
	if err != nil {
		return nil, fmt.Errorf("failed to list weather alerts: %w", err)
	}
	return weathers, nil
}

func (uc *useCase) Update(ctx context.Context, cmd UpdateWeatherCommand) (*domain.Weather, error) {
	existingWeather, err := uc.repo.GetByID(ctx, cmd.WeatherID)
	if err != nil {
		return nil, fmt.Errorf("failed to get weather alert for update: %w", err)
	}

	if existingWeather.FarmID != cmd.FarmID {
		return nil, fmt.Errorf("weather alert not found on specified farm")
	}
	if err := uc.checkFarmOwnership(ctx, cmd.UserID, cmd.FarmID); err != nil {
		return nil, fmt.Errorf("cannot update weather alert: %w", err)
	}

	updatedWeather := &domain.Weather{
		ID:        cmd.WeatherID,
		FarmID:    cmd.FarmID,
		Metric:    cmd.Metric,
		Operator:  cmd.Operator,
		Value:     cmd.Value,
		IsEnabled: cmd.IsEnabled,
		UpdatedAt: time.Now(),
		CreatedAt: existingWeather.CreatedAt,
	}

	if err := uc.repo.Update(ctx, updatedWeather); err != nil {
		return nil, fmt.Errorf("failed to execute weather alert update: %w", err)
	}
	return updatedWeather, nil
}

func (uc *useCase) Delete(ctx context.Context, userID, farmID, weatherID uuid.UUID) error {
	weather, err := uc.repo.GetByID(ctx, weatherID)
	if err != nil {
		return fmt.Errorf("failed to get weather alert for deletion: %w", err)
	}

	if weather.FarmID != farmID {
		return fmt.Errorf("weather alert not found on specified farm")
	}
	if err := uc.checkFarmOwnership(ctx, userID, farmID); err != nil {
		return fmt.Errorf("cannot delete weather alert: %w", err)
	}

	if err := uc.repo.Delete(ctx, weatherID); err != nil {
		return fmt.Errorf("failed to execute weather alert deletion: %w", err)
	}
	return nil
}

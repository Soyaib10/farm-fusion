package recommendation

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type useCase struct {
	repo Repository
}

func NewUseCase(repo Repository) UseCase {
	return &useCase{repo: repo}
}

func (uc *useCase) CropRecommendation(ctx context.Context, cmd CropCommand) (*domain.CropRecommendation, error) {
	return uc.repo.CropRecommendation(ctx, cmd)
}

func (uc *useCase) FertilizerRecommendation(ctx context.Context, cmd FertilizerCommand) (*domain.FertilizerRecommendation, error) {
	return uc.repo.FertilizerRecommendation(ctx, cmd)
}

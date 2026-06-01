package recommendation

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type UseCase interface {
	CropRecommendation(ctx context.Context, cmd CropCommand) (*domain.CropRecommendation, error)
	FertilizerRecommendation(ctx context.Context, cmd FertilizerCommand) (*domain.FertilizerRecommendation, error)
}

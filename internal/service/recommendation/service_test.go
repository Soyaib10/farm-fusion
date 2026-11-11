package recommendation

import (
	"context"
	"testing"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type stubPredictor struct{}

func (stubPredictor) PredictCrop(_ context.Context, _ domain.CropQuery) (*domain.CropRecommendation, error) {
	return &domain.CropRecommendation{Crop: "rice", Confidence: 0.98}, nil
}

func (stubPredictor) PredictFertilizer(_ context.Context, _ domain.FertilizerQuery) (*domain.FertilizerRecommendation, error) {
	return &domain.FertilizerRecommendation{Fertilizer: "Urea", Confidence: 0.94}, nil
}

func TestRecommendationValidation(t *testing.T) {
	ctx := context.Background()
	svc := NewService(stubPredictor{})

	if _, err := svc.RecommendCrop(ctx, domain.CropQuery{
		N: 90, P: 42, K: 43, Temperature: 20.8, Humidity: 82, PH: 12.5, Rainfall: 202,
	}); err == nil {
		t.Fatal("expected validation error when pH > 10")
	}

	cropRec, err := svc.RecommendCrop(ctx, domain.CropQuery{
		N: 90, P: 42, K: 43, Temperature: 20.8, Humidity: 82, PH: 6.5, Rainfall: 202,
	})
	if err != nil || cropRec.Crop != "rice" {
		t.Fatalf("RecommendCrop() got %+v, %v", cropRec, err)
	}

	if _, err := svc.RecommendFertilizer(ctx, domain.FertilizerQuery{
		Temperature: 30, Humidity: 60, SoilMoisture: 40,
		SoilType: "Volcanic", CropType: "Maize",
		Nitrogen: 20, Potassium: 10, Phosphorous: 15,
	}); err == nil {
		t.Fatal("expected validation error for unsupported SoilType")
	}
}

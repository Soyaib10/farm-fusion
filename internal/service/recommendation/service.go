package recommendation

import (
	"context"
	"strings"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/validator"
)

// Predictor defines ML inference operations consumed by the recommendation service.
type Predictor interface {
	PredictCrop(ctx context.Context, query domain.CropQuery) (*domain.CropRecommendation, error)
	PredictFertilizer(ctx context.Context, query domain.FertilizerQuery) (*domain.FertilizerRecommendation, error)
}

// Service validates agronomic feature inputs and orchestrates ML recommendations.
type Service struct {
	predictor Predictor
}

// NewService constructs a recommendation Service.
func NewService(predictor Predictor) *Service {
	return &Service{predictor: predictor}
}

// RecommendCrop validates soil/climate features and returns crop recommendations.
func (s *Service) RecommendCrop(ctx context.Context, query domain.CropQuery) (*domain.CropRecommendation, error) {
	v := validator.New()
	v.Check(validator.Between(query.N, 0, 140), "N", "must be between 0 and 140")
	v.Check(validator.Between(query.P, 5, 145), "P", "must be between 5 and 145")
	v.Check(validator.Between(query.K, 5, 205), "K", "must be between 5 and 205")
	v.Check(validator.Between(query.Temperature, 8, 44), "temperature", "must be between 8 and 44 °C")
	v.Check(validator.Between(query.Humidity, 14, 100), "humidity", "must be between 14 and 100%")
	v.Check(validator.Between(query.PH, 3.5, 10), "ph", "must be between 3.5 and 10")
	v.Check(validator.Between(query.Rainfall, 20, 300), "rainfall", "must be between 20 and 300 mm")

	if err := v.Err(); err != nil {
		return nil, err
	}
	return s.predictor.PredictCrop(ctx, query)
}

// RecommendFertilizer validates soil, crop, and nutrient features and returns fertilizer recommendations.
func (s *Service) RecommendFertilizer(ctx context.Context, query domain.FertilizerQuery) (*domain.FertilizerRecommendation, error) {
	query.SoilType = strings.TrimSpace(query.SoilType)
	query.CropType = strings.TrimSpace(query.CropType)

	v := validator.New()
	v.Check(validator.Between(query.Temperature, 25, 38), "temperature", "must be between 25 and 38 °C")
	v.Check(validator.Between(query.Humidity, 50, 72), "humidity", "must be between 50 and 72%")
	v.Check(validator.Between(query.SoilMoisture, 10, 65), "soil_moisture", "must be between 10 and 65")
	v.Check(validator.In(query.SoilType, domain.AllowedSoilTypes...), "soil_type",
		"must be one of: Sandy, Loamy, Black, Red, Clayey")
	v.Check(validator.In(query.CropType, domain.AllowedCropTypes...), "crop_type",
		"must be a supported crop type")
	v.Check(validator.Between(query.Nitrogen, 0, 39), "nitrogen", "must be between 0 and 39")
	v.Check(validator.Between(query.Potassium, 0, 19), "potassium", "must be between 0 and 19")
	v.Check(validator.Between(query.Phosphorous, 0, 42), "phosphorous", "must be between 0 and 42")

	if err := v.Err(); err != nil {
		return nil, err
	}
	return s.predictor.PredictFertilizer(ctx, query)
}

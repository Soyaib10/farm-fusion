package recommendation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/validator"
)

// Predictor defines ML inference operations consumed by the recommendation service.
type Predictor interface {
	PredictCrop(ctx context.Context, query domain.CropQuery) (*domain.CropRecommendation, error)
	PredictFertilizer(ctx context.Context, query domain.FertilizerQuery) (*domain.FertilizerRecommendation, error)
}

type bound struct{ min, max float64 }

// Input bounds must match ml_service/feature_bounds.json (enforced by TestBoundsMatchMLSpec).
var (
	cropBounds = map[string]bound{
		"N":           {0, 140},
		"P":           {5, 145},
		"K":           {5, 205},
		"temperature": {8, 44},
		"humidity":    {14, 100},
		"ph":          {3.5, 10},
		"rainfall":    {20, 300},
	}
	fertilizerBounds = map[string]bound{
		"temperature":   {25, 38},
		"humidity":      {50, 72},
		"soil_moisture": {25, 65},
		"nitrogen":      {0, 42},
		"potassium":     {0, 19},
		"phosphorous":   {0, 42},
	}
)

// Service validates agronomic feature inputs and orchestrates ML recommendations.
type Service struct {
	predictor Predictor
}

// NewService constructs a recommendation Service.
func NewService(predictor Predictor) *Service {
	return &Service{predictor: predictor}
}

func checkBounds(v *validator.Validator, bounds map[string]bound, values map[string]float64) {
	for field, val := range values {
		b := bounds[field]
		v.Check(validator.Between(val, b.min, b.max), field, fmt.Sprintf("must be between %g and %g", b.min, b.max))
	}
}

// RecommendCrop validates soil/climate features and returns crop recommendations.
func (s *Service) RecommendCrop(ctx context.Context, query domain.CropQuery) (*domain.CropRecommendation, error) {
	v := validator.New()
	checkBounds(v, cropBounds, map[string]float64{
		"N":           query.N,
		"P":           query.P,
		"K":           query.K,
		"temperature": query.Temperature,
		"humidity":    query.Humidity,
		"ph":          query.PH,
		"rainfall":    query.Rainfall,
	})

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
	checkBounds(v, fertilizerBounds, map[string]float64{
		"temperature":   query.Temperature,
		"humidity":      query.Humidity,
		"soil_moisture": query.SoilMoisture,
		"nitrogen":      query.Nitrogen,
		"potassium":     query.Potassium,
		"phosphorous":   query.Phosphorous,
	})
	v.Check(validator.In(query.SoilType, domain.AllowedSoilTypes...), "soil_type",
		"must be one of: Sandy, Loamy, Black, Red, Clayey")
	v.Check(validator.In(query.CropType, domain.AllowedCropTypes...), "crop_type",
		"must be a supported crop type")

	if err := v.Err(); err != nil {
		return nil, err
	}
	return s.predictor.PredictFertilizer(ctx, query)
}

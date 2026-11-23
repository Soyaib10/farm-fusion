package recommendation

import (
	"context"
	"encoding/json"
	"os"
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

	// The training data contains nitrogen up to 42 (labelled Urea).
	if _, err := svc.RecommendFertilizer(ctx, domain.FertilizerQuery{
		Temperature: 30, Humidity: 60, SoilMoisture: 40,
		SoilType: "Sandy", CropType: "Maize",
		Nitrogen: 42, Potassium: 0, Phosphorous: 0,
	}); err != nil {
		t.Fatalf("nitrogen=42 must be accepted, got %v", err)
	}
	// Soil moisture below 25 was never seen in training.
	if _, err := svc.RecommendFertilizer(ctx, domain.FertilizerQuery{
		Temperature: 30, Humidity: 60, SoilMoisture: 10,
		SoilType: "Sandy", CropType: "Maize",
		Nitrogen: 20, Potassium: 10, Phosphorous: 15,
	}); err == nil {
		t.Fatal("expected validation error for soil_moisture=10")
	}
}

func TestBoundsMatchMLSpec(t *testing.T) {
	raw, err := os.ReadFile("../../../ml_service/feature_bounds.json")
	if err != nil {
		t.Fatalf("read shared bounds spec: %v", err)
	}
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}

	for name, goBounds := range map[string]map[string]bound{"crop": cropBounds, "fertilizer": fertilizerBounds} {
		var want map[string][2]float64
		if err := json.Unmarshal(spec[name], &want); err != nil {
			t.Fatalf("decode %s bounds: %v", name, err)
		}
		if len(want) != len(goBounds) {
			t.Fatalf("%s: spec has %d fields, Go has %d", name, len(want), len(goBounds))
		}
		for field, w := range want {
			g, ok := goBounds[field]
			if !ok || g.min != w[0] || g.max != w[1] {
				t.Fatalf("%s.%s: Go bounds %+v differ from spec %v", name, field, g, w)
			}
		}
	}
}

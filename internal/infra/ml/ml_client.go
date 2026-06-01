package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/recommendation"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
)

type MLClient struct {
	baseURL string
	client  *http.Client
	logger  *logger.Logger
}

func NewMLClient(baseURL string, client *http.Client, logger *logger.Logger) recommendation.Repository {
	return &MLClient{
		baseURL: baseURL,
		client:  client,
		logger:  logger,
	}
}

func (c *MLClient) CropRecommendation(ctx context.Context, cmd recommendation.CropCommand) (*domain.CropRecommendation, error) {
	reqBody := map[string]float64{
		"N":           cmd.N,
		"P":           cmd.P,
		"K":           cmd.K,
		"temperature": cmd.Temperature,
		"humidity":    cmd.Humidity,
		"ph":          cmd.PH,
		"rainfall":    cmd.Rainfall,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "marshal_crop_request"})
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/predict/crop", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "create_crop_request"})
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "ml_crop_request", "url": c.baseURL + "/predict/crop"})
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("ml service returned status %d", resp.StatusCode)
		c.logger.PrintError(err, map[string]string{"operation": "ml_crop_response", "status": fmt.Sprintf("%d", resp.StatusCode)})
		return nil, err
	}

	var result struct {
		Crop         string  `json:"crop"`
		Confidence   float64 `json:"confidence"`
		Alternatives []struct {
			Crop       string  `json:"crop"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
		Warning string `json:"warning"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "decode_crop_response"})
		return nil, fmt.Errorf("decode response: %w", err)
	}

	alternatives := make([]domain.Alternative, len(result.Alternatives))
	for i, alt := range result.Alternatives {
		alternatives[i] = domain.Alternative{
			Name:       alt.Crop,
			Confidence: alt.Confidence,
		}
	}

	return &domain.CropRecommendation{
		Crop:         result.Crop,
		Confidence:   result.Confidence,
		Alternatives: alternatives,
		Warning:      result.Warning,
	}, nil
}

func (c *MLClient) FertilizerRecommendation(ctx context.Context, cmd recommendation.FertilizerCommand) (*domain.FertilizerRecommendation, error) {
	reqBody := map[string]interface{}{
		"temperature":   cmd.Temperature,
		"humidity":      cmd.Humidity,
		"soil_moisture": cmd.SoilMoisture,
		"soil_type":     cmd.SoilType,
		"crop_type":     cmd.CropType,
		"nitrogen":      cmd.Nitrogen,
		"potassium":     cmd.Potassium,
		"phosphorous":   cmd.Phosphorous,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "marshal_fertilizer_request"})
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/predict/fertilizer", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "create_fertilizer_request"})
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "ml_fertilizer_request", "url": c.baseURL + "/predict/fertilizer"})
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("ml service returned status %d", resp.StatusCode)
		c.logger.PrintError(err, map[string]string{"operation": "ml_fertilizer_response", "status": fmt.Sprintf("%d", resp.StatusCode)})
		return nil, err
	}

	var result struct {
		Fertilizer   string  `json:"fertilizer"`
		Confidence   float64 `json:"confidence"`
		Alternatives []struct {
			Fertilizer string  `json:"fertilizer"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
		Warning string `json:"warning"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "decode_fertilizer_response"})
		return nil, fmt.Errorf("decode response: %w", err)
	}

	alternatives := make([]domain.Alternative, len(result.Alternatives))
	for i, alt := range result.Alternatives {
		alternatives[i] = domain.Alternative{
			Name:       alt.Fertilizer,
			Confidence: alt.Confidence,
		}
	}

	return &domain.FertilizerRecommendation{
		Fertilizer:   result.Fertilizer,
		Confidence:   result.Confidence,
		Alternatives: alternatives,
		Warning:      result.Warning,
	}, nil
}

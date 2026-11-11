package mlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
)

const (
	maxResponseBytes  = 1 << 20 // 1 MiB
	internalKeyHeader = "X-Internal-API-Key"
)

// Client communicates with the internal Python FastAPI ML inference service.
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
	log     *logger.Logger
}

// NewClient constructs an ML inference HTTP Client.
func NewClient(baseURL, apiKey string, httpClient *http.Client, log *logger.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  httpClient,
		log:     log,
	}
}

// PredictCrop calls POST /predict/crop on the ML service.
func (c *Client) PredictCrop(ctx context.Context, query domain.CropQuery) (*domain.CropRecommendation, error) {
	payload := map[string]float64{
		"N":           query.N,
		"P":           query.P,
		"K":           query.K,
		"temperature": query.Temperature,
		"humidity":    query.Humidity,
		"ph":          query.PH,
		"rainfall":    query.Rainfall,
	}

	var raw struct {
		Crop         string  `json:"crop"`
		Confidence   float64 `json:"confidence"`
		Alternatives []struct {
			Crop       string  `json:"crop"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
		Warning string `json:"warning"`
	}

	if err := c.postJSON(ctx, "/predict/crop", payload, &raw); err != nil {
		return nil, err
	}

	alternatives := make([]domain.Alternative, len(raw.Alternatives))
	for i, alt := range raw.Alternatives {
		alternatives[i] = domain.Alternative{
			Name:       alt.Crop,
			Confidence: alt.Confidence,
		}
	}

	return &domain.CropRecommendation{
		Crop:         raw.Crop,
		Confidence:   raw.Confidence,
		Alternatives: alternatives,
		Warning:      raw.Warning,
	}, nil
}

// PredictFertilizer calls POST /predict/fertilizer on the ML service.
func (c *Client) PredictFertilizer(ctx context.Context, query domain.FertilizerQuery) (*domain.FertilizerRecommendation, error) {
	payload := map[string]any{
		"temperature":   query.Temperature,
		"humidity":      query.Humidity,
		"soil_moisture": query.SoilMoisture,
		"soil_type":     query.SoilType,
		"crop_type":     query.CropType,
		"nitrogen":      query.Nitrogen,
		"potassium":     query.Potassium,
		"phosphorous":   query.Phosphorous,
	}

	var raw struct {
		Fertilizer   string  `json:"fertilizer"`
		Confidence   float64 `json:"confidence"`
		Alternatives []struct {
			Fertilizer string  `json:"fertilizer"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
		Warning string `json:"warning"`
	}

	if err := c.postJSON(ctx, "/predict/fertilizer", payload, &raw); err != nil {
		return nil, err
	}

	alternatives := make([]domain.Alternative, len(raw.Alternatives))
	for i, alt := range raw.Alternatives {
		alternatives[i] = domain.Alternative{
			Name:       alt.Fertilizer,
			Confidence: alt.Confidence,
		}
	}

	return &domain.FertilizerRecommendation{
		Fertilizer:   raw.Fertilizer,
		Confidence:   raw.Confidence,
		Alternatives: alternatives,
		Warning:      raw.Warning,
	}, nil
}

func (c *Client) postJSON(ctx context.Context, path string, reqBody any, dst any) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal ml request: %w", err)
	}

	targetURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create ml request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set(internalKeyHeader, c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error(err, map[string]string{"operation": "ml_http_request", "path": path})
		return fmt.Errorf("%w: ml service unreachable", domain.ErrUpstreamUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("%w: ml service returned status %d", domain.ErrUpstreamUnavailable, resp.StatusCode)
		c.log.Error(err, map[string]string{"operation": "ml_http_response", "path": path})
		return err
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(dst); err != nil {
		c.log.Error(err, map[string]string{"operation": "ml_decode_response", "path": path})
		return fmt.Errorf("%w: invalid ml service response", domain.ErrUpstreamUnavailable)
	}
	return nil
}

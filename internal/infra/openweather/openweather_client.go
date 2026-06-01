package openweather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/notification"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cache      notification.ForecastRepository
	logger     *logger.Logger
}

func NewClient(baseURL, apiKey string, cache notification.ForecastRepository, logger *logger.Logger) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      cache,
		logger:     logger,
	}
}

type owmResponse struct {
	List []struct {
		Dt   int64 `json:"dt"`
		Main struct {
			Temp     float64 `json:"temp"`
			Humidity float64 `json:"humidity"`
		} `json:"main"`
		Rain struct {
			ThreeH float64 `json:"3h"`
		} `json:"rain"`
		Wind struct {
			Speed float64 `json:"speed"` // m/s
		} `json:"wind"`
	} `json:"list"`
}

func (c *Client) FetchForecast(ctx context.Context, lat, lon float64, locationKey string) (*domain.WeatherForecast, error) {
	// cache-aside: check Redis first
	cached, err := c.cache.Get(ctx, locationKey)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, notification.ErrCacheMiss) {
		c.logger.PrintError(err, map[string]string{"operation": "forecast_cache_get", "location_key": locationKey})
	}

	if c.apiKey == "" {
		return nil, fmt.Errorf("openweather api key is required")
	}

	// fetch 24 hours of 3-hour forecast points
	url := fmt.Sprintf("%s?lat=%f&lon=%f&appid=%s&units=metric&cnt=8", c.baseURL, lat, lon, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch forecast: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openweather returned status %d", resp.StatusCode)
	}

	var owm owmResponse
	if err := json.NewDecoder(resp.Body).Decode(&owm); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	points := make([]domain.ForecastPoint, len(owm.List))
	for i, item := range owm.List {
		points[i] = domain.ForecastPoint{
			Timestamp:   time.Unix(item.Dt, 0).UTC(),
			Temperature: item.Main.Temp,
			Humidity:    item.Main.Humidity,
			Rainfall:    item.Rain.ThreeH,
			WindSpeed:   item.Wind.Speed * 3.6, // m/s → km/h
		}
	}

	forecast := &domain.WeatherForecast{
		LocationKey: locationKey,
		Latitude:    lat,
		Longitude:   lon,
		DataPoints:  points,
		FetchedAt:   time.Now().UTC(),
	}

	// store in cache, non-fatal on error
	if err := c.cache.Set(ctx, forecast); err != nil {
		c.logger.PrintError(err, map[string]string{"operation": "forecast_cache_set", "location_key": locationKey})
	}

	return forecast, nil
}

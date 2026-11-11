package weatherapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
)

const maxResponseBytes = 1 << 20 // 1 MiB

// ForecastCache defines the cache operations used by the OpenWeather client.
type ForecastCache interface {
	Get(ctx context.Context, locationKey string) (*domain.WeatherForecast, error)
	Set(ctx context.Context, forecast *domain.WeatherForecast) error
}

// Client fetches 24-hour forecasts from OpenWeather with Redis cache-aside.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cache      ForecastCache
	log        *logger.Logger
}

// NewClient constructs an OpenWeather Client.
func NewClient(baseURL, apiKey string, cache ForecastCache, log *logger.Logger) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      cache,
		log:        log,
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

// FetchForecast returns a 24-hour weather forecast for (lat, lon), consulting cache first.
func (c *Client) FetchForecast(ctx context.Context, lat, lon float64, locationKey string) (*domain.WeatherForecast, error) {
	if c.cache != nil {
		cached, err := c.cache.Get(ctx, locationKey)
		if err == nil {
			return cached, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			c.log.Error(err, map[string]string{"operation": "forecast_cache_get", "location_key": locationKey})
		}
	}

	if c.apiKey == "" {
		return nil, errors.New("openweather api key is required")
	}

	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, errors.New("invalid openweather base url")
	}
	q := endpoint.Query()
	q.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	q.Set("lon", strconv.FormatFloat(lon, 'f', 6, 64))
	q.Set("appid", c.apiKey)
	q.Set("units", "metric")
	q.Set("cnt", "8")
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, errors.New("create openweather request failed")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Never propagate *url.Error raw URL because it contains appid=<API_KEY>.
		return nil, sanitizeTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openweather returned status %d", resp.StatusCode)
	}

	var owm owmResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&owm); err != nil {
		return nil, fmt.Errorf("decode openweather response: %w", err)
	}

	points := make([]domain.ForecastPoint, len(owm.List))
	for i, item := range owm.List {
		points[i] = domain.ForecastPoint{
			Timestamp:   time.Unix(item.Dt, 0).UTC(),
			Temperature: item.Main.Temp,
			Humidity:    item.Main.Humidity,
			Rainfall:    item.Rain.ThreeH,
			WindSpeed:   item.Wind.Speed * 3.6, // Convert m/s to km/h.
		}
	}

	forecast := &domain.WeatherForecast{
		LocationKey: locationKey,
		Latitude:    lat,
		Longitude:   lon,
		DataPoints:  points,
		FetchedAt:   time.Now().UTC(),
	}

	if c.cache != nil {
		if err := c.cache.Set(ctx, forecast); err != nil {
			c.log.Error(err, map[string]string{"operation": "forecast_cache_set", "location_key": locationKey})
		}
	}

	return forecast, nil
}

func sanitizeTransportError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("openweather %s request failed: %w", urlErr.Op, urlErr.Err)
	}
	return errors.New("openweather request failed")
}

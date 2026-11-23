package weatherapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
)

func TestFetchForecastDoesNotLeakAPIKeyOnNetworkError(t *testing.T) {
	const secretAPIKey = "SUPER_SECRET_OPENWEATHER_KEY_99999"
	log := logger.New(io.Discard, logger.LevelOff)

	// Point client to a closed local server so httpClient.Do fails with *url.Error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	client := NewClient(closedURL, secretAPIKey, nil, log)
	_, err := client.FetchForecast(context.Background(), 23.81, 90.41, "23.81_90.41")
	if err == nil {
		t.Fatal("expected network error from closed server")
	}
	if strings.Contains(err.Error(), secretAPIKey) {
		t.Fatalf("API key leaked in error message: %v", err)
	}
}

func TestFetchForecastParsesAndConvertsUnits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appid") != "test-key" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"list": [{
				"dt": 1700000000,
				"main": {"temp": 30.5, "humidity": 75},
				"rain": {"3h": 12.4},
				"wind": {"speed": 10.0}
			}]
		}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", nil, logger.New(io.Discard, logger.LevelOff))
	fc, err := client.FetchForecast(context.Background(), 23.81, 90.41, "23.81_90.41")
	if err != nil {
		t.Fatalf("FetchForecast unexpected error: %v", err)
	}
	if len(fc.DataPoints) != 1 {
		t.Fatalf("got %d points, want 1", len(fc.DataPoints))
	}
	if fc.DataPoints[0].WindSpeed != 36.0 {
		t.Fatalf("got wind speed %v km/h, want 36.0 km/h", fc.DataPoints[0].WindSpeed)
	}
}

type recordingCache struct {
	stored  *domain.WeatherForecast
	getResp *domain.WeatherForecast
}

func (c *recordingCache) Get(context.Context, string) (*domain.WeatherForecast, error) {
	if c.getResp == nil {
		return nil, domain.ErrNotFound
	}
	return c.getResp, nil
}

func (c *recordingCache) Set(_ context.Context, f *domain.WeatherForecast) error {
	c.stored = f
	return nil
}

func TestFetchForecastRejectsEmptyListAndDoesNotCacheIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"list": []}`))
	}))
	defer srv.Close()

	cache := &recordingCache{}
	client := NewClient(srv.URL, "test-key", cache, logger.New(io.Discard, logger.LevelOff))
	if _, err := client.FetchForecast(context.Background(), 23.81, 90.41, "23.81_90.41"); err == nil {
		t.Fatal("expected an error for an empty forecast list")
	}
	if cache.stored != nil {
		t.Fatal("empty forecast must not be cached")
	}
}

func TestFetchForecastIgnoresEmptyCacheHit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"list": [{"dt": 1700000000, "main": {"temp": 30}}]}`))
	}))
	defer srv.Close()

	cache := &recordingCache{getResp: &domain.WeatherForecast{FetchedAt: time.Now()}}
	client := NewClient(srv.URL, "test-key", cache, logger.New(io.Discard, logger.LevelOff))
	fc, err := client.FetchForecast(context.Background(), 23.81, 90.41, "23.81_90.41")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(fc.DataPoints) != 1 {
		t.Fatalf("expected a refetch past the empty cache entry, calls=%d points=%d", calls, len(fc.DataPoints))
	}
}

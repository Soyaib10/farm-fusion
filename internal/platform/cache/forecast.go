package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/redis/go-redis/v9"
)

// ForecastCache stores and retrieves weather forecasts in Redis.
type ForecastCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewForecastCache connects to Redis, verifies connectivity, and returns a ForecastCache.
func NewForecastCache(cfg config.RedisConfig, ttl time.Duration, log *logger.Logger) (*ForecastCache, error) {
	opts := &redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	if cfg.URL != "" {
		parsed, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, errors.New("invalid Redis connection URL")
		}
		opts = parsed
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	log.Info("Connected to Redis", nil)
	return &ForecastCache{
		client: client,
		ttl:    ttl,
	}, nil
}

// Get fetches a cached forecast by locationKey, returning domain.ErrNotFound on cache miss.
func (c *ForecastCache) Get(ctx context.Context, locationKey string) (*domain.WeatherForecast, error) {
	data, err := c.client.Get(ctx, forecastKey(locationKey)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var forecast domain.WeatherForecast
	if err := json.Unmarshal(data, &forecast); err != nil {
		return nil, fmt.Errorf("unmarshal forecast: %w", err)
	}
	return &forecast, nil
}

// Set caches a weather forecast under its LocationKey for the configured TTL.
func (c *ForecastCache) Set(ctx context.Context, forecast *domain.WeatherForecast) error {
	data, err := json.Marshal(forecast)
	if err != nil {
		return fmt.Errorf("marshal forecast: %w", err)
	}
	return c.client.Set(ctx, forecastKey(forecast.LocationKey), data, c.ttl).Err()
}

// Close closes the underlying Redis client connection pool.
func (c *ForecastCache) Close() error {
	return c.client.Close()
}

func forecastKey(locationKey string) string {
	return "forecast:" + locationKey
}

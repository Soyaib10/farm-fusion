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
	if ttl <= 0 {
		return nil, errors.New("forecast cache TTL must be positive")
	}
	opts, err := redisOptions(cfg)
	if err != nil {
		return nil, err
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

// redisOptions builds client options. When REDIS_URL is set it wins, but a password or DB given
// separately fills in what the URL omits, and the default timeouts still apply.
func redisOptions(cfg config.RedisConfig) (*redis.Options, error) {
	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.URL != "" {
		parsed, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, errors.New("invalid Redis connection URL")
		}
		if parsed.Password == "" {
			parsed.Password = cfg.Password
		}
		if parsed.DB == 0 {
			parsed.DB = cfg.DB
		}
		opts = parsed
	}
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 5 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 3 * time.Second
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 3 * time.Second
	}
	return opts, nil
}

// Get fetches a cached forecast by locationKey, returning domain.ErrNotFound on a miss. Entries that
// are empty or older than the TTL are deleted and reported as a miss so the caller refetches.
func (c *ForecastCache) Get(ctx context.Context, locationKey string) (*domain.WeatherForecast, error) {
	key := forecastKey(locationKey)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var forecast domain.WeatherForecast
	if err := json.Unmarshal(data, &forecast); err != nil || !c.usable(&forecast) {
		_ = c.client.Del(ctx, key).Err()
		return nil, domain.ErrNotFound
	}
	return &forecast, nil
}

func (c *ForecastCache) usable(f *domain.WeatherForecast) bool {
	return len(f.DataPoints) > 0 && !f.FetchedAt.IsZero() && time.Since(f.FetchedAt) <= c.ttl
}

// Set caches a weather forecast under its LocationKey for the configured TTL.
func (c *ForecastCache) Set(ctx context.Context, forecast *domain.WeatherForecast) error {
	if len(forecast.DataPoints) == 0 {
		return errors.New("refusing to cache a forecast with no data points")
	}
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

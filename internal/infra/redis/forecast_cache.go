package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/notification"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type ForecastCache struct {
	client *redis.Client
	ttl    time.Duration
	logger *logger.Logger
}

func NewForecastCache(cfg *config.Config, logger *logger.Logger) (notification.ForecastRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	logger.PrintInfo("Successfully connected to Redis", nil)

	return &ForecastCache{
		client: client,
		ttl:    time.Duration(cfg.Weather.CacheTTL) * time.Second,
		logger: logger,
	}, nil
}

func (c *ForecastCache) Get(ctx context.Context, locationKey string) (*domain.WeatherForecast, error) {
	data, err := c.client.Get(ctx, locationKey).Bytes()
	if err != nil {
		return nil, err // caller checks redis.Nil
	}

	var forecast domain.WeatherForecast
	if err := json.Unmarshal(data, &forecast); err != nil {
		return nil, fmt.Errorf("unmarshal forecast: %w", err)
	}

	return &forecast, nil
}

func (c *ForecastCache) Set(ctx context.Context, forecast *domain.WeatherForecast) error {
	data, err := json.Marshal(forecast)
	if err != nil {
		return fmt.Errorf("marshal forecast: %w", err)
	}

	return c.client.Set(ctx, forecast.LocationKey, data, c.ttl).Err()
}

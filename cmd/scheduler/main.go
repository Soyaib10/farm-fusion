package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/infra/openweather"
	"github.com/Soyaib10/farm-fusion/internal/infra/postgres"
	"github.com/Soyaib10/farm-fusion/internal/infra/rabbitmq"
	forecastcache "github.com/Soyaib10/farm-fusion/internal/infra/redis"
	"github.com/Soyaib10/farm-fusion/internal/usecase/notification"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logger.NewLogger(os.Stdout, logger.LevelInfo)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.ConnectDB(ctx, cfg, logger)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	cache, err := forecastcache.NewForecastCache(cfg, logger)
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer cache.Close()

	publisher, err := rabbitmq.NewPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.Queue, logger)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer publisher.Close()

	farmRepo := postgres.NewFarmRepository(db)
	weatherRepo := postgres.NewWeatherRepository(db)
	userRepo := postgres.NewUserRepository(db)
	forecastClient := openweather.NewClient(cfg.Weather.BaseURL, cfg.Weather.APIKey, cache, logger)
	scheduler := notification.NewSchedulerUseCase(farmRepo, weatherRepo, userRepo, forecastClient, publisher)

	if getEnvBool("SCHEDULER_RUN_ONCE", false) {
		run(ctx, logger, scheduler)
		return
	}

	logger.PrintInfo("Weather scheduler started", map[string]string{"schedule": "daily 05:00 local time"})
	for {
		next := nextDailyRun(time.Now(), 5)
		logger.PrintInfo("Next weather scheduler run planned", map[string]string{"run_at": next.Format(time.RFC3339)})

		select {
		case <-ctx.Done():
			logger.PrintInfo("Weather scheduler stopped", nil)
			return
		case <-time.After(time.Until(next)):
			run(ctx, logger, scheduler)
		}
	}
}

func run(ctx context.Context, logger *logger.Logger, scheduler notification.SchedulerUseCase) {
	started := time.Now()
	result, err := scheduler.RunScheduled(ctx)
	if err != nil {
		logger.PrintError(err, map[string]string{"operation": "run_weather_scheduler"})
		return
	}

	props := map[string]string{
		"farms_scanned":           strconv.Itoa(result.FarmsScanned),
		"forecasts_fetched":       strconv.Itoa(result.ForecastsFetched),
		"notifications_published": strconv.Itoa(result.NotificationsPublished),
		"per_farm_error_count":    strconv.Itoa(len(result.Errors)),
		"duration_ms":             strconv.FormatInt(time.Since(started).Milliseconds(), 10),
	}
	logger.PrintInfo("Weather scheduler completed", props)
	for _, message := range result.Errors {
		logger.PrintError(logError(message), map[string]string{"operation": "weather_scheduler_partial_failure"})
	}
}

func nextDailyRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

type logError string

func (e logError) Error() string { return string(e) }

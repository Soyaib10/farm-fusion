package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/platform/cache"
	"github.com/Soyaib10/farm-fusion/internal/platform/database"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/platform/queue"
	"github.com/Soyaib10/farm-fusion/internal/platform/weatherapi"
	"github.com/Soyaib10/farm-fusion/internal/repository/postgres"
	"github.com/Soyaib10/farm-fusion/internal/service/notification"
)

func main() {
	log := logger.New(os.Stdout, logger.LevelInfo)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err, map[string]string{"operation": "load_config"})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.Database, log)
	if err != nil {
		log.Fatal(err, map[string]string{"operation": "connect_database"})
	}
	defer db.Close()

	forecastCache, err := cache.NewForecastCache(cfg.Redis, cfg.Weather.CacheTTL, log)
	if err != nil {
		log.Fatal(err, map[string]string{"operation": "connect_redis"})
	}
	defer forecastCache.Close()

	publisher, err := queue.NewPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.Queue, log)
	if err != nil {
		log.Fatal(err, map[string]string{"operation": "connect_rabbitmq"})
	}
	defer publisher.Close()

	farmRepo := postgres.NewFarmRepository(db)
	weatherRepo := postgres.NewWeatherRepository(db)
	userRepo := postgres.NewUserRepository(db)
	forecastClient := weatherapi.NewClient(cfg.Weather.BaseURL, cfg.Weather.APIKey, forecastCache, log)
	scheduler := notification.NewScheduler(farmRepo, weatherRepo, userRepo, forecastClient, publisher)

	if runOnceEnabled() {
		run(ctx, log, scheduler)
		return
	}

	log.Info("Weather scheduler started", map[string]string{"schedule": "daily 05:00 local time"})
	for {
		next := nextDailyRun(time.Now(), 5)
		log.Info("Next weather scheduler run planned", map[string]string{"run_at": next.Format(time.RFC3339)})

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Info("Weather scheduler stopped", nil)
			return
		case <-timer.C:
			run(ctx, log, scheduler)
		}
	}
}

func run(ctx context.Context, log *logger.Logger, scheduler *notification.Scheduler) {
	started := time.Now()
	result, err := scheduler.RunScheduled(ctx)
	if err != nil {
		log.Error(err, map[string]string{"operation": "run_weather_scheduler"})
		return
	}

	log.Info("Weather scheduler completed", map[string]string{
		"farms_scanned":           strconv.Itoa(result.FarmsScanned),
		"forecasts_fetched":       strconv.Itoa(result.ForecastsFetched),
		"notifications_published": strconv.Itoa(result.NotificationsPublished),
		"per_farm_error_count":    strconv.Itoa(len(result.Errors)),
		"duration_ms":             strconv.FormatInt(time.Since(started).Milliseconds(), 10),
	})
	for _, msg := range result.Errors {
		log.Error(errors.New(msg), map[string]string{"operation": "weather_scheduler_partial_failure"})
	}
}

func nextDailyRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func runOnceEnabled() bool {
	val := os.Getenv("SCHEDULER_RUN_ONCE")
	if val == "" {
		return false
	}
	parsed, err := strconv.ParseBool(val)
	return err == nil && parsed
}

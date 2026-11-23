package main

import (
	"context"
	"errors"
	"fmt"
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

// retryBackoff is the wait before each retry of a failed or partially failed run. Retries only
// redo farms that did not publish: the outbox skips farms already published today.
var retryBackoff = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

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
	if fixed, err := farmRepo.ReconcileLocationKeys(ctx); err != nil {
		log.Error(err, map[string]string{"operation": "reconcile_location_keys"})
	} else if fixed > 0 {
		log.Info("Recomputed farm location keys", map[string]string{"farms_updated": strconv.Itoa(fixed)})
	}

	forecastClient := weatherapi.NewClient(cfg.Weather.BaseURL, cfg.Weather.APIKey, forecastCache, log)
	outbox := postgres.NewNotificationOutboxRepository(db)
	scheduler := notification.NewScheduler(farmRepo, forecastClient, publisher, outbox, cfg.Scheduler.Location)
	lock := postgres.NewAdvisoryLock(db, postgres.SchedulerLockKey)

	if runOnceEnabled() {
		if err := runWithRetries(ctx, log, lock, scheduler); err != nil {
			log.Fatal(err, map[string]string{"operation": "run_weather_scheduler"})
		}
		return
	}

	loc := cfg.Scheduler.Location
	log.Info("Weather scheduler started", map[string]string{
		"schedule": fmt.Sprintf("daily %02d:00 %s", cfg.Scheduler.Hour, loc),
	})
	for {
		next := nextDailyRun(time.Now().In(loc), cfg.Scheduler.Hour)
		log.Info("Next weather scheduler run planned", map[string]string{"run_at": next.Format(time.RFC3339)})

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Info("Weather scheduler stopped", nil)
			return
		case err := <-publisher.Failed():
			timer.Stop()
			log.Fatal(err, map[string]string{"operation": "rabbitmq_publisher"})
		case <-timer.C:
			if err := runWithRetries(ctx, log, lock, scheduler); err != nil && !errors.Is(err, context.Canceled) {
				log.Error(err, map[string]string{"operation": "run_weather_scheduler"})
			}
		}
	}
}

func runWithRetries(ctx context.Context, log *logger.Logger, lock *postgres.AdvisoryLock, scheduler *notification.Scheduler) error {
	for attempt := 0; ; attempt++ {
		err := runLocked(ctx, log, lock, scheduler)
		if err == nil || attempt >= len(retryBackoff) || ctx.Err() != nil {
			return err
		}
		wait := retryBackoff[attempt]
		log.Error(err, map[string]string{"operation": "run_weather_scheduler", "retry_in": wait.String()})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// runLocked executes one run while holding the cluster-wide scheduler lock. If another scheduler
// holds the lock, that process is doing the run and this one returns without error.
func runLocked(ctx context.Context, log *logger.Logger, lock *postgres.AdvisoryLock, scheduler *notification.Scheduler) error {
	release, acquired, err := lock.TryAcquire(ctx)
	if err != nil {
		return err
	}
	if !acquired {
		log.Info("Another weather scheduler holds the lock; skipping this run", nil)
		return nil
	}
	defer release()

	started := time.Now()
	result, err := scheduler.RunScheduled(ctx)
	if result != nil {
		log.Info("Weather scheduler finished", map[string]string{
			"farms_scanned":           strconv.Itoa(result.FarmsScanned),
			"forecasts_fetched":       strconv.Itoa(result.ForecastsFetched),
			"notifications_published": strconv.Itoa(result.NotificationsPublished),
			"already_published":       strconv.Itoa(result.AlreadyPublished),
			"per_farm_error_count":    strconv.Itoa(len(result.Errors)),
			"duration_ms":             strconv.FormatInt(time.Since(started).Milliseconds(), 10),
		})
		for _, msg := range result.Errors {
			log.Error(errors.New(msg), map[string]string{"operation": "weather_scheduler_partial_failure"})
		}
	}
	if err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("weather scheduler run had %d farm failure(s)", len(result.Errors))
	}
	return nil
}

// nextDailyRun returns the next hour:00 in now's location at or after now. Days are stepped on the
// calendar, not by 24h, so the run stays at hour:00 local time across DST changes.
func nextDailyRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if next.Before(now) {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, hour, 0, 0, 0, now.Location())
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

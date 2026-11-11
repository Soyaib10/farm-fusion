package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/database"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/platform/mailer"
	"github.com/Soyaib10/farm-fusion/internal/platform/queue"
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

	consumer, err := queue.NewConsumer(cfg.RabbitMQ.URL, cfg.RabbitMQ.Queue, log)
	if err != nil {
		log.Fatal(err, map[string]string{"operation": "connect_rabbitmq"})
	}
	defer consumer.Close()

	logRepo := postgres.NewNotificationLogRepository(db)
	emailSender := mailer.NewSender(cfg.SMTP)
	processor := notification.NewProcessor(emailSender, logRepo, log)

	log.Info("Weather notification worker started", map[string]string{"queue": cfg.RabbitMQ.Queue})
	if err := consumer.Consume(ctx, func(payload *domain.NotificationPayload) error {
		return processor.Process(ctx, payload)
	}); err != nil {
		log.Fatal(err, map[string]string{"operation": "consume_weather_notifications"})
	}
}

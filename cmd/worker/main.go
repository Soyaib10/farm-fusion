package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/infra/postgres"
	"github.com/Soyaib10/farm-fusion/internal/infra/rabbitmq"
	"github.com/Soyaib10/farm-fusion/internal/infra/smtp"
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

	consumer, err := rabbitmq.NewConsumer(cfg.RabbitMQ.URL, cfg.RabbitMQ.Queue, logger)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer consumer.Close()

	logRepo := postgres.NewNotificationLogRepository(db)
	emailSender := smtp.NewSender(cfg.SMTP)
	processor := notification.NewProcessorUseCase(emailSender, logRepo)

	logger.PrintInfo("Weather notification worker started", map[string]string{"queue": cfg.RabbitMQ.Queue})
	if err := consumer.Consume(ctx, func(payload *domain.NotificationPayload) error {
		return processor.Process(ctx, payload)
	}); err != nil {
		logger.PrintFatal(err, map[string]string{"operation": "consume_weather_notifications"})
	}
}

package notification

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type QueuePublisher interface {
	Publish(ctx context.Context, payload *domain.NotificationPayload) error
	Close()
}

type QueueConsumer interface {
	Consume(ctx context.Context, handler func(payload *domain.NotificationPayload) error) error
	Close()
}

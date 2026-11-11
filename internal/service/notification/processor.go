package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/google/uuid"
)

// EmailSender delivers weather notification emails.
type EmailSender interface {
	SendWeatherNotification(ctx context.Context, payload *domain.NotificationPayload) (string, error)
}

// LogWriter persists notification delivery logs.
type LogWriter interface {
	Create(ctx context.Context, entry *domain.NotificationLog) error
}

// Processor consumes queued weather notifications, sends emails, and logs outcomes.
type Processor struct {
	sender EmailSender
	logs   LogWriter
	log    *logger.Logger
	now    func() time.Time
}

// NewProcessor constructs a notification Processor.
func NewProcessor(sender EmailSender, logs LogWriter, log *logger.Logger) *Processor {
	return &Processor{
		sender: sender,
		logs:   logs,
		log:    log,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// Process delivers the email for payload and records the attempt in notification_log.
func (p *Processor) Process(ctx context.Context, payload *domain.NotificationPayload) error {
	content, sendErr := p.sender.SendWeatherNotification(ctx, payload)

	entry := &domain.NotificationLog{
		ID:               uuid.New(),
		FarmID:           payload.FarmID,
		UserID:           payload.UserID,
		NotificationType: "scheduled",
		AlertCount:       len(payload.Alerts),
		EmailSent:        sendErr == nil,
		EmailContent:     content,
		SentAt:           p.now(),
	}
	if sendErr != nil {
		entry.ErrorMessage = sendErr.Error()
	}

	if logErr := p.logs.Create(ctx, entry); logErr != nil {
		if sendErr != nil {
			return fmt.Errorf("send email: %v; log notification: %w", sendErr, logErr)
		}
		if p.log != nil {
			p.log.Error(logErr, map[string]string{
				"operation": "create_notification_log",
				"farm_id":   payload.FarmID.String(),
			})
		}
	}

	return sendErr
}

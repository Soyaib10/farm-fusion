package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/google/uuid"
)

const markSentAttempts = 3

// EmailSender delivers weather notification emails.
type EmailSender interface {
	SendWeatherNotification(ctx context.Context, payload *domain.NotificationPayload) (string, error)
}

// DeliveryStore persists notification delivery records keyed by notification ID.
type DeliveryStore interface {
	// Begin records an attempt and reports whether an earlier attempt already delivered the email.
	Begin(ctx context.Context, entry *domain.NotificationLog) (alreadySent bool, err error)
	MarkSent(ctx context.Context, notificationID uuid.UUID, content string) error
	MarkFailed(ctx context.Context, notificationID uuid.UUID, content, errMsg string) error
}

// RecipientReader resolves the current state of the notified user.
type RecipientReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// Processor consumes queued weather notifications, sends emails, and logs outcomes.
// Processing the same notification ID twice sends at most one email.
type Processor struct {
	sender     EmailSender
	deliveries DeliveryStore
	users      RecipientReader
	log        *logger.Logger
	now        func() time.Time
	retryDelay time.Duration
}

// NewProcessor constructs a notification Processor.
func NewProcessor(sender EmailSender, deliveries DeliveryStore, users RecipientReader, log *logger.Logger) *Processor {
	return &Processor{
		sender:     sender,
		deliveries: deliveries,
		users:      users,
		log:        log,
		now:        func() time.Time { return time.Now().UTC() },
		retryDelay: 200 * time.Millisecond,
	}
}

// Process delivers the email for payload unless it was already delivered. A returned error means
// the message should be retried; nil means it is done (delivered, duplicate, or no longer wanted).
func (p *Processor) Process(ctx context.Context, payload *domain.NotificationPayload) error {
	if payload.NotificationID == uuid.Nil {
		p.logError(errors.New("notification payload has no notification_id"), payload, "validate_notification")
		return nil
	}

	owner, err := p.users.GetByID(ctx, payload.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			p.logInfo("Skipping notification for deleted user", payload)
			return nil
		}
		return fmt.Errorf("load recipient: %w", err)
	}
	payload.UserEmail = owner.Email

	alreadySent, err := p.deliveries.Begin(ctx, &domain.NotificationLog{
		NotificationID:   payload.NotificationID,
		FarmID:           payload.FarmID,
		UserID:           payload.UserID,
		NotificationType: "scheduled",
		AlertCount:       len(payload.Alerts),
		SentAt:           p.now(),
	})
	if err != nil {
		return fmt.Errorf("record delivery attempt: %w", err)
	}
	if alreadySent {
		p.logInfo("Skipping already delivered notification", payload)
		return nil
	}

	content, sendErr := p.sender.SendWeatherNotification(ctx, payload)
	if sendErr != nil {
		if err := p.deliveries.MarkFailed(ctx, payload.NotificationID, content, sendErr.Error()); err != nil {
			p.logError(err, payload, "mark_notification_failed")
		}
		return fmt.Errorf("send email: %w", sendErr)
	}

	// The SMTP server accepted the message. From here on the delivery is complete: a failure to
	// record it is retried here only, never by resending the email.
	for attempt := 1; ; attempt++ {
		markErr := p.deliveries.MarkSent(ctx, payload.NotificationID, content)
		if markErr == nil {
			return nil
		}
		if attempt == markSentAttempts || ctx.Err() != nil {
			p.logError(markErr, payload, "mark_notification_sent")
			return nil
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Duration(attempt) * p.retryDelay):
		}
	}
}

func (p *Processor) logInfo(msg string, payload *domain.NotificationPayload) {
	if p.log == nil {
		return
	}
	p.log.Info(msg, map[string]string{
		"notification_id": payload.NotificationID.String(),
		"farm_id":         payload.FarmID.String(),
	})
}

func (p *Processor) logError(err error, payload *domain.NotificationPayload, op string) {
	if p.log == nil {
		return
	}
	p.log.Error(err, map[string]string{
		"operation":       op,
		"notification_id": payload.NotificationID.String(),
		"farm_id":         payload.FarmID.String(),
	})
}

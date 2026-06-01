package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/notification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationLogRepository struct {
	db *pgxpool.Pool
}

func NewNotificationLogRepository(db *pgxpool.Pool) notification.LogRepository {
	return &NotificationLogRepository{db: db}
}

func (r *NotificationLogRepository) Create(ctx context.Context, log *domain.NotificationLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.SentAt.IsZero() {
		log.SentAt = time.Now().UTC()
	}

	query := `
		INSERT INTO notification_log (
			id, farm_id, user_id, notification_type, alert_count, email_sent,
			email_content, error_message, sent_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID,
		log.FarmID,
		log.UserID,
		log.NotificationType,
		log.AlertCount,
		log.EmailSent,
		nullableString(log.EmailContent),
		nullableString(log.ErrorMessage),
		log.SentAt,
	)
	if err != nil {
		return fmt.Errorf("create notification log: %w", err)
	}
	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

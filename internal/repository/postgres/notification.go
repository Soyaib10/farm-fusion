package postgres

import (
	"context"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationLogRepository persists email notification delivery logs in PostgreSQL.
type NotificationLogRepository struct {
	db *pgxpool.Pool
}

// NewNotificationLogRepository constructs a NotificationLogRepository.
func NewNotificationLogRepository(db *pgxpool.Pool) *NotificationLogRepository {
	return &NotificationLogRepository{db: db}
}

// Create records a notification delivery attempt.
func (r *NotificationLogRepository) Create(ctx context.Context, entry *domain.NotificationLog) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.SentAt.IsZero() {
		entry.SentAt = time.Now().UTC()
	}

	const query = `
		INSERT INTO notification_log (
			id, farm_id, user_id, notification_type, alert_count, email_sent,
			email_content, error_message, sent_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		entry.ID,
		entry.FarmID,
		entry.UserID,
		entry.NotificationType,
		entry.AlertCount,
		entry.EmailSent,
		nullableString(entry.EmailContent),
		nullableString(entry.ErrorMessage),
		entry.SentAt,
	)
	return mapError("create notification log", err)
}

func nullableString(val string) any {
	if val == "" {
		return nil
	}
	return val
}

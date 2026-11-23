package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationLogRepository persists email notification delivery records in PostgreSQL.
// Rows are keyed by notification_id so a redelivered message is recognized.
type NotificationLogRepository struct {
	db *pgxpool.Pool
}

// NewNotificationLogRepository constructs a NotificationLogRepository.
func NewNotificationLogRepository(db *pgxpool.Pool) *NotificationLogRepository {
	return &NotificationLogRepository{db: db}
}

// Begin records a delivery attempt for entry.NotificationID before the email is sent. When a record
// already exists it is kept, and alreadySent reports whether that earlier attempt delivered the email.
func (r *NotificationLogRepository) Begin(ctx context.Context, entry *domain.NotificationLog) (alreadySent bool, err error) {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.SentAt.IsZero() {
		entry.SentAt = time.Now().UTC()
	}

	const query = `
		INSERT INTO notification_log (
			id, notification_id, farm_id, user_id, notification_type, alert_count,
			email_sent, sent_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, FALSE, $7, $7)
		ON CONFLICT (notification_id) DO UPDATE SET updated_at = EXCLUDED.updated_at
		RETURNING email_sent
	`
	err = r.db.QueryRow(ctx, query,
		entry.ID,
		entry.NotificationID,
		entry.FarmID,
		entry.UserID,
		entry.NotificationType,
		entry.AlertCount,
		entry.SentAt,
	).Scan(&alreadySent)
	if err != nil {
		return false, mapError("begin notification delivery", err)
	}
	return alreadySent, nil
}

// MarkSent records that the SMTP server accepted the email.
func (r *NotificationLogRepository) MarkSent(ctx context.Context, notificationID uuid.UUID, content string) error {
	const query = `
		UPDATE notification_log
		SET email_sent = TRUE, email_content = $2, error_message = NULL, sent_at = NOW(), updated_at = NOW()
		WHERE notification_id = $1
	`
	_, err := r.db.Exec(ctx, query, notificationID, nullableString(content))
	return mapError("mark notification sent", err)
}

// MarkFailed records a failed delivery attempt.
func (r *NotificationLogRepository) MarkFailed(ctx context.Context, notificationID uuid.UUID, content, errMsg string) error {
	const query = `
		UPDATE notification_log
		SET email_content = $2, error_message = $3, updated_at = NOW()
		WHERE notification_id = $1 AND email_sent = FALSE
	`
	_, err := r.db.Exec(ctx, query, notificationID, nullableString(content), nullableString(errMsg))
	return mapError("mark notification failed", err)
}

// NotificationOutboxRepository reserves one scheduled notification per farm per day.
type NotificationOutboxRepository struct {
	db *pgxpool.Pool
}

// NewNotificationOutboxRepository constructs a NotificationOutboxRepository.
func NewNotificationOutboxRepository(db *pgxpool.Pool) *NotificationOutboxRepository {
	return &NotificationOutboxRepository{db: db}
}

// Reserve returns the notification ID for (farmID, alertDate), creating it on first use. A reservation
// that was never confirmed as published keeps its ID, so a retried publish is deduplicated downstream.
func (r *NotificationOutboxRepository) Reserve(ctx context.Context, farmID uuid.UUID, alertDate time.Time) (uuid.UUID, bool, error) {
	const query = `
		INSERT INTO notification_outbox (notification_id, farm_id, alert_date)
		VALUES ($1, $2, $3)
		ON CONFLICT (farm_id, alert_date) DO UPDATE SET farm_id = EXCLUDED.farm_id
		RETURNING notification_id, published_at IS NOT NULL
	`
	var (
		id        uuid.UUID
		published bool
	)
	day := time.Date(alertDate.Year(), alertDate.Month(), alertDate.Day(), 0, 0, 0, 0, time.UTC)
	if err := r.db.QueryRow(ctx, query, uuid.New(), farmID, day).Scan(&id, &published); err != nil {
		return uuid.Nil, false, mapError("reserve notification", err)
	}
	return id, published, nil
}

// MarkPublished records that the broker confirmed the notification.
func (r *NotificationOutboxRepository) MarkPublished(ctx context.Context, notificationID uuid.UUID) error {
	const query = `UPDATE notification_outbox SET published_at = NOW() WHERE notification_id = $1 AND published_at IS NULL`
	_, err := r.db.Exec(ctx, query, notificationID)
	return mapError("mark notification published", err)
}

// AdvisoryLock is a PostgreSQL session-level advisory lock shared by every process using the database.
type AdvisoryLock struct {
	db  *pgxpool.Pool
	key int64
}

// SchedulerLockKey identifies the weather scheduler's advisory lock.
const SchedulerLockKey int64 = 0x66_61_72_6d_2d_73_63_68 // "farm-sch"

// NewAdvisoryLock constructs an AdvisoryLock for key.
func NewAdvisoryLock(db *pgxpool.Pool, key int64) *AdvisoryLock {
	return &AdvisoryLock{db: db, key: key}
}

// TryAcquire takes the lock without waiting. When acquired is true the caller must call release.
// The lock lives on a dedicated pooled connection and is dropped by PostgreSQL if the process dies.
func (l *AdvisoryLock) TryAcquire(ctx context.Context) (release func(), acquired bool, err error) {
	conn, err := l.db.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire connection for advisory lock: %w", err)
	}
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, l.key).Scan(&acquired); err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	if !acquired {
		conn.Release()
		return nil, false, nil
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, l.key); err != nil {
			// Destroy the connection so the session, and with it the lock, ends.
			_ = conn.Conn().Close(ctx)
		}
		conn.Release()
	}, true, nil
}

func nullableString(val string) any {
	if val == "" {
		return nil
	}
	return val
}

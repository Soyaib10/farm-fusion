package postgres

import (
	"context"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshTokenRepository manages hashed refresh tokens in PostgreSQL.
type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

// NewRefreshTokenRepository constructs a RefreshTokenRepository.
func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Save stores a newly issued refresh token record.
func (r *RefreshTokenRepository) Save(ctx context.Context, token *domain.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.Revoked, token.CreatedAt,
	)
	return mapError("save refresh token", err)
}

// Rotate consumes the refresh token identified by oldHash and inserts next for the same user in one
// transaction. The old row is locked with FOR UPDATE, so concurrent rotations of the same token are
// serialized: the loser sees a revoked token and revokes every session of the owner, including the
// row the winner just inserted. next.UserID is set from the consumed row.
func (r *RefreshTokenRepository) Rotate(ctx context.Context, oldHash string, next *domain.RefreshToken) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return mapError("begin refresh rotation", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		userID    uuid.UUID
		revoked   bool
		expiresAt time.Time
	)
	const lockQuery = `
		SELECT user_id, revoked, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`
	if err := tx.QueryRow(ctx, lockQuery, oldHash).Scan(&userID, &revoked, &expiresAt); err != nil {
		return mapError("lock refresh token", err)
	}

	if revoked {
		if _, err := tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`, userID,
		); err != nil {
			return mapError("revoke user refresh tokens", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return mapError("commit refresh revocation", err)
		}
		return domain.ErrTokenRevoked
	}
	if !time.Now().Before(expiresAt) {
		return domain.ErrTokenExpired
	}

	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1`, oldHash); err != nil {
		return mapError("consume refresh token", err)
	}

	next.UserID = userID
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, next.ID, next.UserID, next.TokenHash, next.ExpiresAt, next.Revoked, next.CreatedAt); err != nil {
		return mapError("save rotated refresh token", err)
	}

	return mapError("commit refresh rotation", tx.Commit(ctx))
}

// RevokeByHash marks a single refresh token as revoked.
func (r *RefreshTokenRepository) RevokeByHash(ctx context.Context, tokenHash string) error {
	const query = `UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1`
	_, err := r.db.Exec(ctx, query, tokenHash)
	return mapError("revoke refresh token", err)
}

// RevokeAllForUser revokes all active refresh tokens belonging to userID (used on replay detection).
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const query = `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`
	_, err := r.db.Exec(ctx, query, userID)
	return mapError("revoke user refresh tokens", err)
}

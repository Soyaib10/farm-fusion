package postgres

import (
	"context"

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

// GetByHash retrieves a refresh token record regardless of revocation or expiry state.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, revoked, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t domain.RefreshToken
	if err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Revoked, &t.CreatedAt,
	); err != nil {
		return nil, mapError("get refresh token", err)
	}
	return &t, nil
}

// ConsumeActive atomically marks an unrevoked, unexpired refresh token as revoked and returns it.
// If the token does not exist, is already revoked, or is expired, domain.ErrNotFound is returned.
func (r *RefreshTokenRepository) ConsumeActive(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	const query = `
		UPDATE refresh_tokens
		SET revoked = TRUE
		WHERE token_hash = $1 AND revoked = FALSE AND expires_at > NOW()
		RETURNING id, user_id, token_hash, expires_at, revoked, created_at
	`
	var t domain.RefreshToken
	if err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Revoked, &t.CreatedAt,
	); err != nil {
		return nil, mapError("consume refresh token", err)
	}
	return &t, nil
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

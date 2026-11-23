package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionRepository stores the server-side state access tokens are validated against.
type SessionRepository struct {
	db *pgxpool.Pool
}

// NewSessionRepository constructs a SessionRepository.
func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

// GetSessionState loads the user's role and token version and whether tokenID was revoked,
// in a single round trip. It returns domain.ErrNotFound when the user row is gone.
func (r *SessionRepository) GetSessionState(ctx context.Context, userID uuid.UUID, tokenID string) (domain.SessionState, error) {
	const query = `
		SELECT u.role, u.token_version,
		       EXISTS (SELECT 1 FROM revoked_access_tokens r WHERE r.jti = $2)
		FROM users u
		WHERE u.id = $1
	`
	var (
		state domain.SessionState
		role  string
	)
	if err := r.db.QueryRow(ctx, query, userID, tokenID).Scan(&role, &state.TokenVersion, &state.TokenRevoked); err != nil {
		return domain.SessionState{}, mapError("get session state", err)
	}
	state.Role = domain.Role(role)
	return state, nil
}

// RevokeAccessToken denies tokenID until it expires and prunes entries that have already expired.
func (r *SessionRepository) RevokeAccessToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM revoked_access_tokens WHERE expires_at < NOW()`); err != nil {
		return mapError("prune revoked access tokens", err)
	}
	const query = `
		INSERT INTO revoked_access_tokens (jti, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (jti) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, tokenID, expiresAt)
	return mapError("revoke access token", err)
}

// BumpTokenVersion invalidates every access token issued to userID before this call.
func (r *SessionRepository) BumpTokenVersion(ctx context.Context, userID uuid.UUID) error {
	res, err := r.db.Exec(ctx, `UPDATE users SET token_version = token_version + 1 WHERE id = $1`, userID)
	if err != nil {
		return mapError("bump token version", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("bump token version: %w", domain.ErrNotFound)
	}
	return nil
}

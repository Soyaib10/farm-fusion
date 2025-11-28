package postgres

import (
	"context"
	"fmt"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthRepository struct {
	db *pgxpool.Pool
}

func NewAuthRepository(db *pgxpool.Pool) auth.Repository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, created_at)
              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.Revoked, token.CreatedAt)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, expires_at, revoked, created_at 
              FROM refresh_tokens WHERE token_hash = $1`
	row := r.db.QueryRow(ctx, query, tokenHash)

	var t domain.RefreshToken
	if err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Revoked, &t.CreatedAt); err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &t, nil
}

func (r *AuthRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`
	_, err := r.db.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

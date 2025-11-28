package auth

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type Repository interface {
	SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

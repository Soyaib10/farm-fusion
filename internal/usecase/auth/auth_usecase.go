package auth

import (
	"context"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type UseCase interface {
	Register(ctx context.Context, cmd RegisterCommand) (*AuthResponse, error)
	Login(ctx context.Context, cmd LoginCommand) (*AuthResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error)
	Logout(ctx context.Context, refreshToken string) error
}

type RegisterCommand struct {
	Name     string
	Email    string
	Password string
}

type LoginCommand struct {
	Email    string
	Password string
}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	User         *domain.User
}

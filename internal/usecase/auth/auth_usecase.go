package auth

import (
	"context"
)

type UseCase interface {
	Register(ctx context.Context, cmd RegisterCommand) (*AuthResponse, error)
	Login(ctx context.Context, cmd LoginCommand) (*AuthResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error)
	Logout(ctx context.Context, refreshToken string) error
}

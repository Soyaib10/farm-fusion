package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type useCase struct {
	userRepo user.Repository
	repo     Repository
	cfg      *config.Config
}

func New(userRepo user.Repository, repo Repository, cfg *config.Config) UseCase {
	return &useCase{
		userRepo: userRepo,
		repo:     repo,
		cfg:      cfg,
	}
}

func (uc *useCase) Register(ctx context.Context, cmd RegisterCommand) (*AuthResponse, error) {
	existing, _ := uc.userRepo.GetByEmail(ctx, cmd.Email)
	if existing != nil {
		return nil, errors.New("user already exists")
	}

	u := &domain.User{
		ID:        uuid.New(),
		Name:      cmd.Name,
		Email:     cmd.Email,
		CreatedAt: time.Now(),
	}
	if err := u.SetPassword(cmd.Password); err != nil {
		return nil, fmt.Errorf("set password: %w", err)
	}

	if err := uc.userRepo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return uc.generateTokens(ctx, u)
}

func (uc *useCase) Login(ctx context.Context, cmd LoginCommand) (*AuthResponse, error) {
	u, err := uc.userRepo.GetByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !u.CheckPassword(cmd.Password) {
		return nil, errors.New("invalid credentials")
	}

	return uc.generateTokens(ctx, u)
}

func (uc *useCase) Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	tokenHash := hashToken(refreshToken)

	storedToken, err := uc.repo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	if storedToken.Revoked {
		return nil, errors.New("token revoked")
	}
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, errors.New("token expired")
	}

	u, err := uc.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if err := uc.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, fmt.Errorf("revoke token: %w", err)
	}

	return uc.generateTokens(ctx, u)
}

func (uc *useCase) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := hashToken(refreshToken)
	return uc.repo.RevokeRefreshToken(ctx, tokenHash)
}

func (uc *useCase) generateTokens(ctx context.Context, u *domain.User) (*AuthResponse, error) {
	accessToken, err := uc.createAccessToken(u)
	if err != nil {
		return nil, err
	}

	refreshToken := uuid.New().String()
	tokenHash := hashToken(refreshToken)

	rt := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(uc.cfg.RefreshTokenDuration),
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	if err := uc.repo.SaveRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         u,
	}, nil
}

func (uc *useCase) createAccessToken(u *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"sub": u.ID.String(),
		"exp": time.Now().Add(uc.cfg.AccessTokenDuration).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.cfg.JWTSecret))
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

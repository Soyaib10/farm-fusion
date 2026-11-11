package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/validator"
	"github.com/google/uuid"
)

const refreshTokenBytes = 32

// UserStore defines user persistence operations required by the auth service.
type UserStore interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// RefreshTokenStore defines refresh token persistence operations required by the auth service.
type RefreshTokenStore interface {
	Save(ctx context.Context, token *domain.RefreshToken) error
	GetByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	ConsumeActive(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	RevokeByHash(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

// RegisterInput holds user registration parameters.
type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

// LoginInput holds user login credentials.
type LoginInput struct {
	Email    string
	Password string
}

// AuthResult contains authenticated user details and optional token pair.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	User         *domain.User
}

// Service implements registration, login, token refresh rotation, and logout.
type Service struct {
	users      UserStore
	tokens     RefreshTokenStore
	jwtManager *TokenManager
	refreshTTL time.Duration
}

// NewService constructs an authentication Service.
func NewService(users UserStore, tokens RefreshTokenStore, jwtManager *TokenManager, cfg config.AuthConfig) *Service {
	return &Service{
		users:      users,
		tokens:     tokens,
		jwtManager: jwtManager,
		refreshTTL: cfg.RefreshTokenDuration,
	}
}

// Register validates registration input, hashes the password, and creates a standard user account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	name := strings.TrimSpace(input.Name)
	email := normalizeEmail(input.Email)

	v := validator.New()
	v.Check(validator.NotBlank(name), "name", "must be provided")
	v.Check(validator.RuneCountBetween(name, 2, 100), "name", "must be between 2 and 100 characters")
	v.Check(validator.NoControlChars(name), "name", "must not contain control characters")

	v.Check(validator.NotBlank(email), "email", "must be provided")
	v.Check(len(email) <= 254, "email", "must not exceed 254 characters")
	v.Check(validator.NoControlChars(email), "email", "must not contain control characters")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")

	pwOK, pwMsg := validator.StrongPassword(input.Password)
	v.Check(pwOK, "password", pwMsg)

	if err := v.Err(); err != nil {
		return nil, err
	}

	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("lookup existing user: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrAlreadyExists
	}

	u := &domain.User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		Role:      domain.RoleUser,
		CreatedAt: time.Now().UTC(),
	}
	if err := u.SetPassword(input.Password); err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	return &AuthResult{User: u}, nil
}

// Login authenticates credentials in constant time and issues an access/refresh token pair.
func (s *Service) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	email := normalizeEmail(input.Email)

	v := validator.New()
	v.Check(validator.NotBlank(email), "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
	v.Check(input.Password != "", "password", "must be provided")
	v.Check(len(input.Password) <= 72, "password", "must not exceed 72 bytes")
	if err := v.Err(); err != nil {
		return nil, err
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Equalize response timing to mitigate email enumeration.
			domain.SimulatePasswordCheck(input.Password)
			return nil, domain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user for login: %w", err)
	}

	if !u.CheckPassword(input.Password) {
		return nil, domain.ErrInvalidCredentials
	}

	return s.issueTokenPair(ctx, u)
}

// Refresh atomically rotates an active refresh token and detects token replay attacks.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*AuthResult, error) {
	trimmed := strings.TrimSpace(rawRefreshToken)
	if trimmed == "" || len(trimmed) > 512 {
		return nil, domain.ErrUnauthorized
	}

	tokenHash := hashToken(trimmed)

	consumed, err := s.tokens.ConsumeActive(ctx, tokenHash)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("consume refresh token: %w", err)
		}

		// Inspect whether the token was already revoked (token theft / replay) or expired.
		existing, lookupErr := s.tokens.GetByHash(ctx, tokenHash)
		if lookupErr != nil {
			return nil, domain.ErrUnauthorized
		}
		if existing.Revoked {
			// Compromised token reuse detected: revoke all active sessions for this user.
			_ = s.tokens.RevokeAllForUser(ctx, existing.UserID)
			return nil, domain.ErrTokenRevoked
		}
		if time.Now().UTC().After(existing.ExpiresAt) {
			return nil, domain.ErrTokenExpired
		}
		return nil, domain.ErrUnauthorized
	}

	u, err := s.users.GetByID(ctx, consumed.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, fmt.Errorf("get user for refresh: %w", err)
	}

	return s.issueTokenPair(ctx, u)
}

// Logout revokes the supplied refresh token.
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	trimmed := strings.TrimSpace(rawRefreshToken)
	v := validator.New()
	v.Check(trimmed != "" && len(trimmed) <= 512, "refresh_token", "must be a valid refresh token")
	if err := v.Err(); err != nil {
		return err
	}

	return s.tokens.RevokeByHash(ctx, hashToken(trimmed))
}

func (s *Service) issueTokenPair(ctx context.Context, u *domain.User) (*AuthResult, error) {
	accessToken, err := s.jwtManager.IssueAccessToken(u)
	if err != nil {
		return nil, err
	}

	rawRefresh, err := generateSecureToken(refreshTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now().UTC()
	rt := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		TokenHash: hashToken(rawRefresh),
		ExpiresAt: now.Add(s.refreshTTL),
		Revoked:   false,
		CreatedAt: now,
	}

	if err := s.tokens.Save(ctx, rt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		User:         u,
	}, nil
}

func generateSecureToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

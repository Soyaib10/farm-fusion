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

const (
	refreshTokenBytes = 32

	// maxFailedLogins consecutive failures lock the account for loginLockout.
	maxFailedLogins = 5
	loginLockout    = 15 * time.Minute
)

// UserStore defines user persistence operations required by the auth service.
type UserStore interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	RecordLoginFailure(ctx context.Context, id uuid.UUID, maxFailures int, lockout time.Duration) error
	ResetLoginFailures(ctx context.Context, id uuid.UUID) error
}

// RefreshTokenStore defines refresh token persistence operations required by the auth service.
type RefreshTokenStore interface {
	Save(ctx context.Context, token *domain.RefreshToken) error
	// Rotate atomically consumes the token identified by oldHash and stores next for the same user.
	// It returns domain.ErrNotFound for unknown tokens, domain.ErrTokenExpired for expired ones, and
	// domain.ErrTokenRevoked after revoking every session of the owner when a consumed token is replayed.
	Rotate(ctx context.Context, oldHash string, next *domain.RefreshToken) error
	RevokeByHash(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

// SessionStore holds server-side access-token state.
type SessionStore interface {
	// GetSessionState returns domain.ErrNotFound when the user no longer exists.
	GetSessionState(ctx context.Context, userID uuid.UUID, tokenID string) (domain.SessionState, error)
	RevokeAccessToken(ctx context.Context, tokenID string, expiresAt time.Time) error
	BumpTokenVersion(ctx context.Context, userID uuid.UUID) error
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

// AuthResult contains authenticated user details and the issued token pair.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	User         *domain.User
}

// Service implements registration, login, token refresh rotation, and logout.
type Service struct {
	users      UserStore
	tokens     RefreshTokenStore
	sessions   SessionStore
	jwtManager *TokenManager
	refreshTTL time.Duration
	now        func() time.Time
}

// NewService constructs an authentication Service.
func NewService(users UserStore, tokens RefreshTokenStore, sessions SessionStore, jwtManager *TokenManager, cfg config.AuthConfig) *Service {
	return &Service{
		users:      users,
		tokens:     tokens,
		sessions:   sessions,
		jwtManager: jwtManager,
		refreshTTL: cfg.RefreshTokenDuration,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// Register validates registration input and creates a standard user account. The outcome is the same
// whether or not the email is already registered, and both paths spend the same bcrypt work, so the
// endpoint cannot be used to discover accounts.
func (s *Service) Register(ctx context.Context, input RegisterInput) error {
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
		return err
	}

	u := &domain.User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		Role:      domain.RoleUser,
		CreatedAt: s.now(),
	}
	if err := u.SetPassword(input.Password); err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return nil
		}
		return err
	}
	return nil
}

// Login authenticates credentials in constant time and issues an access/refresh token pair.
// Accounts are locked for a short period after repeated failures; a locked account answers
// exactly like a wrong password.
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

	if u.IsLocked(s.now()) {
		domain.SimulatePasswordCheck(input.Password)
		return nil, domain.ErrInvalidCredentials
	}

	if !u.CheckPassword(input.Password) {
		if err := s.users.RecordLoginFailure(ctx, u.ID, maxFailedLogins, loginLockout); err != nil {
			return nil, fmt.Errorf("record login failure: %w", err)
		}
		return nil, domain.ErrInvalidCredentials
	}

	if u.FailedLoginCount > 0 {
		if err := s.users.ResetLoginFailures(ctx, u.ID); err != nil {
			return nil, fmt.Errorf("reset login failures: %w", err)
		}
	}

	return s.issueTokenPair(ctx, u)
}

// Refresh atomically rotates an active refresh token and detects token replay attacks.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*AuthResult, error) {
	trimmed := strings.TrimSpace(rawRefreshToken)
	if trimmed == "" || len(trimmed) > 512 {
		return nil, domain.ErrUnauthorized
	}

	rawNext, next, err := s.newRefreshToken()
	if err != nil {
		return nil, err
	}

	if err := s.tokens.Rotate(ctx, hashToken(trimmed), next); err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			return nil, domain.ErrUnauthorized
		case errors.Is(err, domain.ErrTokenRevoked), errors.Is(err, domain.ErrTokenExpired):
			return nil, err
		default:
			return nil, fmt.Errorf("rotate refresh token: %w", err)
		}
	}

	u, err := s.users.GetByID(ctx, next.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, fmt.Errorf("get user for refresh: %w", err)
	}

	accessToken, err := s.jwtManager.IssueAccessToken(u)
	if err != nil {
		return nil, err
	}
	return &AuthResult{AccessToken: accessToken, RefreshToken: rawNext, User: u}, nil
}

// Authenticate verifies an access token and checks it against current account state: the user must
// still exist, the token version must match, and the token must not have been individually revoked.
// The returned role comes from the database, not from the token.
func (s *Service) Authenticate(ctx context.Context, rawAccessToken string) (domain.Actor, error) {
	tok, err := s.jwtManager.VerifyAccessToken(rawAccessToken)
	if err != nil {
		return domain.Actor{}, domain.ErrUnauthorized
	}

	state, err := s.sessions.GetSessionState(ctx, tok.UserID, tok.TokenID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Actor{}, domain.ErrUnauthorized
		}
		return domain.Actor{}, fmt.Errorf("load session state: %w", err)
	}
	if state.TokenRevoked || state.TokenVersion != tok.Version || !state.Role.IsValid() {
		return domain.Actor{}, domain.ErrUnauthorized
	}

	return domain.Actor{ID: tok.UserID, Role: state.Role, TokenID: tok.TokenID}, nil
}

// Logout revokes the supplied refresh token and, when given, the access token used for this device.
func (s *Service) Logout(ctx context.Context, rawRefreshToken, rawAccessToken string) error {
	trimmed := strings.TrimSpace(rawRefreshToken)
	v := validator.New()
	v.Check(trimmed != "" && len(trimmed) <= 512, "refresh_token", "must be a valid refresh token")
	if err := v.Err(); err != nil {
		return err
	}

	if err := s.tokens.RevokeByHash(ctx, hashToken(trimmed)); err != nil {
		return err
	}

	if rawAccessToken == "" {
		return nil
	}
	tok, err := s.jwtManager.VerifyAccessToken(rawAccessToken)
	if err != nil {
		return nil
	}
	return s.sessions.RevokeAccessToken(ctx, tok.TokenID, tok.ExpiresAt)
}

// LogoutAll invalidates every access and refresh token issued to the actor.
func (s *Service) LogoutAll(ctx context.Context, actor domain.Actor) error {
	if err := s.sessions.BumpTokenVersion(ctx, actor.ID); err != nil {
		return fmt.Errorf("bump token version: %w", err)
	}
	if err := s.tokens.RevokeAllForUser(ctx, actor.ID); err != nil {
		return fmt.Errorf("revoke refresh tokens: %w", err)
	}
	return nil
}

func (s *Service) issueTokenPair(ctx context.Context, u *domain.User) (*AuthResult, error) {
	accessToken, err := s.jwtManager.IssueAccessToken(u)
	if err != nil {
		return nil, err
	}

	rawRefresh, rt, err := s.newRefreshToken()
	if err != nil {
		return nil, err
	}
	rt.UserID = u.ID

	if err := s.tokens.Save(ctx, rt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		User:         u,
	}, nil
}

func (s *Service) newRefreshToken() (string, *domain.RefreshToken, error) {
	raw, err := generateSecureToken(refreshTokenBytes)
	if err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	now := s.now()
	return raw, &domain.RefreshToken{
		ID:        uuid.New(),
		TokenHash: hashToken(raw),
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
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

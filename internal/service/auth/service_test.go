package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type memUserStore struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]*domain.User
	email map[string]*domain.User
}

func newMemUserStore() *memUserStore {
	return &memUserStore{
		byID:  make(map[uuid.UUID]*domain.User),
		email: make(map[string]*domain.User),
	}
}

func (m *memUserStore) Create(_ context.Context, u *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(u.Email)
	if _, exists := m.email[key]; exists {
		return domain.ErrAlreadyExists
	}
	cp := *u
	m.byID[u.ID] = &cp
	m.email[key] = &cp
	return nil
}

func (m *memUserStore) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.email[strings.ToLower(email)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *memUserStore) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

type memTokenStore struct {
	mu     sync.Mutex
	byHash map[string]*domain.RefreshToken
}

func newMemTokenStore() *memTokenStore {
	return &memTokenStore{byHash: make(map[string]*domain.RefreshToken)}
}

func (m *memTokenStore) Save(_ context.Context, t *domain.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *t
	m.byHash[t.TokenHash] = &cp
	return nil
}

func (m *memTokenStore) GetByHash(_ context.Context, h string) (*domain.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byHash[h]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (m *memTokenStore) ConsumeActive(_ context.Context, h string) (*domain.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byHash[h]
	if !ok || t.Revoked || time.Now().UTC().After(t.ExpiresAt) {
		return nil, domain.ErrNotFound
	}
	t.Revoked = true
	cp := *t
	return &cp, nil
}

func (m *memTokenStore) RevokeByHash(_ context.Context, h string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.byHash[h]; ok {
		t.Revoked = true
	}
	return nil
}

func (m *memTokenStore) RevokeAllForUser(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.byHash {
		if t.UserID == userID {
			t.Revoked = true
		}
	}
	return nil
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		JWTSecret:            "0123456789abcdef0123456789abcdef-test-key",
		JWTIssuer:            "farm-fusion-api",
		JWTAudience:          "farm-fusion-client",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 24 * time.Hour,
	}
}

func TestAuthRegisterLoginRotateAndReplayDetection(t *testing.T) {
	ctx := context.Background()
	users := newMemUserStore()
	tokens := newMemTokenStore()
	cfg := testAuthConfig()
	tm := NewTokenManager(cfg)
	svc := NewService(users, tokens, tm, cfg)

	// 1. Weak password rejected
	if _, err := svc.Register(ctx, RegisterInput{
		Name:     "Zihad",
		Email:    "zihad@example.com",
		Password: "weak",
	}); err == nil {
		t.Fatal("expected validation error for weak password")
	}

	// 2. Valid registration normalizes email and assigns RoleUser
	regRes, err := svc.Register(ctx, RegisterInput{
		Name:     "Zihad",
		Email:    "  ZIHAD@Example.COM ",
		Password: "StrongPassword1!",
	})
	if err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
	if regRes.User.Email != "zihad@example.com" {
		t.Fatalf("got email %q, want normalized lowercase", regRes.User.Email)
	}
	if regRes.User.Role != domain.RoleUser {
		t.Fatalf("got role %q, want %q", regRes.User.Role, domain.RoleUser)
	}

	// 3. Duplicate email rejected with domain.ErrAlreadyExists
	if _, err := svc.Register(ctx, RegisterInput{
		Name:     "Other",
		Email:    "zihad@example.com",
		Password: "StrongPassword1!",
	}); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("got err %v, want ErrAlreadyExists", err)
	}

	// 4. Login issues verifiable JWT and refresh token
	loginRes, err := svc.Login(ctx, LoginInput{
		Email:    "zihad@example.com",
		Password: "StrongPassword1!",
	})
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	actor, err := tm.VerifyAccessToken(loginRes.AccessToken)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error: %v", err)
	}
	if actor.ID != regRes.User.ID || actor.Role != domain.RoleUser {
		t.Fatalf("unexpected actor %+v", actor)
	}

	// 5. Refresh rotates refresh token
	rotated, err := svc.Refresh(ctx, loginRes.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() unexpected error: %v", err)
	}
	if rotated.RefreshToken == loginRes.RefreshToken {
		t.Fatal("expected rotated refresh token to differ from original")
	}

	// 6. Replaying old consumed refresh token triggers theft detection & revokes the new token too!
	if _, err := svc.Refresh(ctx, loginRes.RefreshToken); !errors.Is(err, domain.ErrTokenRevoked) {
		t.Fatalf("replaying old token got %v, want ErrTokenRevoked", err)
	}
	if _, err := svc.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, domain.ErrTokenRevoked) {
		t.Fatalf("expected active session to be revoked after token replay, got %v", err)
	}
}

func TestTokenManagerRejectsTamperedTokens(t *testing.T) {
	cfg := testAuthConfig()
	tm := NewTokenManager(cfg)

	// Forged alg=none token must be rejected
	noneTok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub":  uuid.NewString(),
		"iss":  cfg.JWTIssuer,
		"aud":  cfg.JWTAudience,
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"role": "admin",
	})
	rawNone, _ := noneTok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := tm.VerifyAccessToken(rawNone); err == nil {
		t.Fatal("VerifyAccessToken must reject alg=none token")
	}

	// Token with wrong issuer must be rejected
	wrongIssCfg := cfg
	wrongIssCfg.JWTIssuer = "other-service"
	otherTM := NewTokenManager(wrongIssCfg)
	tok, _ := otherTM.IssueAccessToken(&domain.User{ID: uuid.New(), Role: domain.RoleUser})
	if _, err := tm.VerifyAccessToken(tok); err == nil {
		t.Fatal("VerifyAccessToken must reject token from wrong issuer")
	}
}

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

func (m *memUserStore) RecordLoginFailure(_ context.Context, id uuid.UUID, maxFailures int, lockout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID[id]
	u.FailedLoginCount++
	if u.FailedLoginCount >= maxFailures {
		until := time.Now().UTC().Add(lockout)
		u.LockedUntil = &until
	}
	return nil
}

func (m *memUserStore) ResetLoginFailures(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID[id]
	u.FailedLoginCount = 0
	u.LockedUntil = nil
	return nil
}

func (m *memUserStore) delete(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.byID[id]; ok {
		delete(m.email, strings.ToLower(u.Email))
		delete(m.byID, id)
	}
}

type memSessionStore struct {
	users   *memUserStore
	mu      sync.Mutex
	revoked map[string]time.Time
}

func (m *memSessionStore) GetSessionState(_ context.Context, userID uuid.UUID, tokenID string) (domain.SessionState, error) {
	m.users.mu.Lock()
	u, ok := m.users.byID[userID]
	m.users.mu.Unlock()
	if !ok {
		return domain.SessionState{}, domain.ErrNotFound
	}
	m.mu.Lock()
	_, revoked := m.revoked[tokenID]
	m.mu.Unlock()
	return domain.SessionState{Role: u.Role, TokenVersion: u.TokenVersion, TokenRevoked: revoked}, nil
}

func (m *memSessionStore) RevokeAccessToken(_ context.Context, tokenID string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked[tokenID] = expiresAt
	return nil
}

func (m *memSessionStore) BumpTokenVersion(_ context.Context, userID uuid.UUID) error {
	m.users.mu.Lock()
	defer m.users.mu.Unlock()
	u, ok := m.users.byID[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.TokenVersion++
	return nil
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

func (m *memTokenStore) Rotate(_ context.Context, oldHash string, next *domain.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.byHash[oldHash]
	if !ok {
		return domain.ErrNotFound
	}
	if old.Revoked {
		for _, t := range m.byHash {
			if t.UserID == old.UserID {
				t.Revoked = true
			}
		}
		return domain.ErrTokenRevoked
	}
	if !time.Now().Before(old.ExpiresAt) {
		return domain.ErrTokenExpired
	}
	old.Revoked = true
	next.UserID = old.UserID
	cp := *next
	m.byHash[next.TokenHash] = &cp
	return nil
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

type testEnv struct {
	svc    *Service
	tm     *TokenManager
	users  *memUserStore
	tokens *memTokenStore
}

func newTestEnv() testEnv {
	cfg := testAuthConfig()
	users := newMemUserStore()
	tokens := newMemTokenStore()
	sessions := &memSessionStore{users: users, revoked: make(map[string]time.Time)}
	tm := NewTokenManager(cfg)
	return testEnv{svc: NewService(users, tokens, sessions, tm, cfg), tm: tm, users: users, tokens: tokens}
}

func (e testEnv) registerAndLogin(t *testing.T, email string) *AuthResult {
	t.Helper()
	ctx := context.Background()
	if err := e.svc.Register(ctx, RegisterInput{Name: "Zihad", Email: email, Password: "StrongPassword1!"}); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
	res, err := e.svc.Login(ctx, LoginInput{Email: email, Password: "StrongPassword1!"})
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}
	return res
}

func TestAuthRegisterLoginRotateAndReplayDetection(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv()
	svc := env.svc

	// 1. Weak password rejected
	if err := svc.Register(ctx, RegisterInput{
		Name:     "Zihad",
		Email:    "zihad@example.com",
		Password: "weak",
	}); err == nil {
		t.Fatal("expected validation error for weak password")
	}

	// 2. Valid registration normalizes email and assigns RoleUser
	if err := svc.Register(ctx, RegisterInput{
		Name:     "Zihad",
		Email:    "  ZIHAD@Example.COM ",
		Password: "StrongPassword1!",
	}); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
	stored, err := env.users.GetByEmail(ctx, "zihad@example.com")
	if err != nil {
		t.Fatalf("registered user not stored under normalized email: %v", err)
	}
	if stored.Role != domain.RoleUser {
		t.Fatalf("got role %q, want %q", stored.Role, domain.RoleUser)
	}

	// 3. Duplicate email answers exactly like a new one and does not overwrite the account
	if err := svc.Register(ctx, RegisterInput{
		Name:     "Other",
		Email:    "zihad@example.com",
		Password: "OtherPassword1!",
	}); err != nil {
		t.Fatalf("duplicate Register() got %v, want nil (no enumeration)", err)
	}
	if _, err := svc.Login(ctx, LoginInput{Email: "zihad@example.com", Password: "OtherPassword1!"}); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("duplicate registration must not change the password, got %v", err)
	}

	// 4. Login issues verifiable JWT and refresh token
	loginRes, err := svc.Login(ctx, LoginInput{
		Email:    "zihad@example.com",
		Password: "StrongPassword1!",
	})
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	actor, err := svc.Authenticate(ctx, loginRes.AccessToken)
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
	if actor.ID != stored.ID || actor.Role != domain.RoleUser {
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

func TestAccessTokenRejectedAfterAccountDeletion(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv()
	res := env.registerAndLogin(t, "gone@example.com")

	if _, err := env.svc.Authenticate(ctx, res.AccessToken); err != nil {
		t.Fatalf("Authenticate() before delete: %v", err)
	}

	env.users.delete(res.User.ID)

	if _, err := env.svc.Authenticate(ctx, res.AccessToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Authenticate() after delete got %v, want ErrUnauthorized", err)
	}
	if _, err := env.tm.VerifyAccessToken(res.AccessToken); err != nil {
		t.Fatal("test precondition: the JWT itself is still cryptographically valid")
	}
}

func TestAuthenticateUsesRoleFromDatabase(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv()
	res := env.registerAndLogin(t, "demoted@example.com")

	// A token minted with role=admin for this user does not grant admin while the row says user.
	forged := *res.User
	forged.Role = domain.RoleAdmin
	adminToken, err := env.tm.IssueAccessToken(&forged)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := env.svc.Authenticate(ctx, adminToken)
	if err != nil {
		t.Fatalf("Authenticate(): %v", err)
	}
	if actor.IsAdmin() {
		t.Fatal("role must come from the users row, not the token claim")
	}
}

func TestLogoutRevokesAccessTokenAndLogoutAllRevokesEverything(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv()

	first := env.registerAndLogin(t, "farmer@example.com")
	second, err := env.svc.Login(ctx, LoginInput{Email: "farmer@example.com", Password: "StrongPassword1!"})
	if err != nil {
		t.Fatal(err)
	}

	// Single-device logout kills that device's access token only.
	if err := env.svc.Logout(ctx, first.RefreshToken, first.AccessToken); err != nil {
		t.Fatalf("Logout(): %v", err)
	}
	if _, err := env.svc.Authenticate(ctx, first.AccessToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("logged-out access token got %v, want ErrUnauthorized", err)
	}
	actor, err := env.svc.Authenticate(ctx, second.AccessToken)
	if err != nil {
		t.Fatalf("other device must stay logged in, got %v", err)
	}

	// Log out everywhere.
	if err := env.svc.LogoutAll(ctx, actor); err != nil {
		t.Fatalf("LogoutAll(): %v", err)
	}
	if _, err := env.svc.Authenticate(ctx, second.AccessToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("access token after LogoutAll got %v, want ErrUnauthorized", err)
	}
	if _, err := env.svc.Refresh(ctx, second.RefreshToken); err == nil {
		t.Fatal("refresh token must be revoked after LogoutAll")
	}

	// A fresh login works and carries the new token version.
	fresh, err := env.svc.Login(ctx, LoginInput{Email: "farmer@example.com", Password: "StrongPassword1!"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.Authenticate(ctx, fresh.AccessToken); err != nil {
		t.Fatalf("fresh login after LogoutAll: %v", err)
	}
}

func TestLoginLocksAccountAfterRepeatedFailures(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv()
	env.registerAndLogin(t, "target@example.com")

	for i := 0; i < maxFailedLogins; i++ {
		if _, err := env.svc.Login(ctx, LoginInput{Email: "target@example.com", Password: "WrongGuess1!"}); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("attempt %d got %v, want ErrInvalidCredentials", i+1, err)
		}
	}

	// The correct password is refused while locked, with the same error as a wrong one.
	if _, err := env.svc.Login(ctx, LoginInput{Email: "target@example.com", Password: "StrongPassword1!"}); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("locked account login got %v, want ErrInvalidCredentials", err)
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

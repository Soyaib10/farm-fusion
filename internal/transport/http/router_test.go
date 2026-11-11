package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	authsvc "github.com/Soyaib10/farm-fusion/internal/service/auth"
	farmsvc "github.com/Soyaib10/farm-fusion/internal/service/farm"
	usersvc "github.com/Soyaib10/farm-fusion/internal/service/user"
	weathersvc "github.com/Soyaib10/farm-fusion/internal/service/weather"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/handler"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/middleware"
	"github.com/google/uuid"
)

type nopAuthSvc struct{}

func (nopAuthSvc) Register(context.Context, authsvc.RegisterInput) (*authsvc.AuthResult, error) {
	return nil, domain.ErrInvalidCredentials
}
func (nopAuthSvc) Login(context.Context, authsvc.LoginInput) (*authsvc.AuthResult, error) {
	return nil, domain.ErrInvalidCredentials
}
func (nopAuthSvc) Refresh(context.Context, string) (*authsvc.AuthResult, error) {
	return nil, domain.ErrUnauthorized
}
func (nopAuthSvc) Logout(context.Context, string) error { return nil }

type stubUserSvc struct {
	self *domain.User
}

func (s stubUserSvc) GetByID(_ context.Context, actor domain.Actor, targetID uuid.UUID) (*domain.User, error) {
	if !actor.CanAccessUser(targetID) {
		return nil, domain.ErrForbidden
	}
	return s.self, nil
}
func (s stubUserSvc) Update(_ context.Context, actor domain.Actor, targetID uuid.UUID, _ usersvc.UpdateInput) (*domain.User, error) {
	if !actor.CanAccessUser(targetID) {
		return nil, domain.ErrForbidden
	}
	return s.self, nil
}
func (s stubUserSvc) Delete(_ context.Context, actor domain.Actor, targetID uuid.UUID) error {
	if !actor.CanAccessUser(targetID) {
		return domain.ErrForbidden
	}
	return nil
}
func (s stubUserSvc) List(_ context.Context, actor domain.Actor) ([]*domain.User, error) {
	if !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}
	return []*domain.User{s.self}, nil
}

type nopFarmSvc struct{}

func (nopFarmSvc) Create(context.Context, uuid.UUID, farmsvc.UpsertInput) (*domain.Farm, error) {
	return nil, nil
}
func (nopFarmSvc) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.Farm, error) {
	return nil, domain.ErrNotFound
}
func (nopFarmSvc) ListByUserID(context.Context, uuid.UUID) ([]*domain.Farm, error) { return nil, nil }
func (nopFarmSvc) Update(context.Context, uuid.UUID, uuid.UUID, farmsvc.UpsertInput) (*domain.Farm, error) {
	return nil, domain.ErrNotFound
}
func (nopFarmSvc) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type nopWeatherSvc struct{}

func (nopWeatherSvc) Create(context.Context, weathersvc.CreateInput) (*domain.Weather, error) {
	return nil, nil
}
func (nopWeatherSvc) GetByID(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Weather, error) {
	return nil, domain.ErrNotFound
}
func (nopWeatherSvc) ListByFarm(context.Context, uuid.UUID, uuid.UUID) ([]*domain.Weather, error) {
	return nil, nil
}
func (nopWeatherSvc) Update(context.Context, weathersvc.UpdateInput) (*domain.Weather, error) {
	return nil, nil
}
func (nopWeatherSvc) Delete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }

type nopRecSvc struct{}

func (nopRecSvc) RecommendCrop(context.Context, domain.CropQuery) (*domain.CropRecommendation, error) {
	return &domain.CropRecommendation{Crop: "rice", Confidence: 0.99}, nil
}
func (nopRecSvc) RecommendFertilizer(context.Context, domain.FertilizerQuery) (*domain.FertilizerRecommendation, error) {
	return &domain.FertilizerRecommendation{Fertilizer: "Urea", Confidence: 0.95}, nil
}

func TestRouterSecurityHeadersRBACAndRateLimit(t *testing.T) {
	log := logger.New(io.Discard, logger.LevelOff)
	cfg := &config.Config{
		Env: "production",
		Auth: config.AuthConfig{
			JWTSecret:           "0123456789abcdef0123456789abcdef-router-key",
			JWTIssuer:           "farm-fusion-api",
			JWTAudience:         "farm-fusion-client",
			AccessTokenDuration: 15 * time.Minute,
		},
		CORS: config.CORSConfig{AllowedOrigins: []string{"https://app.farmfusion.com"}},
	}

	tm := authsvc.NewTokenManager(cfg.Auth)
	userID := uuid.New()
	selfUser := &domain.User{ID: userID, Name: "Farmer", Email: "farmer@example.com", Role: domain.RoleUser}

	authLimiter := middleware.NewRateLimiter(0.01, 2, false, log)
	defer authLimiter.Close()

	router := NewRouter(Dependencies{
		Config:         cfg,
		Logger:         log,
		TokenVerifier:  tm,
		Health:         handler.NewHealthHandler(nil, log),
		Auth:           handler.NewAuthHandler(nopAuthSvc{}, log),
		User:           handler.NewUserHandler(stubUserSvc{self: selfUser}, log),
		Farm:           handler.NewFarmHandler(nopFarmSvc{}, log),
		Weather:        handler.NewWeatherHandler(nopWeatherSvc{}, log),
		Recommendation: handler.NewRecommendationHandler(nopRecSvc{}, log),
		AuthLimiter:    authLimiter,
	})

	// 1. Liveness endpoint sets full security header suite including HSTS in production
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", rec.Code)
	}
	for _, hdr := range []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Content-Security-Policy",
		"Strict-Transport-Security",
		"X-Request-ID",
	} {
		if rec.Header().Get(hdr) == "" {
			t.Fatalf("expected security header %s to be set", hdr)
		}
	}

	// 2. Protected route rejects unauthenticated calls with 401 + WWW-Authenticate
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("unauthenticated /users/me = %d, want 401 Bearer", rec.Code)
	}

	// 3. Regular user can access /users/me, CANNOT access /users (admin-only) or /users/{otherID}
	userToken, _ := tm.IssueAccessToken(selfUser)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated /users/me = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin GET /users = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+uuid.NewString(), nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner DELETE /users/{other} = %d, want 403", rec.Code)
	}

	// 4. Auth rate limiter blocks brute force even if attacker spoofs X-Forwarded-For (TrustProxy=false)
	for i := 1; i <= 3; i++ {
		body := bytes.NewBufferString(`{"email":"a@b.com","password":"pw"}`)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
		req.RemoteAddr = "203.0.113.10:54321"
		req.Header.Set("X-Forwarded-For", "198.51.100."+string(rune('0'+i)))
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if i == 1 && rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("expected Cache-Control: no-store on /auth, got %q", rec.Header().Get("Cache-Control"))
		}
		if i == 3 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d got status %d, want 429 Too Many Requests", i, rec.Code)
		}
	}
}

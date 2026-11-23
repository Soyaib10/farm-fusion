package main

import (
	"context"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/platform/database"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/platform/mlclient"
	"github.com/Soyaib10/farm-fusion/internal/repository/postgres"
	authsvc "github.com/Soyaib10/farm-fusion/internal/service/auth"
	farmsvc "github.com/Soyaib10/farm-fusion/internal/service/farm"
	recsvc "github.com/Soyaib10/farm-fusion/internal/service/recommendation"
	usersvc "github.com/Soyaib10/farm-fusion/internal/service/user"
	weathersvc "github.com/Soyaib10/farm-fusion/internal/service/weather"
	httpapi "github.com/Soyaib10/farm-fusion/internal/transport/http"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/handler"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/middleware"
)

func main() {
	log := logger.New(os.Stdout, logger.LevelInfo)

	cfg, err := config.LoadForAPI()
	if err != nil {
		log.Fatal(err, map[string]string{"operation": "load_config"})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.Database, log)
	if err != nil {
		log.Fatal(err, map[string]string{"operation": "connect_database"})
	}
	defer db.Close()

	// Repositories
	userRepo := postgres.NewUserRepository(db)
	tokenRepo := postgres.NewRefreshTokenRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	farmRepo := postgres.NewFarmRepository(db)
	weatherRepo := postgres.NewWeatherRepository(db)

	// External clients
	mlHTTPClient := &http.Client{Timeout: cfg.ML.Timeout}
	mlClient := mlclient.NewClient(cfg.ML.BaseURL, cfg.ML.APIKey, mlHTTPClient, log)

	// Services
	tokenManager := authsvc.NewTokenManager(cfg.Auth)
	authService := authsvc.NewService(userRepo, tokenRepo, sessionRepo, tokenManager, cfg.Auth)
	userService := usersvc.NewService(userRepo)
	farmService := farmsvc.NewService(farmRepo)
	weatherService := weathersvc.NewService(weatherRepo, farmService)
	recService := recsvc.NewService(mlClient)

	// Rate limiters
	var trustedProxies []netip.Prefix
	if cfg.Server.TrustProxy {
		trustedProxies = cfg.Server.TrustedProxies
	}
	var globalLimiter, authLimiter *middleware.RateLimiter
	if cfg.RateLimit.Enabled {
		globalLimiter, err = middleware.NewRateLimiter(cfg.RateLimit.RPS, cfg.RateLimit.Burst, trustedProxies, log)
		if err != nil {
			log.Fatal(err, map[string]string{"operation": "create_rate_limiter"})
		}
		defer globalLimiter.Close()

		authLimiter, err = middleware.NewRateLimiter(cfg.RateLimit.AuthRPS, cfg.RateLimit.AuthBurst, trustedProxies, log)
		if err != nil {
			log.Fatal(err, map[string]string{"operation": "create_auth_rate_limiter"})
		}
		defer authLimiter.Close()
	}

	// HTTP Transport
	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:         cfg,
		Logger:         log,
		Authenticator:  authService,
		Health:         handler.NewHealthHandler(db, log),
		Auth:           handler.NewAuthHandler(authService, log),
		User:           handler.NewUserHandler(userService, log),
		Farm:           handler.NewFarmHandler(farmService, log),
		Weather:        handler.NewWeatherHandler(weatherService, log),
		Recommendation: handler.NewRecommendationHandler(recService, log),
		GlobalLimiter:  globalLimiter,
		AuthLimiter:    authLimiter,
	})

	srv := httpapi.NewServer(cfg.Server, router, log)
	if err := srv.Serve(ctx); err != nil {
		log.Fatal(err, map[string]string{"operation": "serve_http"})
	}
}

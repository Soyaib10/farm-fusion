package main

import (
	"context"
	"net/http"
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
	farmRepo := postgres.NewFarmRepository(db)
	weatherRepo := postgres.NewWeatherRepository(db)

	// External clients
	mlHTTPClient := &http.Client{Timeout: cfg.ML.Timeout}
	mlClient := mlclient.NewClient(cfg.ML.BaseURL, cfg.ML.APIKey, mlHTTPClient, log)

	// Services
	tokenManager := authsvc.NewTokenManager(cfg.Auth)
	authService := authsvc.NewService(userRepo, tokenRepo, tokenManager, cfg.Auth)
	userService := usersvc.NewService(userRepo)
	farmService := farmsvc.NewService(farmRepo)
	weatherService := weathersvc.NewService(weatherRepo, farmService)
	recService := recsvc.NewService(mlClient)

	// Rate limiters
	var globalLimiter, authLimiter *middleware.RateLimiter
	if cfg.RateLimit.Enabled {
		globalLimiter = middleware.NewRateLimiter(
			cfg.RateLimit.RPS, cfg.RateLimit.Burst, cfg.Server.TrustProxy, log,
		)
		defer globalLimiter.Close()

		authLimiter = middleware.NewRateLimiter(
			cfg.RateLimit.AuthRPS, cfg.RateLimit.AuthBurst, cfg.Server.TrustProxy, log,
		)
		defer authLimiter.Close()
	}

	// HTTP Transport
	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:         cfg,
		Logger:         log,
		TokenVerifier:  tokenManager,
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

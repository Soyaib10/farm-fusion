package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/config"
	httpDelivery "github.com/Soyaib10/farm-fusion/internal/delivery/http"
	authHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/auth"
	farmHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/farm"
	recommendationHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/recommendation"
	userHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/user"
	weatherHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/weather"
	"github.com/Soyaib10/farm-fusion/internal/infra/ml"
	"github.com/Soyaib10/farm-fusion/internal/infra/postgres"
	"github.com/Soyaib10/farm-fusion/internal/usecase/auth"
	farmUsecase "github.com/Soyaib10/farm-fusion/internal/usecase/farm"
	"github.com/Soyaib10/farm-fusion/internal/usecase/recommendation"
	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	weatherUsecase "github.com/Soyaib10/farm-fusion/internal/usecase/weather"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logger.NewLogger(os.Stdout, logger.LevelInfo)

	db, err := postgres.ConnectDB(context.Background(), cfg, logger)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	app := app.NewApplication(cfg, logger, db)

	userRepo := postgres.NewUserRepository(db)
	userUsecase := user.NewUseCase(userRepo)

	authRepo := postgres.NewAuthRepository(db)
	authUsecase := auth.NewUseCase(userRepo, authRepo, cfg)

	farmRepo := postgres.NewFarmRepository(db)
	farmUsecase := farmUsecase.NewUseCase(farmRepo)

	weatherRepo := postgres.NewWeatherRepository(db)
	weatherUsecase := weatherUsecase.NewUseCase(weatherRepo, farmUsecase)

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	mlClient := ml.NewMLClient(cfg.MLServiceURL, httpClient, logger)
	recommendationUsecase := recommendation.NewUseCase(mlClient)

	handlers := &httpDelivery.Handlers{
		User:           userHandler.NewHandler(app, userUsecase),
		Auth:           authHandler.NewHandler(app, authUsecase),
		Recommendation: recommendationHandler.NewHandler(app, recommendationUsecase),
		Farm:           farmHandler.NewHandler(app, farmUsecase),
		Weather:        weatherHandler.NewHandler(app, weatherUsecase),
	}
	router := httpDelivery.NewHandlers(handlers, cfg, app)

	port := ":" + cfg.ServerPort
	logger.PrintInfo("Server starting", map[string]string{"port": port})
	if err := http.ListenAndServe(port, router); err != nil {
		logger.PrintFatal(err, map[string]string{"error": "server failed to start"})
	}
}

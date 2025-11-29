package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/infra/ml"
	"github.com/Soyaib10/farm-fusion/internal/infra/postgres"
	"github.com/Soyaib10/farm-fusion/internal/usecase/auth"
	"github.com/Soyaib10/farm-fusion/internal/usecase/recommendation"
	"github.com/Soyaib10/farm-fusion/internal/usecase/user"
	httpDelivery "github.com/Soyaib10/farm-fusion/internal/delivery/http"
	authHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/auth"
	recommendationHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/recommendation"
	userHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/user"
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

	mlClient := ml.NewMLClient(cfg.MLServiceURL)
	recommendationUsecase := recommendation.NewUseCase(mlClient)

	handlers := &httpDelivery.Handlers{
		User:           userHandler.NewHandler(app, userUsecase),
		Auth:           authHandler.NewHandler(app, authUsecase),
		Recommendation: recommendationHandler.NewHandler(app, recommendationUsecase),
	}
	router := httpDelivery.NewHandlers(handlers, cfg, app)

	port := ":" + cfg.ServerPort
	logger.PrintInfo("Server starting", map[string]string{"port": port})
	if err := http.ListenAndServe(port, router); err != nil {
		logger.PrintFatal(err, map[string]string{"error": "server failed to start"})
	}
}

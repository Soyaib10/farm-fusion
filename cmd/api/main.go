package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/config"
	httpDelivery "github.com/Soyaib10/farm-fusion/internal/delivery/http"
	userHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/user"
	"github.com/Soyaib10/farm-fusion/internal/infra/postgres"
	userUsecase "github.com/Soyaib10/farm-fusion/internal/usecase/user"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logger.New(os.Stdout, logger.LevelInfo)

	db, err := postgres.ConnectDB(context.Background(), cfg, logger)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	application := app.New(cfg, logger, db)

	userRepo := postgres.NewUserRepositoryPG(db)
	userUsecase := userUsecase.New(userRepo)

	handlers := &httpDelivery.Handlers{
		User: userHandler.NewHandler(application, userUsecase),
	}
	router := httpDelivery.NewHandlers(handlers)

	port := ":" + cfg.ServerPort
	logger.PrintInfo("Server starting", map[string]string{"port": port})
	if err := http.ListenAndServe(port, router); err != nil {
		logger.PrintFatal(err, map[string]string{"error": "server failed to start"})
	}
}

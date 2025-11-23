package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/config"
	httpDelivery "github.com/Soyaib10/farm-fusion/internal/delivery/http"
	farmHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/farm"
	userHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/user"
	"github.com/Soyaib10/farm-fusion/internal/infra/postgres"
	farmUsecase "github.com/Soyaib10/farm-fusion/internal/usecase/farm"
	userUsecase "github.com/Soyaib10/farm-fusion/internal/usecase/user"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.ConnectDB(context.Background(), cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connection successful")

	userRepo := postgres.NewUserRepositoryPG(db)
	userUsecase := userUsecase.New(userRepo)

	farmRepo := postgres.NewFarmRepositoryPG(db)
	farmUsecase := farmUsecase.NewUseCase(farmRepo)

	handlers := &httpDelivery.Handlers{
		User: userHandler.NewHandler(userUsecase),
		Farm: farmHandler.NewHandler(farmUsecase),
	}
	router := httpDelivery.NewHandlers(handlers)

	port := ":" + cfg.ServerPort
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

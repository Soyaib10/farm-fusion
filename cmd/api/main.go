package main

import (
	"log"
	"net/http"

	"github.com/Soyaib10/farm-fusion/internal/config"
	httpDelivery "github.com/Soyaib10/farm-fusion/internal/delivery/http"
	userHandler "github.com/Soyaib10/farm-fusion/internal/delivery/http/user"
	"github.com/Soyaib10/farm-fusion/internal/infra/postgres"
	userUsecase "github.com/Soyaib10/farm-fusion/internal/usecase/user"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close() 
	log.Println("Database connection successful")

	userRepo := postgres.NewUserRepositoryPG(db)
	usecase := userUsecase.NewUseCase(userRepo)
	handlers := &httpDelivery.Handlers{
		User: userHandler.NewHandler(usecase),
	}
	router := httpDelivery.NewHandlers(handlers)
	log.Println("HTTP router initialized")

	port := ":" + cfg.ServerPort
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
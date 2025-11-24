package app

import (
	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Application struct {
	Config *config.Config
	Logger *logger.Logger
	DB     *pgxpool.Pool
}

func New(cfg *config.Config, logger *logger.Logger, db *pgxpool.Pool) *Application {
	return &Application{
		Config: cfg,
		Logger: logger,
		DB:     db,
	}
}

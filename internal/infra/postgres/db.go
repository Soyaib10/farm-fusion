package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database URL cannot be empty")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres url: %w", err)
	}

	poolConfig.MaxConns = cfg.DBPool.MaxConns
	poolConfig.MinConns = cfg.DBPool.MinConns
	poolConfig.MaxConnLifetime = cfg.DBPool.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.DBPool.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.DBPool.HealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = cfg.DBPool.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	fmt.Println("Successfully connected to PostgreSQL")
	return pool, nil
}

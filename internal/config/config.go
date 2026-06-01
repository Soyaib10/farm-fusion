package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort           string
	DatabaseURL          string
	DBPool               DBPoolConfig
	JWTSecret            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	MLServiceURL         string
	Redis                RedisConfig
	RabbitMQ             RabbitMQConfig
	Weather              WeatherConfig
	SMTP                 SMTPConfig
}

type RedisConfig struct {
	URL      string
	Addr     string
	Password string
	DB       int
}

type RabbitMQConfig struct {
	URL   string
	Queue string
}

type WeatherConfig struct {
	BaseURL  string
	APIKey   string
	CacheTTL int
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type DBPoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env found, using environment variables")
	}

	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	maxConns := getEnvInt32("DB_MAX_CONNS", 10)
	minConns := getEnvInt32("DB_MIN_CONNS", 2)

	if maxConns < 1 {
		return nil, fmt.Errorf("DB_MAX_CONNS must be at least 1")
	}
	if minConns < 0 {
		return nil, fmt.Errorf("DB_MIN_CONNS cannot be negative")
	}
	if minConns > maxConns {
		return nil, fmt.Errorf("DB_MIN_CONNS cannot be greater than DB_MAX_CONNS")
	}

	return &Config{
		ServerPort:  getEnv("SERVER_PORT", getEnv("PORT", "8080")),
		DatabaseURL: dbURL,
		DBPool: DBPoolConfig{
			MaxConns:          maxConns,
			MinConns:          minConns,
			MaxConnLifetime:   getEnvDuration("DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
			HealthCheckPeriod: getEnvDuration("DB_HEALTH_CHECK_PERIOD", 5*time.Minute),
			ConnectTimeout:    getEnvDuration("DB_CONNECT_TIMEOUT", 10*time.Second),
		},
		JWTSecret:            getEnv("JWT_SECRET", "your-secret-key"),
		AccessTokenDuration:  getEnvDuration("ACCESS_TOKEN_DURATION", 15*time.Minute),
		RefreshTokenDuration: getEnvDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		MLServiceURL:         getEnv("ML_SERVICE_URL", "http://localhost:8000"),
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", ""),
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		RabbitMQ: RabbitMQConfig{
			URL:   getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			Queue: getEnv("RABBITMQ_QUEUE", "weather.notifications"),
		},
		Weather: WeatherConfig{
			BaseURL:  getEnv("OPENWEATHER_BASE_URL", "https://api.openweathermap.org/data/2.5/forecast"),
			APIKey:   getEnv("OPENWEATHER_API_KEY", ""),
			CacheTTL: getEnvInt("WEATHER_CACHE_TTL", 10800),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:     getEnvInt("SMTP_PORT", 587),
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", getEnv("SMTP_PASS", "")),
			From:     getEnv("SMTP_FROM", "noreply@farmfusion.com"),
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt32(key string, fallback int32) int32 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(i)
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

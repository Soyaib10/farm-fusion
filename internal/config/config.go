package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const minJWTSecretLength = 32

// Config holds all application configuration loaded from the environment.
type Config struct {
	Env       string
	Server    ServerConfig
	Database  DatabaseConfig
	Auth      AuthConfig
	ML        MLConfig
	Redis     RedisConfig
	RabbitMQ  RabbitMQConfig
	Weather   WeatherConfig
	SMTP      SMTPConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
}

// IsProduction reports whether the application is running in production mode.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production") || strings.EqualFold(c.Env, "prod")
}

// ServerConfig configures the HTTP server and proxy behavior.
type ServerConfig struct {
	Host            string
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	TrustProxy      bool
}

// Addr returns the host:port listen address for net/http.
func (s ServerConfig) Addr() string {
	return s.Host + ":" + s.Port
}

// DatabaseConfig configures PostgreSQL connectivity and connection pooling.
type DatabaseConfig struct {
	URL               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

// AuthConfig configures JWT signing/verification and token lifetimes.
type AuthConfig struct {
	JWTSecret            string
	JWTIssuer            string
	JWTAudience          string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

// MLConfig configures connectivity to the internal Python ML service.
type MLConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// RedisConfig configures Redis connectivity.
type RedisConfig struct {
	URL      string
	Addr     string
	Password string
	DB       int
}

// RabbitMQConfig configures RabbitMQ connectivity and queue naming.
type RabbitMQConfig struct {
	URL   string
	Queue string
}

// WeatherConfig configures the OpenWeather forecast provider and cache TTL.
type WeatherConfig struct {
	BaseURL  string
	APIKey   string
	CacheTTL time.Duration
}

// SMTPConfig configures outgoing email delivery.
type SMTPConfig struct {
	Host       string
	Port       int
	User       string
	Password   string
	From       string
	RequireTLS bool
	Timeout    time.Duration
}

// CORSConfig configures Cross-Origin Resource Sharing policies.
type CORSConfig struct {
	AllowedOrigins []string
}

// RateLimitConfig configures per-IP token-bucket rate limiting.
type RateLimitConfig struct {
	Enabled   bool
	RPS       float64
	Burst     int
	AuthRPS   float64
	AuthBurst int
}

// Load reads environment variables (optionally loading .env first) and validates
// required settings and security invariants.
func Load() (*Config, error) {
	// Best-effort load of local .env file; production environments inject real env vars.
	_ = godotenv.Load()

	env := strings.ToLower(strings.TrimSpace(getEnv("ENV", "development")))

	cfg := &Config{
		Env: env,
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", ""),
			Port:            getEnv("SERVER_PORT", getEnv("PORT", "8080")),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
			TrustProxy:      getEnvBool("TRUST_PROXY", false),
		},
		Database: DatabaseConfig{
			URL:               getEnv("DATABASE_URL", ""),
			MaxConns:          getEnvInt32("DB_MAX_CONNS", 10),
			MinConns:          getEnvInt32("DB_MIN_CONNS", 2),
			MaxConnLifetime:   getEnvDuration("DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
			HealthCheckPeriod: getEnvDuration("DB_HEALTH_CHECK_PERIOD", 5*time.Minute),
			ConnectTimeout:    getEnvDuration("DB_CONNECT_TIMEOUT", 10*time.Second),
		},
		Auth: AuthConfig{
			JWTSecret:            getEnv("JWT_SECRET", ""),
			JWTIssuer:            getEnv("JWT_ISSUER", "farm-fusion-api"),
			JWTAudience:          getEnv("JWT_AUDIENCE", "farm-fusion-client"),
			AccessTokenDuration:  getEnvDuration("ACCESS_TOKEN_DURATION", 15*time.Minute),
			RefreshTokenDuration: getEnvDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		},
		ML: MLConfig{
			BaseURL: strings.TrimRight(getEnv("ML_SERVICE_URL", "http://127.0.0.1:8000"), "/"),
			APIKey:  getEnv("ML_API_KEY", ""),
			Timeout: getEnvDuration("ML_TIMEOUT", 10*time.Second),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", ""),
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		RabbitMQ: RabbitMQConfig{
			URL:   getEnv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
			Queue: getEnv("RABBITMQ_QUEUE", "weather.notifications"),
		},
		Weather: WeatherConfig{
			BaseURL:  getEnv("OPENWEATHER_BASE_URL", "https://api.openweathermap.org/data/2.5/forecast"),
			APIKey:   getEnv("OPENWEATHER_API_KEY", ""),
			CacheTTL: time.Duration(getEnvInt("WEATHER_CACHE_TTL", 10800)) * time.Second,
		},
		SMTP: SMTPConfig{
			Host:       getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:       getEnvInt("SMTP_PORT", 587),
			User:       getEnv("SMTP_USER", ""),
			Password:   getEnv("SMTP_PASSWORD", getEnv("SMTP_PASS", "")),
			From:       getEnv("SMTP_FROM", "noreply@farmfusion.com"),
			RequireTLS: getEnvBool("SMTP_REQUIRE_TLS", true),
			Timeout:    getEnvDuration("SMTP_TIMEOUT", 15*time.Second),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvCSV("CORS_ALLOWED_ORIGINS", nil),
		},
		RateLimit: RateLimitConfig{
			Enabled:   getEnvBool("RATE_LIMIT_ENABLED", true),
			RPS:       getEnvFloat("RATE_LIMIT_RPS", 20),
			Burst:     getEnvInt("RATE_LIMIT_BURST", 40),
			AuthRPS:   getEnvFloat("AUTH_RATE_LIMIT_RPS", 2),
			AuthBurst: getEnvInt("AUTH_RATE_LIMIT_BURST", 5),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadForAPI loads configuration and additionally enforces API-specific secrets (such as JWT_SECRET).
func LoadForAPI() (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateAuth(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks baseline configuration invariants shared across binaries.
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.Database.MaxConns < 1 {
		return errors.New("DB_MAX_CONNS must be at least 1")
	}
	if c.Database.MinConns < 0 {
		return errors.New("DB_MIN_CONNS cannot be negative")
	}
	if c.Database.MinConns > c.Database.MaxConns {
		return errors.New("DB_MIN_CONNS cannot be greater than DB_MAX_CONNS")
	}
	if port, err := strconv.Atoi(c.Server.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("SERVER_PORT must be a valid TCP port (1-65535), got %q", c.Server.Port)
	}
	if c.SMTP.Port < 1 || c.SMTP.Port > 65535 {
		return fmt.Errorf("SMTP_PORT must be a valid TCP port (1-65535), got %d", c.SMTP.Port)
	}
	if c.IsProduction() && strings.Contains(c.Database.URL, "sslmode=disable") {
		return errors.New("DATABASE_URL must not disable SSL in production")
	}
	for _, origin := range c.CORS.AllowedOrigins {
		if origin == "*" && c.IsProduction() {
			return errors.New("CORS_ALLOWED_ORIGINS cannot contain wildcard '*' in production")
		}
	}
	return nil
}

// ValidateAuth verifies cryptographic secret requirements for JWT authentication.
func (c *Config) ValidateAuth() error {
	secret := strings.TrimSpace(c.Auth.JWTSecret)
	if secret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(secret) < minJWTSecretLength {
		return fmt.Errorf("JWT_SECRET must be at least %d bytes long for HS256 security", minJWTSecretLength)
	}
	lower := strings.ToLower(secret)
	if c.IsProduction() && (strings.Contains(lower, "change-me") || strings.Contains(lower, "your-secret")) {
		return errors.New("JWT_SECRET contains an insecure placeholder value in production")
	}
	if c.Auth.AccessTokenDuration <= 0 || c.Auth.AccessTokenDuration > 24*time.Hour {
		return errors.New("ACCESS_TOKEN_DURATION must be between 1s and 24h")
	}
	if c.Auth.RefreshTokenDuration <= 0 {
		return errors.New("REFRESH_TOKEN_DURATION must be positive")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
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

func getEnvInt32(key string, fallback int32) int32 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(i)
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
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

func getEnvCSV(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

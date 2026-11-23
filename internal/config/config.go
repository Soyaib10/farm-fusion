package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	minJWTSecretLength      = 32
	minMLAPIKeyLength       = 32
	maxAccessTokenDuration  = 15 * time.Minute
	maxRefreshTokenDuration = 30 * 24 * time.Hour
)

// knownExampleSecrets lists secret values shipped in this repository (.env.example, README).
// Anyone who has read the repo knows them, so they are rejected in every environment.
var knownExampleSecrets = []string{
	"replace-with-a-secure-random-32-byte-secret-key",
	"replace-with-a-secure-random-32-byte-ml-api-key",
}

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
	Scheduler SchedulerConfig
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
	// TrustedProxies lists the CIDRs whose X-Forwarded-For entries are honored when TrustProxy is set.
	TrustedProxies []netip.Prefix
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
	StatementTimeout  time.Duration
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

// SchedulerConfig configures the daily weather evaluation run.
type SchedulerConfig struct {
	Hour     int
	Location *time.Location
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

	trustedProxies, err := parseCIDRs(getEnvCSV("TRUSTED_PROXY_CIDRS", nil))
	if err != nil {
		return nil, err
	}

	schedulerLocation, err := time.LoadLocation(getEnv("SCHEDULER_TIMEZONE", "Local"))
	if err != nil {
		return nil, fmt.Errorf("SCHEDULER_TIMEZONE is not a valid IANA zone: %w", err)
	}

	cfg := &Config{
		Env: env,
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "127.0.0.1"),
			Port:            getEnv("SERVER_PORT", getEnv("PORT", "8080")),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
			TrustProxy:      getEnvBool("TRUST_PROXY", false),
			TrustedProxies:  trustedProxies,
		},
		Database: DatabaseConfig{
			URL:               getEnv("DATABASE_URL", ""),
			MaxConns:          getEnvInt32("DB_MAX_CONNS", 10),
			MinConns:          getEnvInt32("DB_MIN_CONNS", 2),
			MaxConnLifetime:   getEnvDuration("DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
			HealthCheckPeriod: getEnvDuration("DB_HEALTH_CHECK_PERIOD", 5*time.Minute),
			ConnectTimeout:    getEnvDuration("DB_CONNECT_TIMEOUT", 10*time.Second),
			StatementTimeout:  getEnvDuration("DB_STATEMENT_TIMEOUT", 30*time.Second),
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
		Scheduler: SchedulerConfig{
			Hour:     getEnvInt("SCHEDULER_HOUR", 5),
			Location: schedulerLocation,
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
	if err := cfg.ValidateML(); err != nil {
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
	if c.Database.StatementTimeout < 0 {
		return errors.New("DB_STATEMENT_TIMEOUT cannot be negative")
	}
	if c.IsProduction() {
		if err := validateProductionSSLMode(c.Database.URL); err != nil {
			return err
		}
		if rabbitUsesGuest(c.RabbitMQ.URL) {
			return errors.New("RABBITMQ_URL must not use the default guest account in production")
		}
	}
	for _, origin := range c.CORS.AllowedOrigins {
		if origin == "*" && c.IsProduction() {
			return errors.New("CORS_ALLOWED_ORIGINS cannot contain wildcard '*' in production")
		}
	}
	if c.Server.TrustProxy && len(c.Server.TrustedProxies) == 0 {
		return errors.New("TRUSTED_PROXY_CIDRS is required when TRUST_PROXY is enabled")
	}
	if c.RateLimit.Enabled {
		if c.RateLimit.RPS <= 0 || c.RateLimit.Burst < 1 {
			return errors.New("RATE_LIMIT_RPS must be positive and RATE_LIMIT_BURST at least 1")
		}
		if c.RateLimit.AuthRPS <= 0 || c.RateLimit.AuthBurst < 1 {
			return errors.New("AUTH_RATE_LIMIT_RPS must be positive and AUTH_RATE_LIMIT_BURST at least 1")
		}
	}
	if c.Weather.CacheTTL <= 0 {
		return errors.New("WEATHER_CACHE_TTL must be a positive number of seconds")
	}
	if c.Scheduler.Hour < 0 || c.Scheduler.Hour > 23 {
		return fmt.Errorf("SCHEDULER_HOUR must be between 0 and 23, got %d", c.Scheduler.Hour)
	}
	return nil
}

// ValidateML verifies the shared secret used to authenticate calls to the internal ML service.
func (c *Config) ValidateML() error {
	key := strings.TrimSpace(c.ML.APIKey)
	if len(key) < minMLAPIKeyLength {
		return fmt.Errorf("ML_API_KEY is required and must be at least %d bytes long", minMLAPIKeyLength)
	}
	if isKnownExampleSecret(key) {
		return errors.New("ML_API_KEY is the example value from the repository; generate a new one")
	}
	return nil
}

// validateProductionSSLMode requires certificate-verifying TLS for PostgreSQL. pgx treats a missing
// sslmode as "prefer" (unverified TLS with plaintext fallback), and "require" skips verification.
func validateProductionSSLMode(dsn string) error {
	params := databaseParams(dsn)
	mode := params["sslmode"]
	if mode == "" {
		mode = os.Getenv("PGSSLMODE")
	}
	switch mode {
	case "verify-full":
		return nil
	case "verify-ca":
		if params["sslrootcert"] == "" && os.Getenv("PGSSLROOTCERT") == "" {
			return errors.New("DATABASE_URL with sslmode=verify-ca requires an explicit sslrootcert in production")
		}
		return nil
	case "":
		return errors.New("DATABASE_URL must set sslmode=verify-full in production")
	default:
		return fmt.Errorf("DATABASE_URL sslmode=%s does not verify the server certificate; use verify-full in production", mode)
	}
}

// databaseParams extracts connection parameters from a postgres:// URL or a key=value DSN.
func databaseParams(dsn string) map[string]string {
	params := make(map[string]string)
	trimmed := strings.TrimSpace(dsn)
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return params
		}
		for k, v := range u.Query() {
			if len(v) > 0 {
				params[k] = v[len(v)-1]
			}
		}
		return params
	}
	for _, field := range strings.Fields(trimmed) {
		if k, v, ok := strings.Cut(field, "="); ok {
			params[k] = strings.Trim(v, `'`)
		}
	}
	return params
}

func rabbitUsesGuest(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return false
	}
	pass, _ := u.User.Password()
	return u.User.Username() == "guest" || pass == "guest"
}

func isKnownExampleSecret(secret string) bool {
	for _, known := range knownExampleSecrets {
		if strings.EqualFold(secret, known) {
			return true
		}
	}
	return false
}

func parseCIDRs(raw []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, entry := range raw {
		if !strings.Contains(entry, "/") {
			addr, err := netip.ParseAddr(entry)
			if err != nil {
				return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS contains invalid address %q", entry)
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
			continue
		}
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS contains invalid CIDR %q", entry)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
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
	if isKnownExampleSecret(secret) {
		return errors.New("JWT_SECRET is the example value from the repository; generate one with: openssl rand -hex 32")
	}
	lower := strings.ToLower(secret)
	if c.IsProduction() && (strings.Contains(lower, "change-me") || strings.Contains(lower, "your-secret") ||
		strings.Contains(lower, "replace-with")) {
		return errors.New("JWT_SECRET contains an insecure placeholder value in production")
	}
	if c.Auth.AccessTokenDuration <= 0 || c.Auth.AccessTokenDuration > maxAccessTokenDuration {
		return fmt.Errorf("ACCESS_TOKEN_DURATION must be between 1s and %s", maxAccessTokenDuration)
	}
	if c.Auth.RefreshTokenDuration <= 0 || c.Auth.RefreshTokenDuration > maxRefreshTokenDuration {
		return fmt.Errorf("REFRESH_TOKEN_DURATION must be between 1s and %s", maxRefreshTokenDuration)
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

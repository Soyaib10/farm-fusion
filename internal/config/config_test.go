package config

import (
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfigValidateAuth(t *testing.T) {
	base := func() Config {
		return Config{
			Env:      "development",
			Server:   ServerConfig{Port: "8080"},
			Database: DatabaseConfig{URL: "postgres://u:p@127.0.0.1:5432/db?sslmode=disable", MaxConns: 10, MinConns: 2},
			SMTP:     SMTPConfig{Port: 587},
			Weather:  WeatherConfig{CacheTTL: 3 * time.Hour},
			RabbitMQ: RabbitMQConfig{URL: "amqp://farm:s3cret@127.0.0.1:5672/"},
			Auth: AuthConfig{
				JWTSecret:            "0123456789abcdef0123456789abcdef",
				AccessTokenDuration:  15 * time.Minute,
				RefreshTokenDuration: time.Hour,
			},
			ML: MLConfig{APIKey: "fedcba9876543210fedcba9876543210"},
		}
	}

	t.Run("valid config passes", func(t *testing.T) {
		cfg := base()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
		if err := cfg.ValidateAuth(); err != nil {
			t.Fatalf("ValidateAuth() unexpected error: %v", err)
		}
		if err := cfg.ValidateML(); err != nil {
			t.Fatalf("ValidateML() unexpected error: %v", err)
		}
	})

	t.Run("missing JWT_SECRET fails", func(t *testing.T) {
		cfg := base()
		cfg.Auth.JWTSecret = ""
		if err := cfg.ValidateAuth(); err == nil {
			t.Fatal("expected error when JWT_SECRET is empty")
		}
	})

	t.Run("short JWT_SECRET fails", func(t *testing.T) {
		cfg := base()
		cfg.Auth.JWTSecret = "too-short-secret"
		if err := cfg.ValidateAuth(); err == nil {
			t.Fatal("expected error when JWT_SECRET < 32 bytes")
		}
	})

	t.Run("access token lifetime is capped", func(t *testing.T) {
		cfg := base()
		cfg.Auth.AccessTokenDuration = 24 * time.Hour
		if err := cfg.ValidateAuth(); err == nil {
			t.Fatal("expected error for a 24h access token lifetime")
		}
	})

	t.Run("refresh token lifetime is capped", func(t *testing.T) {
		cfg := base()
		cfg.Auth.RefreshTokenDuration = 365 * 24 * time.Hour
		if err := cfg.ValidateAuth(); err == nil {
			t.Fatal("expected error for a one-year refresh token lifetime")
		}
	})

	t.Run("production rejects weak sslmodes", func(t *testing.T) {
		for _, dsn := range []string{
			"postgres://u:p@db.internal:5432/db?sslmode=disable",
			"postgres://u:p@db.internal:5432/db?sslmode=allow",
			"postgres://u:p@db.internal:5432/db?sslmode=prefer",
			"postgres://u:p@db.internal:5432/db?sslmode=require",
			"postgres://u:p@db.internal:5432/db",
			"postgres://u:p@db.internal:5432/db?sslmode=verify-ca",
			"host=db.internal user=u sslmode=require",
		} {
			cfg := base()
			cfg.Env = "production"
			cfg.Database.URL = dsn
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected production validation to reject %q", dsn)
			}
		}
	})

	t.Run("production accepts verified TLS", func(t *testing.T) {
		for _, dsn := range []string{
			"postgres://u:p@db.internal:5432/db?sslmode=verify-full",
			"postgres://u:p@db.internal:5432/db?sslmode=verify-ca&sslrootcert=/etc/ssl/db-ca.pem",
			"host=db.internal user=u sslmode=verify-full",
		} {
			cfg := base()
			cfg.Env = "production"
			cfg.Database.URL = dsn
			if err := cfg.Validate(); err != nil {
				t.Fatalf("expected production validation to accept %q, got %v", dsn, err)
			}
		}
	})

	t.Run("production rejects guest RabbitMQ account", func(t *testing.T) {
		cfg := base()
		cfg.Env = "production"
		cfg.Database.URL = "postgres://u:p@db.internal:5432/db?sslmode=verify-full"
		cfg.RabbitMQ.URL = "amqp://guest:guest@mq.internal:5672/"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected production validation to reject guest RabbitMQ credentials")
		}
	})

	t.Run("production rejects wildcard CORS", func(t *testing.T) {
		cfg := base()
		cfg.Env = "production"
		cfg.Database.URL = "postgres://u:p@db.internal:5432/db?sslmode=verify-full"
		cfg.CORS.AllowedOrigins = []string{"*"}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected production validation to reject wildcard CORS")
		}
	})

	t.Run("production rejects placeholder JWT_SECRET", func(t *testing.T) {
		cfg := base()
		cfg.Env = "production"
		cfg.Auth.JWTSecret = "change-me-in-production-0123456789abcdef"
		if err := cfg.ValidateAuth(); err == nil {
			t.Fatal("expected production validation to reject placeholder secret")
		}
	})

	t.Run("TRUST_PROXY requires proxy CIDRs", func(t *testing.T) {
		cfg := base()
		cfg.Server.TrustProxy = true
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error when TRUST_PROXY has no TRUSTED_PROXY_CIDRS")
		}
		cfg.Server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("non-positive rate limits fail", func(t *testing.T) {
		cfg := base()
		cfg.RateLimit = RateLimitConfig{Enabled: true, RPS: 0, Burst: 40, AuthRPS: 2, AuthBurst: 5}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for RATE_LIMIT_RPS=0")
		}
	})

	t.Run("zero weather cache TTL fails", func(t *testing.T) {
		cfg := base()
		cfg.Weather.CacheTTL = 0
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for WEATHER_CACHE_TTL=0")
		}
	})

	t.Run("short or missing ML_API_KEY fails", func(t *testing.T) {
		cfg := base()
		cfg.ML.APIKey = ""
		if err := cfg.ValidateML(); err == nil {
			t.Fatal("expected error for empty ML_API_KEY")
		}
		cfg.ML.APIKey = "short"
		if err := cfg.ValidateML(); err == nil {
			t.Fatal("expected error for short ML_API_KEY")
		}
	})
}

func TestEnvExampleSecretsAreRejected(t *testing.T) {
	values := readEnvFile(t, "../../.env.example")

	for _, env := range []string{"production", "development"} {
		cfg := Config{
			Env: env,
			Auth: AuthConfig{
				JWTSecret:            values["JWT_SECRET"],
				AccessTokenDuration:  15 * time.Minute,
				RefreshTokenDuration: time.Hour,
			},
			ML: MLConfig{APIKey: values["ML_API_KEY"]},
		}
		if err := cfg.ValidateAuth(); err == nil {
			t.Fatalf("ENV=%s: ValidateAuth accepted the JWT_SECRET from .env.example", env)
		}
		if err := cfg.ValidateML(); err == nil {
			t.Fatalf("ENV=%s: ValidateML accepted the ML_API_KEY from .env.example", env)
		}
	}
}

func readEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			values[k] = v
		}
	}
	return values
}

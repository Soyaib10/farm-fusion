package config

import "testing"

func TestConfigValidateAuth(t *testing.T) {
	base := func() Config {
		return Config{
			Env:      "development",
			Server:   ServerConfig{Port: "8080"},
			Database: DatabaseConfig{URL: "postgres://u:p@127.0.0.1:5432/db?sslmode=disable", MaxConns: 10, MinConns: 2},
			SMTP:     SMTPConfig{Port: 587},
			Auth: AuthConfig{
				JWTSecret:            "0123456789abcdef0123456789abcdef",
				AccessTokenDuration:  900_000_000_000,
				RefreshTokenDuration: 3600_000_000_000,
			},
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

	t.Run("production rejects sslmode=disable", func(t *testing.T) {
		cfg := base()
		cfg.Env = "production"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected production validation to reject sslmode=disable")
		}
	})

	t.Run("production rejects wildcard CORS", func(t *testing.T) {
		cfg := base()
		cfg.Env = "production"
		cfg.Database.URL = "postgres://u:p@db.internal:5432/db?sslmode=require"
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
}

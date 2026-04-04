package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected default database url")
	}
	if cfg.JWTSecret != "very-secret-key" {
		t.Fatalf("expected default jwt secret, got %s", cfg.JWTSecret)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://custom")
	t.Setenv("JWT_SECRET", "custom-secret")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Fatalf("expected overridden port, got %s", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://custom" {
		t.Fatalf("expected overridden database url, got %s", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "custom-secret" {
		t.Fatalf("expected overridden jwt secret, got %s", cfg.JWTSecret)
	}
}

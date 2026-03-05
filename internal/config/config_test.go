package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	// List of environment variables used by the config package
	envVars := []string{
		"PORT", "ENVIRONMENT", "DATABASE_URL", "JWT_SECRET", "JWT_EXPIRATION_HOURS",
		"STORAGE_PATH", "WORKER_COUNT", "ALLOWED_ORIGINS", "APP_URL",
		"RESEND_API_KEY", "FROM_EMAIL", "ENABLE_EMAIL_NOTIFICATIONS",
		"ROLLBAR_TOKEN", "ROLLBAR_CODE_VERSION", "ROLLBAR_SERVER_ROOT",
		"DEFAULT_EMAIL", "DEFAULT_PASSWORD",
	}

	// Helper to unset all environment variables for a clean test
	cleanEnv := func(t *testing.T) {
		for _, v := range envVars {
			t.Setenv(v, "")
			// os.Unsetenv is needed because t.Setenv(v, "") sets it to empty string,
			// and getEnv uses os.LookupEnv which treats empty string as "exists".
			os.Unsetenv(v)
		}
	}

	t.Run("successful load with all environment variables", func(t *testing.T) {
		cleanEnv(t)

		t.Setenv("PORT", "9090")
		t.Setenv("ENVIRONMENT", "production")
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		t.Setenv("JWT_SECRET", "super-secret")
		t.Setenv("JWT_EXPIRATION_HOURS", "48")
		t.Setenv("STORAGE_PATH", "/tmp/storage")
		t.Setenv("WORKER_COUNT", "10")
		t.Setenv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:4000")
		t.Setenv("APP_URL", "https://api.example.com")
		t.Setenv("RESEND_API_KEY", "re_123")
		t.Setenv("FROM_EMAIL", "test@example.com")
		t.Setenv("ENABLE_EMAIL_NOTIFICATIONS", "true")
		t.Setenv("ROLLBAR_TOKEN", "rb_123")
		t.Setenv("ROLLBAR_CODE_VERSION", "v1.0.0")
		t.Setenv("ROLLBAR_SERVER_ROOT", "github.com/org/repo")
		t.Setenv("DEFAULT_EMAIL", "admin@example.com")
		t.Setenv("DEFAULT_PASSWORD", "admin123")

		cfg, err := Load()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config to be not nil")
		}

		if cfg.Port != "9090" {
			t.Errorf("expected Port 9090, got %s", cfg.Port)
		}
		if cfg.Environment != "production" {
			t.Errorf("expected Environment production, got %s", cfg.Environment)
		}
		if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
			t.Errorf("expected DatabaseURL postgres://user:pass@localhost:5432/db, got %s", cfg.DatabaseURL)
		}
		if cfg.JWTSecret != "super-secret" {
			t.Errorf("expected JWTSecret super-secret, got %s", cfg.JWTSecret)
		}
		if cfg.JWTExpirationHours != 48 {
			t.Errorf("expected JWTExpirationHours 48, got %d", cfg.JWTExpirationHours)
		}
		if cfg.StoragePath != "/tmp/storage" {
			t.Errorf("expected StoragePath /tmp/storage, got %s", cfg.StoragePath)
		}
		if cfg.WorkerCount != 10 {
			t.Errorf("expected WorkerCount 10, got %d", cfg.WorkerCount)
		}
		expectedOrigins := []string{"http://localhost:3000", "http://localhost:4000"}
		if !reflect.DeepEqual(cfg.AllowedOrigins, expectedOrigins) {
			t.Errorf("expected AllowedOrigins %v, got %v", expectedOrigins, cfg.AllowedOrigins)
		}
		if cfg.AppURL != "https://api.example.com" {
			t.Errorf("expected AppURL https://api.example.com, got %s", cfg.AppURL)
		}
		if cfg.ResendAPIKey != "re_123" {
			t.Errorf("expected ResendAPIKey re_123, got %s", cfg.ResendAPIKey)
		}
		if cfg.FromEmail != "test@example.com" {
			t.Errorf("expected FromEmail test@example.com, got %s", cfg.FromEmail)
		}
		if !cfg.EnableEmailNotifications {
			t.Errorf("expected EnableEmailNotifications true, got false")
		}
		if cfg.RollbarToken != "rb_123" {
			t.Errorf("expected RollbarToken rb_123, got %s", cfg.RollbarToken)
		}
		if cfg.RollbarCodeVersion != "v1.0.0" {
			t.Errorf("expected RollbarCodeVersion v1.0.0, got %s", cfg.RollbarCodeVersion)
		}
		if cfg.RollbarServerRoot != "github.com/org/repo" {
			t.Errorf("expected RollbarServerRoot github.com/org/repo, got %s", cfg.RollbarServerRoot)
		}
		if cfg.DefaultEmail != "admin@example.com" {
			t.Errorf("expected DefaultEmail admin@example.com, got %s", cfg.DefaultEmail)
		}
		if cfg.DefaultPassword != "admin123" {
			t.Errorf("expected DefaultPassword admin123, got %s", cfg.DefaultPassword)
		}
	})

	t.Run("default values when environment variables are missing", func(t *testing.T) {
		cleanEnv(t)

		// DATABASE_URL is required, so we must set it
		t.Setenv("DATABASE_URL", "postgres://localhost/db")

		cfg, err := Load()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config to be not nil")
		}

		if cfg.Port != "8080" {
			t.Errorf("expected Port 8080, got %s", cfg.Port)
		}
		if cfg.Environment != "development" {
			t.Errorf("expected Environment development, got %s", cfg.Environment)
		}
		if cfg.JWTSecret != "dev-secret-change-in-production" {
			t.Errorf("expected JWTSecret dev-secret-change-in-production, got %s", cfg.JWTSecret)
		}
		if cfg.JWTExpirationHours != 24 {
			t.Errorf("expected JWTExpirationHours 24, got %d", cfg.JWTExpirationHours)
		}
		if cfg.StoragePath != "./storage" {
			t.Errorf("expected StoragePath ./storage, got %s", cfg.StoragePath)
		}
		if cfg.WorkerCount != 5 {
			t.Errorf("expected WorkerCount 5, got %d", cfg.WorkerCount)
		}
		expectedOrigins := []string{"*"}
		if !reflect.DeepEqual(cfg.AllowedOrigins, expectedOrigins) {
			t.Errorf("expected AllowedOrigins %v, got %v", expectedOrigins, cfg.AllowedOrigins)
		}
		if cfg.AppURL != "https://fintera.securexapp.com" {
			t.Errorf("expected AppURL https://fintera.securexapp.com, got %s", cfg.AppURL)
		}
		if cfg.FromEmail != "noreply@fintera.app" {
			t.Errorf("expected FromEmail noreply@fintera.app, got %s", cfg.FromEmail)
		}
		if cfg.EnableEmailNotifications {
			t.Errorf("expected EnableEmailNotifications false, got true")
		}
		if cfg.RollbarServerRoot != "github.com/sjperalta/fintera-api" {
			t.Errorf("expected RollbarServerRoot github.com/sjperalta/fintera-api, got %s", cfg.RollbarServerRoot)
		}
	})

	t.Run("missing DATABASE_URL", func(t *testing.T) {
		cleanEnv(t)

		cfg, err := Load()
		if err == nil {
			t.Error("expected error, got nil")
		}
		if cfg != nil {
			t.Error("expected nil config, got non-nil")
		}
		if err != nil && !strings.Contains(err.Error(), "DATABASE_URL is required") {
			t.Errorf("expected error containing 'DATABASE_URL is required', got %v", err)
		}
	})

	t.Run("missing JWT_SECRET in production", func(t *testing.T) {
		cleanEnv(t)

		t.Setenv("DATABASE_URL", "postgres://localhost/db")
		t.Setenv("ENVIRONMENT", "production")

		cfg, err := Load()
		if err == nil {
			t.Error("expected error, got nil")
		}
		if cfg != nil {
			t.Error("expected nil config, got non-nil")
		}
		if err != nil && !strings.Contains(err.Error(), "JWT_SECRET is required in production") {
			t.Errorf("expected error containing 'JWT_SECRET is required in production', got %v", err)
		}
	})
}

func TestHelpers(t *testing.T) {
	t.Run("getEnv", func(t *testing.T) {
		key := "TEST_ENV_VAR"
		os.Unsetenv(key)
		if val := getEnv(key, "default"); val != "default" {
			t.Errorf("expected default, got %s", val)
		}

		t.Setenv(key, "value")
		if val := getEnv(key, "default"); val != "value" {
			t.Errorf("expected value, got %s", val)
		}
	})

	t.Run("getEnvAsInt", func(t *testing.T) {
		key := "TEST_INT_VAR"
		os.Unsetenv(key)
		if val := getEnvAsInt(key, 123); val != 123 {
			t.Errorf("expected 123, got %d", val)
		}

		t.Setenv(key, "456")
		if val := getEnvAsInt(key, 123); val != 456 {
			t.Errorf("expected 456, got %d", val)
		}

		t.Setenv(key, "not-an-int")
		if val := getEnvAsInt(key, 123); val != 123 {
			t.Errorf("expected 123, got %d", val)
		}
	})

	t.Run("getEnvAsBool", func(t *testing.T) {
		key := "TEST_BOOL_VAR"
		os.Unsetenv(key)
		if val := getEnvAsBool(key, true); val != true {
			t.Errorf("expected true, got %v", val)
		}
		if val := getEnvAsBool(key, false); val != false {
			t.Errorf("expected false, got %v", val)
		}

		t.Setenv(key, "true")
		if val := getEnvAsBool(key, false); val != true {
			t.Errorf("expected true, got %v", val)
		}

		t.Setenv(key, "1")
		if val := getEnvAsBool(key, false); val != true {
			t.Errorf("expected true, got %v", val)
		}

		t.Setenv(key, "false")
		if val := getEnvAsBool(key, true); val != false {
			t.Errorf("expected false, got %v", val)
		}

		t.Setenv(key, "not-a-bool")
		if val := getEnvAsBool(key, true); val != true {
			t.Errorf("expected true, got %v", val)
		}
	})

	t.Run("getEnvAsSlice", func(t *testing.T) {
		key := "TEST_SLICE_VAR"
		os.Unsetenv(key)
		defaultSlice := []string{"a", "b"}
		if val := getEnvAsSlice(key, defaultSlice); !reflect.DeepEqual(val, defaultSlice) {
			t.Errorf("expected %v, got %v", defaultSlice, val)
		}

		t.Setenv(key, "x,y,z")
		expected := []string{"x", "y", "z"}
		if val := getEnvAsSlice(key, defaultSlice); !reflect.DeepEqual(val, expected) {
			t.Errorf("expected %v, got %v", expected, val)
		}
	})
}

package config

import (
	"os"
	"strings"
	"testing"
)

func TestConfig_DevelopmentDefaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("APP_ENV")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("ALLOWED_ORIGINS")

	cfg := Load()
	if cfg.Environment != "development" {
		t.Errorf("expected development env, got %s", cfg.Environment)
	}
	if cfg.DatabaseURL != DefaultDevDatabaseURL {
		t.Errorf("expected default dev DB URL, got %s", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != DefaultDevJWTSecret {
		t.Errorf("expected default dev JWT secret, got %s", cfg.JWTSecret)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected dev config to validate cleanly, got err: %v", err)
	}
}

func TestConfig_ProductionRejections(t *testing.T) {
	// Case 1: Production with missing DATABASE_URL
	t.Run("Missing DATABASE_URL rejected", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Unsetenv("DATABASE_URL")
		os.Setenv("JWT_SECRET", "super-secret-jwt-key-with-over-32-characters!")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("JWT_SECRET")
		}()

		cfg := Load()
		if cfg.DatabaseURL != "" {
			t.Errorf("expected empty DatabaseURL when omitted in production, got: %s", cfg.DatabaseURL)
		}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "DATABASE_URL is required") {
			t.Errorf("expected error for missing DATABASE_URL in production, got: %v", err)
		}
	})

	// Case 2: Production with Default Dev Database URL rejected
	t.Run("Default dev DB URL rejected in production", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", DefaultDevDatabaseURL)
		os.Setenv("JWT_SECRET", "super-secret-jwt-key-with-over-32-characters!")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "cannot use the development fallback database") {
			t.Errorf("expected error for dev DB URL in production, got: %v", err)
		}
	})

	// Case 3: Production with missing JWT_SECRET rejected
	t.Run("Missing JWT_SECRET rejected in production", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Unsetenv("JWT_SECRET")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
		}()

		cfg := Load()
		if cfg.JWTSecret != "" {
			t.Errorf("expected empty JWTSecret when omitted in production, got: %s", cfg.JWTSecret)
		}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "JWT_SECRET is required") {
			t.Errorf("expected error for missing JWT_SECRET in production, got: %v", err)
		}
	})

	// Case 4: Production with default dev JWT_SECRET rejected
	t.Run("Default dev JWT_SECRET rejected in production", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", DefaultDevJWTSecret)
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "cannot use the development fallback key") {
			t.Errorf("expected error for default dev JWT_SECRET in production, got: %v", err)
		}
	})

	// Case 5: Production with short JWT_SECRET rejected (< 32 chars)
	t.Run("Short JWT_SECRET rejected in production", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "short_secret_key_123")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "too short") {
			t.Errorf("expected error for short JWT_SECRET in production, got: %v", err)
		}
	})

	// Case 6: Production with missing ALLOWED_ORIGINS rejected
	t.Run("Missing ALLOWED_ORIGINS rejected in production", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "super-secret-jwt-production-key-minimum-32-chars-long!")
		os.Unsetenv("ALLOWED_ORIGINS")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "ALLOWED_ORIGINS is required") {
			t.Errorf("expected error for missing ALLOWED_ORIGINS in production, got: %v", err)
		}
	})

	// Case 7: Production with wildcard origin rejected
	t.Run("Wildcard origin rejected in production", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "super-secret-jwt-production-key-minimum-32-chars-long!")
		os.Setenv("ALLOWED_ORIGINS", "https://*.onrender.com")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
			os.Unsetenv("ALLOWED_ORIGINS")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "wildcard origin") {
			t.Errorf("expected error for wildcard origin in production, got: %v", err)
		}
	})

	// Case 8: Production valid configuration succeeds with explicit origins
	t.Run("Valid production configuration succeeds", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "super-secret-jwt-production-key-minimum-32-chars-long!")
		os.Setenv("ALLOWED_ORIGINS", "https://ratels-web.onrender.com,https://ratels.org.ng")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
			os.Unsetenv("ALLOWED_ORIGINS")
		}()

		cfg, err := LoadAndValidate()
		if err != nil {
			t.Errorf("expected valid production config to pass, got err: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
	})
}

func TestConfig_AllowedOrigins(t *testing.T) {
	os.Setenv("ALLOWED_ORIGINS", "https://ratels-frontend.onrender.com, https://ratels.org.ng")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	cfg := Load()
	foundRender := false
	foundCustom := false
	for _, o := range cfg.AllowedOrigins {
		if o == "https://ratels-frontend.onrender.com" {
			foundRender = true
		}
		if o == "https://ratels.org.ng" {
			foundCustom = true
		}
	}

	if !foundRender || !foundCustom {
		t.Errorf("expected parsed custom origins in cfg.AllowedOrigins, got: %v", cfg.AllowedOrigins)
	}
}

func TestConfig_EmailProviderConfiguration(t *testing.T) {
	t.Run("Explicit EMAIL_PROVIDER=brevo with valid settings passes validation", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "super-secret-jwt-production-key-minimum-32-chars-long!")
		os.Setenv("ALLOWED_ORIGINS", "https://ratels-web.onrender.com")
		os.Setenv("EMAIL_PROVIDER", "brevo")
		os.Setenv("BREVO_API_KEY", "xkeysib-mocktestkey")
		os.Setenv("EMAIL_FROM_NAME", "Nigerian Ratels")
		os.Setenv("EMAIL_FROM_ADDRESS", "info@ratels.org.ng")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
			os.Unsetenv("ALLOWED_ORIGINS")
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("BREVO_API_KEY")
			os.Unsetenv("EMAIL_FROM_NAME")
			os.Unsetenv("EMAIL_FROM_ADDRESS")
		}()

		cfg, err := LoadAndValidate()
		if err != nil {
			t.Fatalf("expected valid brevo config to pass, got err: %v", err)
		}
		if cfg.EmailProvider != "brevo" {
			t.Errorf("expected EmailProvider 'brevo', got %q", cfg.EmailProvider)
		}
		if cfg.EmailFromName != "Nigerian Ratels" {
			t.Errorf("expected EmailFromName 'Nigerian Ratels', got %q", cfg.EmailFromName)
		}
		if cfg.EmailFromAddress != "info@ratels.org.ng" {
			t.Errorf("expected EmailFromAddress 'info@ratels.org.ng', got %q", cfg.EmailFromAddress)
		}
	})

	t.Run("Production with EMAIL_PROVIDER=brevo requires BREVO_API_KEY", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "super-secret-jwt-production-key-minimum-32-chars-long!")
		os.Setenv("ALLOWED_ORIGINS", "https://ratels-web.onrender.com")
		os.Setenv("EMAIL_PROVIDER", "brevo")
		os.Unsetenv("BREVO_API_KEY")
		os.Setenv("EMAIL_FROM_ADDRESS", "info@ratels.org.ng")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
			os.Unsetenv("ALLOWED_ORIGINS")
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("EMAIL_FROM_ADDRESS")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "BREVO_API_KEY is required") {
			t.Errorf("expected error for missing BREVO_API_KEY, got: %v", err)
		}
	})

	t.Run("Production with EMAIL_PROVIDER=brevo requires valid EMAIL_FROM_ADDRESS", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "super-secret-jwt-production-key-minimum-32-chars-long!")
		os.Setenv("ALLOWED_ORIGINS", "https://ratels-web.onrender.com")
		os.Setenv("EMAIL_PROVIDER", "brevo")
		os.Setenv("BREVO_API_KEY", "xkeysib-mocktestkey")
		os.Setenv("EMAIL_FROM_ADDRESS", "invalid-no-at-sign")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
			os.Unsetenv("ALLOWED_ORIGINS")
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("BREVO_API_KEY")
			os.Unsetenv("EMAIL_FROM_ADDRESS")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "EMAIL_FROM_ADDRESS must be a valid email") {
			t.Errorf("expected error for invalid EMAIL_FROM_ADDRESS, got: %v", err)
		}
	})

	t.Run("Production rejects EMAIL_PROVIDER=dev", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("DATABASE_URL", "postgres://user:pass@ep-production-host.neon.tech/dbname?sslmode=require")
		os.Setenv("JWT_SECRET", "super-secret-jwt-production-key-minimum-32-chars-long!")
		os.Setenv("ALLOWED_ORIGINS", "https://ratels-web.onrender.com")
		os.Setenv("EMAIL_PROVIDER", "dev")
		defer func() {
			os.Unsetenv("APP_ENV")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("JWT_SECRET")
			os.Unsetenv("ALLOWED_ORIGINS")
			os.Unsetenv("EMAIL_PROVIDER")
		}()

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "cannot be set to 'dev' in production") {
			t.Errorf("expected error for dev provider in production, got: %v", err)
		}
	})

	t.Run("Rejects unknown EMAIL_PROVIDER", func(t *testing.T) {
		os.Setenv("EMAIL_PROVIDER", "unknown_smtp_service")
		defer os.Unsetenv("EMAIL_PROVIDER")

		cfg := Load()
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "unsupported EMAIL_PROVIDER") {
			t.Errorf("expected error for unsupported EMAIL_PROVIDER, got: %v", err)
		}
	})
}


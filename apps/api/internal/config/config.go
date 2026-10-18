package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultDevDatabaseURL = "postgres://postgres@127.0.0.1:5432/ratels_dev?sslmode=disable"
	DefaultDevJWTSecret   = "ratels-secret-key-dev-only-change-in-production-min-32-chars"
	MinJWTSecretLength    = 32
)

type Config struct {
	DatabaseURL               string
	Port                      string
	JWTSecret                 string
	JWTExpiryHours            int
	Environment               string
	DevSafeDelivery           bool
	FoundationProvisionalName string
	AllowedOrigins            []string
}

func Load() *Config {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	isProduction := strings.ToLower(env) == "production"

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" && !isProduction {
		dbURL = DefaultDevDatabaseURL
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" && !isProduction {
		jwtSecret = DefaultDevJWTSecret
	}

	jwtExpiryHours := 24
	if expStr := os.Getenv("JWT_EXPIRY_HOURS"); expStr != "" {
		if val, err := strconv.Atoi(expStr); err == nil && val > 0 {
			jwtExpiryHours = val
		}
	}

	devDelivery := true
	if isProduction || os.Getenv("DEV_DELIVERY_SAFE_MODE") == "false" {
		devDelivery = false
	}

	foundationName := os.Getenv("FOUNDATION_PROVISIONAL_NAME")
	if foundationName == "" {
		foundationName = "Martins Vincent Otse Foundation"
	}

	// AllowedOrigins: In production, allow ONLY explicitly configured frontend origins.
	// In development/test, default to local dev server origins.
	var allowedOrigins []string
	if isProduction {
		if rawOrigins := os.Getenv("ALLOWED_ORIGINS"); rawOrigins != "" {
			for _, o := range strings.Split(rawOrigins, ",") {
				trimmed := strings.TrimSpace(o)
				if trimmed != "" {
					allowedOrigins = append(allowedOrigins, trimmed)
				}
			}
		}
	} else {
		allowedOrigins = []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:8000",
			"http://127.0.0.1:8000",
		}
		if rawOrigins := os.Getenv("ALLOWED_ORIGINS"); rawOrigins != "" {
			for _, o := range strings.Split(rawOrigins, ",") {
				trimmed := strings.TrimSpace(o)
				if trimmed != "" {
					allowedOrigins = append(allowedOrigins, trimmed)
				}
			}
		}
	}

	return &Config{
		DatabaseURL:               dbURL,
		Port:                      port,
		JWTSecret:                 jwtSecret,
		JWTExpiryHours:            jwtExpiryHours,
		Environment:               env,
		DevSafeDelivery:           devDelivery,
		FoundationProvisionalName: foundationName,
		AllowedOrigins:            allowedOrigins,
	}
}

// Validate verifies production requirements and safety invariants.
func (c *Config) Validate() error {
	isProduction := strings.ToLower(c.Environment) == "production"
	if isProduction {
		if strings.TrimSpace(c.DatabaseURL) == "" {
			return errors.New("DATABASE_URL is required in production environment; no connection string provided")
		}
		if c.DatabaseURL == DefaultDevDatabaseURL {
			return errors.New("DATABASE_URL cannot use the development fallback database in production")
		}
		if strings.TrimSpace(c.JWTSecret) == "" {
			return errors.New("JWT_SECRET is required in production environment; no secret key provided")
		}
		if c.JWTSecret == DefaultDevJWTSecret {
			return errors.New("JWT_SECRET cannot use the development fallback key in production")
		}
		if len(c.JWTSecret) < MinJWTSecretLength {
			return fmt.Errorf("JWT_SECRET is too short (%d characters); minimum requirement is %d characters in production", len(c.JWTSecret), MinJWTSecretLength)
		}
		if len(c.AllowedOrigins) == 0 {
			return errors.New("ALLOWED_ORIGINS is required in production environment; specify explicit frontend origin(s)")
		}
		for _, o := range c.AllowedOrigins {
			if strings.Contains(o, "*") {
				return fmt.Errorf("wildcard origin %q is prohibited in production; specify explicit frontend origins", o)
			}
		}
	}
	return nil
}

// LoadAndValidate loads configuration and verifies all required invariants.
func LoadAndValidate() (*Config, error) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

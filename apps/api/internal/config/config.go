package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL               string
	Port                      string
	JWTSecret                 string
	JWTExpiryHours            int
	Environment               string
	DevSafeDelivery           bool
	FoundationProvisionalName string
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres@127.0.0.1:5432/ratels_dev?sslmode=disable"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "ratels-secret-key-dev-only-change-in-production-min-32-chars"
	}

	jwtExpiryHours := 24
	if expStr := os.Getenv("JWT_EXPIRY_HOURS"); expStr != "" {
		if val, err := strconv.Atoi(expStr); err == nil && val > 0 {
			jwtExpiryHours = val
		}
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	devDelivery := true
	if os.Getenv("DEV_DELIVERY_SAFE_MODE") == "false" {
		devDelivery = false
	}

	foundationName := os.Getenv("FOUNDATION_PROVISIONAL_NAME")
	if foundationName == "" {
		foundationName = "Martins Vincent Otse Foundation"
	}

	return &Config{
		DatabaseURL:               dbURL,
		Port:                      port,
		JWTSecret:                 jwtSecret,
		JWTExpiryHours:            jwtExpiryHours,
		Environment:               env,
		DevSafeDelivery:           devDelivery,
		FoundationProvisionalName: foundationName,
	}
}

// Command migrate applies pending embedded migrations before standalone seeding.
package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
	"os/signal"
	"ratels-api/internal/config"
	"ratels-api/internal/migrator"
	"ratels-api/internal/security"
	"syscall"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database configuration failed: %v", security.RedactError(err, cfg.DatabaseURL))
	}
	defer pool.Close()
	if err := migrator.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("Migration failed: %v", security.RedactError(err, cfg.DatabaseURL))
	}
	log.Println("Database migrations complete.")
}

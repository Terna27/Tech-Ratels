package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/config"
	"ratels-api/internal/delivery"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/worker"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Starting Standalone Ratels Notification Outbox & Communications Worker...")

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Invalid database connection configuration: %v\n", security.RedactError(err, cfg.DatabaseURL))
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", security.RedactError(err, cfg.DatabaseURL))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Database ping failed: %v\n", security.RedactError(err, cfg.DatabaseURL))
	}

	repo := repository.NewRepository(pool)
	auditService := service.NewAuditService(repo)
	isProduction := strings.ToLower(cfg.Environment) == "production"

	var emailSender delivery.EmailSender
	if cfg.BrevoAPIKey != "" && cfg.EmailFromAddress != "" {
		emailSender = delivery.NewBrevoDeliveryAdapter(cfg.BrevoAPIKey, cfg.EmailFromName, cfg.EmailFromAddress)
	} else if !isProduction && cfg.DevSafeDelivery {
		emailSender = delivery.NewDevSafeEmailSender()
	} else {
		emailSender = delivery.NewUnconfiguredEmailSender()
	}

	var smsSender delivery.SMSSender
	if cfg.TermiiAPIKey != "" {
		smsSender = delivery.NewTermiiDeliveryAdapter(cfg.TermiiAPIKey, cfg.TermiiSenderID)
	} else if !isProduction && cfg.DevSafeDelivery {
		smsSender = delivery.NewDevSafeSMSSender()
	} else {
		smsSender = delivery.NewUnconfiguredSMSSender()
	}

	notifService := service.NewNotificationService(repo, auditService, emailSender, smsSender, cfg.FrontendBaseURL)
	reportService := service.NewReportService(repo, notifService, auditService)

	outboxWorker := worker.NewOutboxWorker(repo, notifService, emailSender, smsSender, nil)
	outboxWorker.Start(ctx)

	reportScheduler := worker.NewReportScheduler(repo, reportService, nil)
	reportScheduler.Start(ctx)

	log.Println("Standalone Notification Outbox & Reports Scheduler Worker running. Waiting for events...")
	<-ctx.Done()

	log.Println("Shutting down workers gracefully...")
	reportScheduler.Stop()
	outboxWorker.Stop()
	log.Println("Worker terminated.")
}

package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/config"
	"ratels-api/internal/delivery"
	"ratels-api/internal/handler"
	"ratels-api/internal/repository"
	"ratels-api/internal/service"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Connecting to PostgreSQL at: %s ...\n", cfg.DatabaseURL)
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Invalid DATABASE_URL: %v\n", err)
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}
	log.Println("PostgreSQL connection established successfully.")

	repo := repository.NewRepository(pool)
	isProduction := strings.ToLower(cfg.Environment) == "production" || strings.ToLower(os.Getenv("APP_ENV")) == "production"
	devDelivery := delivery.NewDevSafeDeliveryAdapter(isProduction)

	// Services
	auditService := service.NewAuditService(repo)
	geoService := service.NewGeographyService(repo)
	authService := service.NewAuthService(repo, cfg, devDelivery, auditService)
	leadService := service.NewLeadershipService(repo, auditService)
	cleanupService := service.NewCleanupService(repo, auditService)
	partService := service.NewParticipationService(repo, auditService)
	foundService := service.NewFoundationService(repo, cfg, auditService)
	adminService := service.NewAdminService(repo)
	financeService := service.NewFinanceService(repo, auditService)

	// Handlers
	healthH := handler.NewHealthHandler(repo)
	authH := handler.NewAuthHandler(authService, cfg)
	memberH := handler.NewMemberHandler(authService)
	geoH := handler.NewGeographyHandler(geoService)
	leadH := handler.NewLeadershipHandler(leadService)
	activityH := handler.NewActivityHandler(cleanupService)
	partH := handler.NewParticipationHandler(partService)
	foundH := handler.NewFoundationHandler(foundService)
	adminH := handler.NewAdminHandler(adminService)
	financeH := handler.NewFinanceHandler(financeService)

	router := handler.NewRouter(handler.RouterParams{
		Config:               cfg,
		Repo:                 repo,
		HealthHandler:        healthH,
		AuthHandler:          authH,
		MemberHandler:        memberH,
		GeographyHandler:     geoH,
		LeadershipHandler:    leadH,
		ActivityHandler:      activityH,
		ParticipationHandler: partH,
		FoundationHandler:    foundH,
		AdminHandler:         adminH,
		FinanceHandler:       financeH,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Ratels Management System Go API server listening on 0.0.0.0:%s (Environment: %s) ...\n", cfg.Port, cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v\n", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down API server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v\n", err)
	}

	log.Println("API server stopped.")
}

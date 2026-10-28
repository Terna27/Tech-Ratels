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
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/handler"
	"ratels-api/internal/migrator"
	"ratels-api/internal/payment"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/seeder"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
	"ratels-api/internal/worker"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal configuration error: %v\n", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Connecting to PostgreSQL...")
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Invalid database connection configuration: %v\n", security.RedactError(err, cfg.DatabaseURL))
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
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
	log.Println("PostgreSQL connection established successfully.")

	// Execute any unapplied database migrations
	if err := migrator.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("Database migration failed: %v\n", err)
	}

	// Ensure authoritative reference geographic and institutional data is populated
	if err := seeder.SeedReferenceData(ctx, pool, db.New(pool)); err != nil {
		log.Fatalf("Reference data seeding failed: %v\n", err)
	}

	repo := repository.NewRepository(pool)
	isProduction := strings.ToLower(cfg.Environment) == "production" || strings.ToLower(os.Getenv("APP_ENV")) == "production"
	
	var emailDelivery delivery.DeliveryAdapter
	switch cfg.EmailProvider {
	case "brevo":
		if cfg.BrevoAPIKey != "" && cfg.EmailFromAddress != "" {
			emailDelivery = delivery.NewBrevoDeliveryAdapter(cfg.BrevoAPIKey, cfg.EmailFromName, cfg.EmailFromAddress)
			log.Printf("Email delivery adapter configured via Brevo HTTPS API (sender: %s <%s>)\n", cfg.EmailFromName, cfg.EmailFromAddress)
		} else if !isProduction && cfg.DevSafeDelivery {
			emailDelivery = delivery.NewDevSafeDeliveryAdapter(false)
			log.Println("Brevo credentials missing; falling back to development safe sandbox mode.")
		} else {
			emailDelivery = delivery.NewProductionUnconfiguredAdapter()
			log.Println("WARNING: Brevo credentials (BREVO_API_KEY or EMAIL_FROM_ADDRESS) missing; verification emails will fail.")
		}
	case "resend":
		if cfg.ResendAPIKey != "" {
			emailDelivery = delivery.NewResendDeliveryAdapter(cfg.ResendAPIKey, cfg.EmailFrom)
			log.Printf("Email delivery adapter configured via Resend HTTPS API (sender: %s)\n", cfg.EmailFrom)
		} else if !isProduction && cfg.DevSafeDelivery {
			emailDelivery = delivery.NewDevSafeDeliveryAdapter(false)
			log.Println("Resend credentials missing; falling back to development safe sandbox mode.")
		} else {
			emailDelivery = delivery.NewProductionUnconfiguredAdapter()
			log.Println("WARNING: Resend credentials (RESEND_API_KEY) missing; verification emails will fail.")
		}
	case "dev":
		emailDelivery = delivery.NewDevSafeDeliveryAdapter(false)
		log.Println("Email delivery running in development safe sandbox mode (no external dispatch).")
	default:
		if !isProduction && cfg.DevSafeDelivery {
			emailDelivery = delivery.NewDevSafeDeliveryAdapter(false)
			log.Println("Email delivery running in development safe sandbox mode (no external dispatch).")
		} else {
			emailDelivery = delivery.NewProductionUnconfiguredAdapter()
			log.Println("WARNING: No email delivery provider configured; external verification emails will be rejected.")
		}
	}

	// Services
	auditService := service.NewAuditService(repo)
	geoService := service.NewGeographyService(repo)
	authService := service.NewAuthService(repo, cfg, emailDelivery, auditService)
	leadService := service.NewLeadershipService(repo, auditService)
	cleanupService := service.NewCleanupService(repo, auditService)
	partService := service.NewParticipationService(repo, auditService)
	foundService := service.NewFoundationService(repo, cfg, auditService)
	adminService := service.NewAdminService(repo, auditService)
	inventoryService := service.NewInventoryService(repo, auditService)

	var paymentGateway payment.PaymentGateway
	if cfg.PaystackSecretKey != "" {
		paymentGateway = payment.NewPaystackGateway(cfg.PaystackSecretKey, cfg.PaystackPublicKey)
		log.Println("Marketplace payment gateway initialized via Paystack HTTPS API.")
	} else if !isProduction && (cfg.PaymentProvider == "sandbox" || cfg.PaymentProvider == "dev_sandbox") {
		paymentGateway = payment.NewSandboxGateway(cfg.FrontendBaseURL)
		log.Println("Marketplace payment running in development safe sandbox mode (no external charges).")
	} else {
		paymentGateway = payment.NewUnconfiguredGateway()
		log.Println("WARNING: Marketplace payment gateway is UNCONFIGURED. Checkout is safely disabled in this environment.")
	}

	var storageService storage.StorageService
	if cfg.R2AccessKeyID != "" && cfg.R2SecretAccessKey != "" && cfg.R2BucketName != "" {
		r2, err := storage.NewR2StorageService(ctx, storage.R2Config{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			BucketName:      cfg.R2BucketName,
			PublicDomain:    cfg.R2PublicDomain,
			Endpoint:        cfg.R2Endpoint,
		})
		if err != nil {
			log.Fatalf("Failed to initialize Cloudflare R2 storage client: %v\n", err)
		}
		storageService = r2
		log.Printf("Marketplace image storage initialized via Cloudflare R2 (bucket: %s, domain: %s)\n", cfg.R2BucketName, r2.GetPublicURL(""))
	} else if !isProduction {
		storageDomain := cfg.R2PublicDomain
		if storageDomain == "" {
			storageDomain = "https://pub-82990f82f02446b5a11817fdce96912d.r2.dev"
		}
		storageService = storage.NewDevSandboxStorageService(storageDomain)
		log.Println("Media storage running in development safe sandbox mode (in-memory mock).")
	} else {
		storageService = storage.NewUnconfiguredStorageService()
		log.Println("WARNING: Marketplace image storage is UNCONFIGURED in production. Image uploads will return 503 until R2 credentials are configured.")
	}

	financeService := service.NewFinanceService(repo, auditService, paymentGateway, storageService)
	marketplaceService := service.NewMarketplaceService(repo, auditService, paymentGateway, storageService)
	careersService := service.NewCareersService(repo, auditService, emailDelivery)
	heroMediaService := service.NewHeroMediaService(repo, auditService, storageService)
	communityProjectService := service.NewCommunityProjectService(repo, auditService, storageService, financeService)
	galleryService := service.NewGalleryService(repo, storageService, auditService)

	// Transactional Notifications & Outbox Worker Providers
	var emailSender delivery.EmailSender
	if cfg.BrevoAPIKey != "" && cfg.EmailFromAddress != "" {
		emailSender = delivery.NewBrevoDeliveryAdapter(cfg.BrevoAPIKey, cfg.EmailFromName, cfg.EmailFromAddress)
		log.Printf("Transactional email sender configured via Brevo (%s <%s>)\n", cfg.EmailFromName, cfg.EmailFromAddress)
	} else if !isProduction && cfg.DevSafeDelivery {
		emailSender = delivery.NewDevSafeEmailSender()
		log.Println("Transactional email running in development safe sandbox mode.")
	} else {
		emailSender = delivery.NewUnconfiguredEmailSender()
		log.Println("WARNING: Brevo credentials missing; transactional emails will fail safely.")
	}

	var smsSender delivery.SMSSender
	if cfg.TermiiAPIKey != "" {
		smsSender = delivery.NewTermiiDeliveryAdapter(cfg.TermiiAPIKey, cfg.TermiiSenderID)
		log.Printf("Transactional SMS sender configured via Termii (sender: %s)\n", cfg.TermiiSenderID)
	} else if !isProduction && cfg.DevSafeDelivery {
		smsSender = delivery.NewDevSafeSMSSender()
		log.Println("Transactional SMS running in development safe sandbox mode.")
	} else {
		smsSender = delivery.NewUnconfiguredSMSSender()
		log.Println("WARNING: Termii credentials missing; SMS notifications will fail safely.")
	}

	notifService := service.NewNotificationService(repo, auditService, emailSender, smsSender, cfg.FrontendBaseURL)
	cleanupService.SetNotificationService(notifService)
	leadService.SetNotificationService(notifService)

	// Launch Outbox Background Worker if enabled
	var outboxWorker *worker.OutboxWorker
	if cfg.EnableOutboxWorker {
		outboxWorker = worker.NewOutboxWorker(repo, notifService, emailSender, smsSender, nil)
		outboxWorker.Start(ctx)
		defer outboxWorker.Stop()
		log.Println("Transactional Notification Outbox & Cleanup Reminder worker started.")
	}

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
	inventoryH := handler.NewInventoryHandler(inventoryService)
	marketplaceH := handler.NewMarketplaceHandler(marketplaceService)
	marketplaceH.SetCronSecret(cfg.MarketplaceCronSecret)
	careersH := handler.NewCareersHandler(careersService)
	heroMediaH := handler.NewHeroMediaHandler(heroMediaService)
	communityProjectH := handler.NewCommunityProjectHandler(communityProjectService)
	galleryH := handler.NewGalleryHandler(galleryService)
	notifH := handler.NewNotificationHandler(notifService)

	// Safeguarding & Confidential Incident Reporting
	safeguardCipher, err := security.NewSafeguardingCipher(cfg.SafeguardingEncryptionKey, isProduction)
	if err != nil {
		log.Fatalf("Failed to initialize safeguarding cipher: %v\n", err)
	}
	safeguardStorage := storage.NewDevSafeguardingStorageService()
	safeguardService := service.NewSafeguardingService(repo, safeguardCipher, safeguardStorage, notifService, cfg)
	safeguardH := handler.NewSafeguardingHandler(safeguardService, safeguardStorage, cfg)

	// Transparency & Public Reports Subsystem
	var reportExportStorage storage.ReportExportStorage
	if cfg.R2ReportsAccessKeyID != "" && cfg.R2ReportsSecretAccessKey != "" && cfg.R2ReportsBucketName != "" {
		r2Exp, err := storage.NewR2ReportExportStorageFromConfig(ctx, storage.R2Config{
			AccountID:       cfg.R2ReportsAccountID,
			AccessKeyID:     cfg.R2ReportsAccessKeyID,
			SecretAccessKey: cfg.R2ReportsSecretAccessKey,
			BucketName:      cfg.R2ReportsBucketName,
			Endpoint:        cfg.R2ReportsEndpoint,
		})
		if err != nil {
			if isProduction {
				log.Fatalf("FATAL: Failed to initialize private Cloudflare R2 report export storage in production: %v\n", err)
			}
			log.Printf("WARNING: Failed to initialize private Cloudflare R2 report export storage: %v; falling back to dev sandbox\n", err)
			reportExportStorage = storage.NewDevSandboxReportExportStorage()
		} else {
			reportExportStorage = r2Exp
			log.Printf("Report export storage initialized via dedicated Cloudflare R2 credentials (private bucket: %s)\n", cfg.R2ReportsBucketName)
		}
	} else if !isProduction {
		reportExportStorage = storage.NewDevSandboxReportExportStorage()
		log.Println("Report export storage running in development safe sandbox mode (in-memory mock).")
	} else {
		reportExportStorage = storage.NewUnconfiguredReportExportStorage()
		log.Println("WARNING: Private report export storage is UNCONFIGURED in production. Document exports will return 503 until dedicated reports R2 credentials are configured.")
	}

	reportService := service.NewReportService(repo, notifService, auditService, reportExportStorage)
	reportScheduler := worker.NewReportScheduler(repo, reportService, nil)
	reportScheduler.Start(ctx)
	defer reportScheduler.Stop()

	refundWorker := worker.NewRefundWorker(marketplaceService, nil)
	refundWorker.Start(ctx)
	defer refundWorker.Stop()

	reportH := handler.NewReportHandler(reportService)
	reportH.SetScheduler(reportScheduler, cfg.ReportCronSecret)

	router := handler.NewRouter(handler.RouterParams{
		Config:                  cfg,
		Repo:                    repo,
		HealthHandler:           healthH,
		AuthHandler:             authH,
		MemberHandler:           memberH,
		GeographyHandler:        geoH,
		LeadershipHandler:       leadH,
		ActivityHandler:         activityH,
		ParticipationHandler:    partH,
		FoundationHandler:       foundH,
		AdminHandler:            adminH,
		FinanceHandler:          financeH,
		InventoryHandler:        inventoryH,
		MarketplaceHandler:      marketplaceH,
		CareersHandler:          careersH,
		HeroMediaHandler:        heroMediaH,
		CommunityProjectHandler: communityProjectH,
		GalleryHandler:          galleryH,
		NotificationHandler:     notifH,
		SafeguardingHandler:     safeguardH,
		ReportHandler:           reportH,
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

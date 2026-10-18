package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/config"
	"ratels-api/internal/delivery"
	"ratels-api/internal/repository"
	"ratels-api/internal/service"
)

type TestEnv struct {
	Pool         *pgxpool.Pool
	Repo         *repository.Repository
	Config       *config.Config
	Delivery     *delivery.DevSafeDeliveryAdapter
	AuditService *service.AuditService
	GeoService   *service.GeographyService
	AuthService  *service.AuthService
	LeadService  *service.LeadershipService
	CleanupService *service.CleanupService
	PartService  *service.ParticipationService
	FoundService *service.FoundationService
	AdminService *service.AdminService
	FinanceService *service.FinanceService
}

func setupTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	repo := repository.NewRepository(pool)
	cfg := config.Load()
	cfg.DatabaseURL = testDBURL
	del := delivery.NewDevSafeDeliveryAdapter(false)

	auditSvc := service.NewAuditService(repo)
	geoSvc := service.NewGeographyService(repo)
	authSvc := service.NewAuthService(repo, cfg, del, auditSvc)
	leadSvc := service.NewLeadershipService(repo, auditSvc)
	cleanupSvc := service.NewCleanupService(repo, auditSvc)
	partSvc := service.NewParticipationService(repo, auditSvc)
	foundSvc := service.NewFoundationService(repo, cfg, auditSvc)
	adminSvc := service.NewAdminService(repo)
	finSvc := service.NewFinanceService(repo, auditSvc)

	return &TestEnv{
		Pool:           pool,
		Repo:           repo,
		Config:         cfg,
		Delivery:       del,
		AuditService:   auditSvc,
		GeoService:     geoSvc,
		AuthService:    authSvc,
		LeadService:    leadSvc,
		CleanupService: cleanupSvc,
		PartService:    partSvc,
		FoundService:   foundSvc,
		AdminService:   adminSvc,
		FinanceService: finSvc,
	}
}

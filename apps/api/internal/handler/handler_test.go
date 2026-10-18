package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/config"
	"ratels-api/internal/delivery"
	"ratels-api/internal/handler"
	"ratels-api/internal/repository"
	"ratels-api/internal/service"
)

type TestHarness struct {
	Router         http.Handler
	Pool           *pgxpool.Pool
	Repo           *repository.Repository
	Config         *config.Config
	Delivery       *delivery.DevSafeDeliveryAdapter
	AuthService    *service.AuthService
	LeadService    *service.LeadershipService
	CleanupService *service.CleanupService
	PartService    *service.ParticipationService
	FoundService   *service.FoundationService
	GeoService     *service.GeographyService
	AdminService   *service.AdminService
	FinanceService *service.FinanceService
	Cleanup        func()
}

func setupTestHarness(t *testing.T) *TestHarness {
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
	financeSvc := service.NewFinanceService(repo, auditSvc)

	healthH := handler.NewHealthHandler(repo)
	authH := handler.NewAuthHandler(authSvc, cfg)
	memberH := handler.NewMemberHandler(authSvc)
	geoH := handler.NewGeographyHandler(geoSvc)
	leadH := handler.NewLeadershipHandler(leadSvc)
	activityH := handler.NewActivityHandler(cleanupSvc)
	partH := handler.NewParticipationHandler(partSvc)
	foundH := handler.NewFoundationHandler(foundSvc)
	adminH := handler.NewAdminHandler(adminSvc)
	financeH := handler.NewFinanceHandler(financeSvc)

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

	return &TestHarness{
		Router:         router,
		Pool:           pool,
		Repo:           repo,
		Config:         cfg,
		Delivery:       del,
		AuthService:    authSvc,
		LeadService:    leadSvc,
		CleanupService: cleanupSvc,
		PartService:    partSvc,
		FoundService:   foundSvc,
		GeoService:     geoSvc,
		AdminService:   adminSvc,
		FinanceService: financeSvc,
		Cleanup: func() {
			pool.Close()
		},
	}
}

func setupTestRouter(t *testing.T) (http.Handler, *delivery.DevSafeDeliveryAdapter, func()) {
	h := setupTestHarness(t)
	return h.Router, h.Delivery, h.Cleanup
}

func TestHealthEndpoint(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["status"] != "ok" || body["database"] != "healthy" {
		t.Errorf("Unexpected health response: %v", body)
	}
}

func TestHTTPAuthFlow(t *testing.T) {
	router, del, cleanup := setupTestRouter(t)
	defer cleanup()

	// 1. Register via HTTP
	userEmail := fmt.Sprintf("http_member_%d@example.com", time.Now().UnixNano())
	regPayload := map[string]string{
		"email":        userEmail,
		"password":     "HttpPassword123!",
		"display_name": "HTTP Volunteer",
	}
	regBytes, _ := json.Marshal(regPayload)
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(regBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Register HTTP failed: code %d, body: %s", rr.Code, rr.Body.String())
	}

	var regResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &regResp)
	userID := regResp["user_id"].(string)

	// 2. Verify Contact via HTTP
	otpCode := del.GetLastCode(userEmail)
	verifyPayload := map[string]string{
		"user_id":    userID,
		"identifier": userEmail,
		"code":       otpCode,
	}
	verBytes, _ := json.Marshal(verifyPayload)
	req, _ = http.NewRequest("POST", "/api/v1/auth/verify-contact", bytes.NewBuffer(verBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Verify Contact HTTP failed: code %d, body: %s", rr.Code, rr.Body.String())
	}

	// 3. Login via HTTP
	loginPayload := map[string]string{
		"username": userEmail,
		"password": "HttpPassword123!",
	}
	loginBytes, _ := json.Marshal(loginPayload)
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Login HTTP failed: code %d, body: %s", rr.Code, rr.Body.String())
	}

	var loginResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// 4. Authenticated Request: GET /api/v1/members/me
	req, _ = http.NewRequest("GET", "/api/v1/members/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GetMe HTTP failed: code %d, body: %s", rr.Code, rr.Body.String())
	}

	// 5. Unauthenticated request to protected endpoint: GET /api/v1/members/me without token -> 401
	req, _ = http.NewRequest("GET", "/api/v1/members/me", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", rr.Code)
	}
}

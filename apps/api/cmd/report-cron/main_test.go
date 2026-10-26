package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Helper to capture log output
func captureLog(f func()) string {
	var buf bytes.Buffer
	origOutput := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
	}()
	f()
	return buf.String()
}

// 1. Successful completed cycle
func TestReportCron_SuccessfulCompletedCycle(t *testing.T) {
	expectedSecret := "secret_report_cron_token_production_xyz123"
	called := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/internal/cron/reports" {
			t.Errorf("expected path /api/v1/internal/cron/reports, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("period_type") != "monthly" {
			t.Errorf("expected period_type=monthly, got %q", r.URL.Query().Get("period_type"))
		}
		if r.Header.Get("X-Cron-Secret") != expectedSecret {
			t.Errorf("expected X-Cron-Secret=%q, got %q", expectedSecret, r.Header.Get("X-Cron-Secret"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SchedulerResponse{
			Status:           "completed",
			ReportsGenerated: 6,
			ReportsSkipped:   0,
			ReportsFailed:    0,
			Errors:           []string{},
		})
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     expectedSecret,
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	output := captureLog(func() {
		err := Run(context.Background(), cfg)
		if err != nil {
			t.Fatalf("expected nil error on successful completed cycle, got: %v", err)
		}
	})

	if !called {
		t.Fatal("expected mock server to be called")
	}
	if !strings.Contains(output, "Reporting completed successfully: generated=6, skipped=0, failed=0") {
		t.Errorf("unexpected log output: %s", output)
	}
	if strings.Contains(output, expectedSecret) {
		t.Errorf("secret leaked in log output: %s", output)
	}
}

// 2. Completed cycle with skipped duplicates
func TestReportCron_CompletedCycleWithSkippedDuplicates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SchedulerResponse{
			Status:           "completed",
			ReportsGenerated: 0,
			ReportsSkipped:   6,
			ReportsFailed:    0,
			Errors:           []string{},
		})
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     "valid_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected nil error when cycle completes with skipped duplicates, got: %v", err)
	}
}

// 3. Missing secret
func TestReportCron_MissingSecret(t *testing.T) {
	// 3a. Via LoadConfigFromEnv
	_ = os.Setenv("REPORT_CRON_SECRET", "")
	_ = os.Setenv("RATELS_API_URL", "https://ratels.onrender.com")
	defer func() {
		_ = os.Unsetenv("REPORT_CRON_SECRET")
		_ = os.Unsetenv("RATELS_API_URL")
	}()

	cfg, err := LoadConfigFromEnv()
	if err == nil || cfg != nil {
		t.Fatal("expected error from LoadConfigFromEnv when REPORT_CRON_SECRET is missing")
	}
	if !strings.Contains(err.Error(), "REPORT_CRON_SECRET is missing") {
		t.Errorf("unexpected error message: %v", err)
	}

	// 3b. Via Run()
	err = Run(context.Background(), &Config{
		Secret:  "",
		BaseURL: "https://ratels.onrender.com",
	})
	if err == nil {
		t.Fatal("expected error from Run() when Secret is empty")
	}
	if !strings.Contains(err.Error(), "REPORT_CRON_SECRET is missing") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 4. Missing API URL
func TestReportCron_MissingAPIURL(t *testing.T) {
	// 4a. Via LoadConfigFromEnv
	_ = os.Setenv("REPORT_CRON_SECRET", "super_secret")
	_ = os.Setenv("RATELS_API_URL", "")
	defer func() {
		_ = os.Unsetenv("REPORT_CRON_SECRET")
		_ = os.Unsetenv("RATELS_API_URL")
	}()

	cfg, err := LoadConfigFromEnv()
	if err == nil || cfg != nil {
		t.Fatal("expected error from LoadConfigFromEnv when RATELS_API_URL is missing")
	}
	if !strings.Contains(err.Error(), "RATELS_API_URL is missing") {
		t.Errorf("unexpected error message: %v", err)
	}

	// 4b. Via Run()
	err = Run(context.Background(), &Config{
		Secret:  "super_secret",
		BaseURL: "",
	})
	if err == nil {
		t.Fatal("expected error from Run() when BaseURL is empty")
	}
	if !strings.Contains(err.Error(), "RATELS_API_URL is missing") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 5. HTTP 401 Unauthorized
func TestReportCron_HTTP401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"UNAUTHORIZED","message":"Invalid or missing cron secret"}`))
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     "wrong_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on HTTP 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 401 Unauthorized error, got: %v", err)
	}
}

// 6. HTTP 409 Lock Conflict
func TestReportCron_HTTP409(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"LOCK_CONFLICT","message":"Scheduler cycle already running on another node"}`))
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     "valid_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on HTTP 409, got nil")
	}
	if !strings.Contains(err.Error(), "409") && !strings.Contains(err.Error(), "Conflict") {
		t.Errorf("expected 409 Conflict error, got: %v", err)
	}
}

// 7. HTTP 500 Internal Server Error
func TestReportCron_HTTP500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"INTERNAL_ERROR","message":"Database connection failed"}`))
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     "valid_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 status in error, got: %v", err)
	}
}

// 8. Partial failure
func TestReportCron_PartialFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SchedulerResponse{
			Status:           "partial_failure",
			ReportsGenerated: 4,
			ReportsSkipped:   0,
			ReportsFailed:    2,
			Errors:           []string{"failed generating finance report", "failed generating foundation report"},
		})
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     "valid_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on partial failure status, got nil")
	}
	if !strings.Contains(err.Error(), "partial_failure") {
		t.Errorf("expected error to mention partial_failure, got: %v", err)
	}
}

// 9. Failed scheduler response
func TestReportCron_FailedSchedulerResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SchedulerResponse{
			Status:           "failed",
			ReportsGenerated: 0,
			ReportsSkipped:   0,
			ReportsFailed:    6,
			Errors:           []string{"database transaction rolled back"},
		})
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     "valid_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on failed status, got nil")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected error to mention failed, got: %v", err)
	}
}

// 10. Malformed JSON response
func TestReportCron_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json at all`))
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     "valid_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("expected error to mention JSON, got: %v", err)
	}
}

// 11. Request timeout
func TestReportCron_RequestTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Short timeout that will be exceeded
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	cfg := &Config{
		Secret:     "valid_secret",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    20 * time.Millisecond,
	}

	err := Run(ctx, cfg)
	if err == nil {
		t.Fatal("expected error on request timeout, got nil")
	}
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "canceled") {
		t.Errorf("expected timeout/deadline/canceled in error, got: %v", err)
	}
}

// 12. Non-HTTPS production URL rejection
func TestReportCron_ProductionRequiresHTTPS(t *testing.T) {
	_ = os.Setenv("REPORT_CRON_SECRET", "super_secret_production")
	_ = os.Setenv("RATELS_API_URL", "http://ratels.onrender.com")
	_ = os.Setenv("APP_ENV", "production")
	defer func() {
		_ = os.Unsetenv("REPORT_CRON_SECRET")
		_ = os.Unsetenv("RATELS_API_URL")
		_ = os.Unsetenv("APP_ENV")
	}()

	cfg, err := LoadConfigFromEnv()
	if err == nil || cfg != nil {
		t.Fatal("expected error when production RATELS_API_URL uses HTTP instead of HTTPS")
	}
	if !strings.Contains(err.Error(), "must use HTTPS in production") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 13. Remote HTTP URL rejection even when not production
func TestReportCron_RemoteURLRequiresHTTPS(t *testing.T) {
	_ = os.Setenv("REPORT_CRON_SECRET", "super_secret_dev")
	_ = os.Setenv("RATELS_API_URL", "http://api.ratels.org")
	_ = os.Setenv("APP_ENV", "development")
	defer func() {
		_ = os.Unsetenv("REPORT_CRON_SECRET")
		_ = os.Unsetenv("RATELS_API_URL")
		_ = os.Unsetenv("APP_ENV")
	}()

	cfg, err := LoadConfigFromEnv()
	if err == nil || cfg != nil {
		t.Fatal("expected error when remote RATELS_API_URL uses HTTP")
	}
	if !strings.Contains(err.Error(), "must use HTTPS for remote hosts") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 14. Never log secrets or authentication headers
func TestReportCron_NeverLogSecrets(t *testing.T) {
	secretValue := "super_secret_token_1234567890"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"UNAUTHORIZED"}`))
	}))
	defer ts.Close()

	cfg := &Config{
		Secret:     secretValue,
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		Timeout:    5 * time.Second,
	}

	logOutput := captureLog(func() {
		err := Run(context.Background(), cfg)
		if err == nil {
			t.Fatal("expected error on 401")
		}
		if strings.Contains(err.Error(), secretValue) {
			t.Errorf("secret leaked in error message: %v", err)
		}
	})

	if strings.Contains(logOutput, secretValue) {
		t.Errorf("secret leaked in log output: %s", logOutput)
	}
}

// 15. run() helper with environment variables
func TestReportCron_RunFunctionIntegration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SchedulerResponse{
			Status:           "completed",
			ReportsGenerated: 6,
			ReportsSkipped:   0,
			ReportsFailed:    0,
			Errors:           []string{},
		})
	}))
	defer ts.Close()

	_ = os.Setenv("REPORT_CRON_SECRET", "integration_secret")
	_ = os.Setenv("RATELS_API_URL", ts.URL)
	_ = os.Setenv("APP_ENV", "test")
	defer func() {
		_ = os.Unsetenv("REPORT_CRON_SECRET")
		_ = os.Unsetenv("RATELS_API_URL")
		_ = os.Unsetenv("APP_ENV")
	}()

	err := run(context.Background())
	if err != nil {
		t.Fatalf("run(ctx) failed: %v", err)
	}
}

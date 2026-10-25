package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ratels-api/internal/worker"
)

func TestReportScheduler_AdminTriggerAuthorization(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, memberToken := createHandlerTestUser(t, h, "member", fmt.Sprintf("sched_member_%d@test.com", uid))
	_, adminToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("sched_admin_%d@test.com", uid))

	targetPeriod := fmt.Sprintf("204%d-%02d", uid%4, (uid%12)+1)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", targetPeriod)
	}()

	body, _ := json.Marshal(map[string]string{
		"target_period": targetPeriod,
	})

	// 1. Unauthenticated request -> HTTP 401
	req, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated trigger, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. Member request (unauthorized role) -> HTTP 403
	req, _ = http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member trigger, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Invalid target period format -> HTTP 400
	invalidBody, _ := json.Marshal(map[string]string{
		"target_period": "invalid-month-date",
	})
	req, _ = http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(invalidBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed target_period, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. Authorized National Admin -> HTTP 200
	req, _ = http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for national admin trigger, got %d: %s", rr.Code, rr.Body.String())
	}

	var res worker.SchedulerRunResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling run result: %v", err)
	}
	if !res.LockAcquired {
		t.Fatalf("expected lock to be acquired")
	}
	if res.ReportsGenerated != 6 {
		t.Errorf("expected 6 generated reports, got %d", res.ReportsGenerated)
	}
}

func TestReportScheduler_CronEndpointSecurity(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	targetPeriod := fmt.Sprintf("205%d-%02d", uid%4, (uid%12)+1)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", targetPeriod)
	}()

	body, _ := json.Marshal(map[string]string{
		"target_period": targetPeriod,
	})

	// 1. Missing secret header -> HTTP 401
	req, _ := http.NewRequest("POST", "/api/v1/internal/cron/reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing cron secret, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. Incorrect secret header -> HTTP 401
	req, _ = http.NewRequest("POST", "/api/v1/internal/cron/reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cron-Secret", "wrong_invalid_secret")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid cron secret, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Valid X-Cron-Secret header -> HTTP 200
	req, _ = http.NewRequest("POST", "/api/v1/internal/cron/reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cron-Secret", "test_cron_secret_32_chars_long_123")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid cron secret, got %d: %s", rr.Code, rr.Body.String())
	}

	var res worker.SchedulerRunResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling run result: %v", err)
	}
	if !res.LockAcquired {
		t.Fatalf("expected lock to be acquired via cron")
	}
	if res.ReportsGenerated != 6 {
		t.Errorf("expected 6 generated reports via cron, got %d", res.ReportsGenerated)
	}

	// 4. Repeated call with Bearer auth -> Idempotent skip HTTP 200
	req, _ = http.NewRequest("POST", "/api/v1/internal/cron/reports?target_period="+targetPeriod, nil)
	req.Header.Set("Authorization", "Bearer test_cron_secret_32_chars_long_123")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for second cron trigger, got %d: %s", rr.Code, rr.Body.String())
	}

	var res2 worker.SchedulerRunResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res2); err != nil {
		t.Fatalf("failed unmarshaling run result: %v", err)
	}
	if res2.ReportsGenerated != 0 {
		t.Errorf("expected 0 reports generated on repeat (idempotent), got %d", res2.ReportsGenerated)
	}
	if res2.ReportsSkipped != 6 {
		t.Errorf("expected 6 reports skipped on repeat (idempotent), got %d", res2.ReportsSkipped)
	}
}

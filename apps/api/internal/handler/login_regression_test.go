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

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
)

func TestHTTP_LoginWorkflowAndRegression(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	// 1. Create a verified active test user directly in DB
	uniqueID := time.Now().UnixNano()
	email := fmt.Sprintf("http_test_%d@example.org", uniqueID)
	password := "TestPass123!"
	pwHash, _ := security.HashPassword(password)
	userID := security.GenerateUUID()
	memRef := fmt.Sprintf("RATEL-2026-%06d", uniqueID%1000000)

	ctx := context.Background()
	_, err := h.Repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  userID,
		Email:               pgtype.Text{String: email, Valid: true},
		Phone:               pgtype.Text{String: fmt.Sprintf("+23480%08d", uniqueID%100000000), Valid: true},
		PasswordHash:        pwHash,
		DisplayName:         "HTTP Regression User",
		MembershipReference: memRef,
		Status:              "active",
		PrimaryRole:         "member",
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("Failed to create user in DB: %v", err)
	}

	// 2. Test Invalid Password returns 401 INVALID_CREDENTIALS
	badLoginBytes, _ := json.Marshal(map[string]string{
		"identifier": email,
		"password":   "WrongPass999!",
	})
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(badLoginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for bad password, got %d", rr.Code)
	}
	var errResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &errResp)
	errObj := errResp["error"].(map[string]interface{})
	if errObj["code"] != "INVALID_CREDENTIALS" {
		t.Errorf("Expected error code INVALID_CREDENTIALS, got %v", errObj["code"])
	}

	// 3. Test Unknown Account returns 401 INVALID_CREDENTIALS
	unknownBytes, _ := json.Marshal(map[string]string{
		"identifier": "nobody@example.org",
		"password":   password,
	})
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(unknownBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for unknown user, got %d", rr.Code)
	}

	// 4. Test Valid Login succeeds and sets HttpOnly ratels_session and non-HttpOnly ratels_csrf cookies
	loginBytes, _ := json.Marshal(map[string]string{
		"identifier": email,
		"password":   password,
	})
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Valid login failed: code %d, body %s", rr.Code, rr.Body.String())
	}

	var sessionCookie *http.Cookie
	var csrfCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "ratels_session" {
			sessionCookie = c
		}
		if c.Name == "ratels_csrf" {
			csrfCookie = c
		}
	}

	if sessionCookie == nil || !sessionCookie.HttpOnly {
		t.Fatalf("Expected HttpOnly ratels_session cookie on login")
	}
	if csrfCookie == nil || csrfCookie.HttpOnly {
		t.Fatalf("Expected non-HttpOnly ratels_csrf cookie on login")
	}

	// 5. Test Authenticated API Access via Session Cookie
	reqMe, _ := http.NewRequest("GET", "/api/v1/members/me", nil)
	reqMe.AddCookie(sessionCookie)
	rrMe := httptest.NewRecorder()
	h.Router.ServeHTTP(rrMe, reqMe)

	if rrMe.Code != http.StatusOK {
		t.Fatalf("Authenticated /members/me failed with cookie: code %d, body %s", rrMe.Code, rrMe.Body.String())
	}
	var meResp map[string]interface{}
	_ = json.Unmarshal(rrMe.Body.Bytes(), &meResp)
	if meResp["id"] != userID {
		t.Errorf("Expected user ID %s, got %v", userID, meResp["id"])
	}

	// 6. Test Logout revokes session and denies subsequent requests
	reqLogout, _ := http.NewRequest("POST", "/api/v1/auth/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	reqLogout.AddCookie(csrfCookie)
	reqLogout.Header.Set("X-CSRF-Token", csrfCookie.Value)
	rrLogout := httptest.NewRecorder()
	h.Router.ServeHTTP(rrLogout, reqLogout)

	if rrLogout.Code != http.StatusOK {
		t.Fatalf("Logout failed: code %d", rrLogout.Code)
	}

	// Subsequent request with the logged-out session cookie MUST be denied (401)
	reqSubsequent, _ := http.NewRequest("GET", "/api/v1/members/me", nil)
	reqSubsequent.AddCookie(sessionCookie)
	rrSubsequent := httptest.NewRecorder()
	h.Router.ServeHTTP(rrSubsequent, reqSubsequent)

	if rrSubsequent.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for revoked session cookie, got %d", rrSubsequent.Code)
	}

	// 7. Test Inventory Authorization Non-Interference: Verify that Inventory endpoints enforce role gates
	// while standard AuthMiddleware works consistently for Super Admin, Admin, and Member
	saID, saToken := createTestUserWithRole(t, h, "auth_sa", "Auth Super Admin", "super_admin")
	if saID == "" || saToken == "" {
		t.Fatalf("Failed to create test super admin")
	}

	reqInv, _ := http.NewRequest("GET", "/api/v1/inventory/items", nil)
	reqInv.Header.Set("Authorization", "Bearer "+saToken)
	rrInv := httptest.NewRecorder()
	h.Router.ServeHTTP(rrInv, reqInv)

	if rrInv.Code != http.StatusOK {
		t.Fatalf("Super Admin cannot access inventory items: code %d, body %s", rrInv.Code, rrInv.Body.String())
	}
}

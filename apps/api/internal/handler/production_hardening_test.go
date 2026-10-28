package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"testing"
	"time"
)

func TestAuthenticationRejectsSessionOwnedByAnotherUser(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	owner, token := createTestUserWithRole(t, h, "session_owner69", "Owner", "member")
	other, _ := createTestUserWithRole(t, h, "session_other69", "Other", "member")
	claims, err := security.ParseJWT(token, h.Config.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Pool.Exec(context.Background(), "UPDATE user_sessions SET user_id=$1 WHERE token_jti=$2", other, claims.JTI); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/members/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("session for %s accepted as %s: %d", other, owner, rec.Code)
	}
}

func TestCancelledEventCannotVerifyAttendance(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	st, lga, _, _, _ := getLagosAndKanoData(t, h)
	actor, _ := createTestUserWithRole(t, h, "cancel_coord69", "Coordinator", "member")
	insertTestAppointment(t, h, actor, "state_coordinator", "Coordinator", "state", &st.ID, "active", time.Now().Add(-time.Hour), nil)
	event, err := h.CleanupService.CreateEvent(context.Background(), actor, "state_coordinator", &st.ID, service.CreateEventRequest{
		Title: "Cancelled check-in regression", StateID: st.ID, LGAID: lga.ID, Address: "Test venue", StartTime: time.Now().Add(-time.Hour), EndTime: time.Now().Add(time.Hour), Capacity: 10, Status: "published", PublicContact: "+2348000000069",
	})
	if err != nil {
		t.Fatal(err)
	}
	member, _ := createTestUserWithRole(t, h, "cancel_member69", "Member", "member")
	reg, err := h.CleanupService.RegisterForEvent(context.Background(), member, event.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Pool.Exec(context.Background(), "UPDATE event_instances SET status='cancelled' WHERE id=$1", event.ID); err != nil {
		t.Fatal(err)
	}
	_, _, err = h.CleanupService.VerifyAttendance(context.Background(), actor, event.ID, service.VerifyAttendanceRequest{CheckinCode: reg.CheckinCode.String})
	if !errors.Is(err, service.ErrEventCancelled) {
		t.Fatalf("expected cancellation rejection, got %v", err)
	}
	var status string
	if err = h.Pool.QueryRow(context.Background(), "SELECT status FROM event_registrations WHERE id=$1", reg.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "registered" {
		t.Fatalf("cancelled event mutated attendance: %s", status)
	}
}

func TestLoginDoesNotTrustSpoofedForwardingHeaders(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	_, email, _, _, _ := registerAndVerifyMember(t, h, "proxy69", "Proxy Regression")
	payload, _ := json.Marshal(map[string]string{"username": email, "password": "TestPass123!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.RemoteAddr = "198.51.100.69:12345"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	req.Header.Set("X-Real-IP", "127.0.0.1")
	req.Header.Set("CF-Connecting-IP", "203.0.113.9")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	claims, err := security.ParseJWT(response.AccessToken, h.Config.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	session, err := h.Repo.Queries().GetUserSession(context.Background(), claims.JTI)
	if err != nil {
		t.Fatal(err)
	}
	if session.IpAddress.String != "198.51.100.69" {
		t.Fatalf("spoofed IP recorded: %s", session.IpAddress.String)
	}
}

func TestCORSAllowsPatchFromConfiguredOrigin(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/leadership/appointments/test/scope-dates", nil)
	req.Header.Set("Origin", h.Config.AllowedOrigins[0])
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,X-CSRF-Token")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Methods") != "PATCH" {
		t.Fatalf("PATCH preflight denied: %v", rec.Header())
	}
}

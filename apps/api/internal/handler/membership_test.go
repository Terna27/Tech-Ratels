package handler_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"strings"
	"sync"
	"testing"
	"time"
)

func cardFixture(t *testing.T, h *TestHarness, token string) map[string]interface{} {
	t.Helper()
	w := operationRequest(h, token, "GET", "/members/me/card", nil)
	assertStatus(t, w, 200)
	var c map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(c["qr_image_data"].(string), "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	image, err := png.Decode(bytes.NewReader(raw))
	if err != nil || image.Bounds().Dx() != 320 || image.Bounds().Dy() != 320 {
		t.Fatal("invalid QR image", err)
	}
	return c
}
func TestPhase71CardPrivacyLifecycleAndAuthorization(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	id, token := operationalUser(t, h, "member")
	admin, root := operationalUser(t, h, "super_admin")
	_, national := operationalUser(t, h, "national_admin")
	la, lga, kn, klga, _ := getLagosAndKanoData(t, h)
	home(t, h, id, la.ID, lga.ID)
	c := cardFixture(t, h, token)
	path := c["verification_path"].(string)
	ref := c["membership_reference"].(string)
	if strings.Contains(path, id) || strings.Contains(path, ref) || len(strings.TrimPrefix(path, "/verify-membership/")) != 64 {
		t.Fatal("QR exposes an identifier or predictable reference")
	}
	if !strings.HasPrefix(c["qr_image_data"].(string), "data:image/png;base64,iVBOR") {
		t.Fatal("missing PNG QR")
	}
	verify := "/members/verify/" + strings.TrimPrefix(path, "/verify-membership/")
	w := operationRequest(h, "", "GET", verify, nil)
	assertStatus(t, w, 200)
	var out map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out) != 3 || out["status"] != "active" || out["membership_reference"] != ref {
		t.Fatal("public response allowlist", out)
	}
	for _, key := range []string{"id", "email", "phone", "address", "password_hash", "primary_role", "avatar_url", "home_state_id", "appointments"} {
		if _, ok := out[key]; ok {
			t.Fatal("private field", key)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store, private" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("privacy headers missing")
	}
	assertStatus(t, operationRequest(h, "", "GET", "/members/verify/"+id, nil), 404)
	assertStatus(t, operationRequest(h, "", "GET", "/members/me/card", nil), 401)
	assertStatus(t, operationRequest(h, token, "PATCH", "/members/"+id+"/status", map[string]string{"status": "active", "reason": "self"}), 403)
	assertStatus(t, operationRequest(h, national, "PATCH", "/members/"+id+"/status", map[string]string{"status": "suspended", "reason": "not authorized"}), 403)
	coord, coordToken := operationalUser(t, h, "member")
	insertTestAppointment(t, h, coord, "state_coordinator", "Lagos", "state", &la.ID, "active", time.Now().Add(-time.Hour), nil)
	home(t, h, coord, la.ID, lga.ID)
	assertStatus(t, operationRequest(h, coordToken, "PATCH", "/members/"+id+"/status", map[string]string{"status": "suspended", "reason": "coordinator denied"}), 403)
	outside, _ := operationalUser(t, h, "member")
	home(t, h, outside, kn.ID, klga.ID)
	assertStatus(t, operationRequest(h, coordToken, "PATCH", "/coordinator/members/"+outside+"/profile", map[string]string{"home_state_id": la.ID, "home_lga_id": lga.ID, "reason": "outside"}), 403)
	for _, status := range []string{"suspended", "inactive", "revoked", "active"} {
		assertStatus(t, operationRequest(h, root, "PATCH", "/members/"+id+"/status", map[string]string{"status": status, "reason": "Reviewed membership"}), 200)
		w = operationRequest(h, "", "GET", verify, nil)
		assertStatus(t, w, 200)
		out = map[string]interface{}{}
		json.Unmarshal(w.Body.Bytes(), &out)
		if out["status"] != status {
			t.Fatal("stale public status", out)
		}
		if status != "active" && strings.Contains(w.Body.String(), "display_name") {
			t.Fatal("inactive name exposed")
		}
	}
	assertStatus(t, operationRequest(h, token, "GET", "/members/me/card", nil), 401)
	var n int
	err := h.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE actor_id=$1 AND resource_id=$2 AND action='change_membership_status'`, admin, id).Scan(&n)
	if err != nil || n != 4 {
		t.Fatal("status audit", n, err)
	}
	assertStatus(t, operationRequest(h, root, "PATCH", "/members/"+coord+"/status", map[string]string{"status": "revoked", "reason": "officer denied"}), 403)
}
func TestPhase71ProfileValidationStableReferencesAndConcurrentCards(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	id, token := operationalUser(t, h, "member")
	ctx := context.Background()
	la, lga, _, klga, _ := getLagosAndKanoData(t, h)
	c := cardFixture(t, h, token)
	ref := c["membership_reference"]
	good := map[string]interface{}{"display_name": "Updated Member", "home_state_id": la.ID, "home_lga_id": lga.ID, "address": "Private address", "bio": "Volunteer", "avatar_url": "https://example.test/member.jpg", "is_public_badges": false}
	assertStatus(t, operationRequest(h, token, "PUT", "/members/me/profile", good), 200)
	w := operationRequest(h, token, "GET", "/members/me", nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"display_name":"Updated Member"`) || !strings.Contains(w.Body.String(), `"is_public_badges":false`) {
		t.Fatal(w.Body.String())
	}
	for _, bad := range []map[string]interface{}{{"primary_role": "super_admin"}, {"membership_reference": "other"}, {"status": "active"}, {"display_name": " "}, {"address": strings.Repeat("x", 256)}, {"bio": strings.Repeat("x", 2001)}, {"avatar_url": "javascript:alert(1)"}, {"avatar_url": "https://user:secret@example.test/photo"}, {"home_state_id": la.ID, "home_lga_id": klga.ID}, {"home_state_id": "missing"}} {
		assertStatus(t, operationRequest(h, token, "PUT", "/members/me/profile", bad), 400)
	}
	assertStatus(t, operationRequest(h, token, "PUT", "/members/me/profile", map[string]string{"bio": "Revised"}), 200)
	updated := cardFixture(t, h, token)
	if updated["membership_reference"] != ref || updated["display_name"] != "Updated Member" || updated["photo_url"] != "https://example.test/member.jpg" {
		t.Fatal(updated)
	}
	w = operationRequest(h, token, "GET", "/members/me/card/download", nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(w.Body.String(), "data:image/png;base64,") {
		t.Fatal("download incomplete")
	}
	_, err := h.Pool.Exec(ctx, `UPDATE users SET membership_reference='REPLACEMENT' WHERE id=$1`, id)
	if err == nil {
		t.Fatal("reference was mutable")
	}
	id2, token2 := operationalUser(t, h, "member")
	_, err = h.Pool.Exec(ctx, `UPDATE users SET membership_reference=$2 WHERE id=$1`, id2, ref)
	if err == nil {
		t.Fatal("duplicate reference accepted")
	}
	_, err = h.Pool.Exec(ctx, `INSERT INTO users(id,password_hash,display_name,membership_reference) VALUES($1,'test-only','Duplicate',$2)`, security.GenerateUUID(), ref)
	if err == nil {
		t.Fatal("duplicate reference inserted")
	}
	_, err = h.Pool.Exec(ctx, `DELETE FROM member_profiles WHERE user_id=$1`, id2)
	if err != nil {
		t.Fatal(err)
	}
	// Legacy users missing an optional profile still get a card, then self-service repairs it.
	const count = 12
	results := make(chan *httptest.ResponseRecorder, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- operationRequest(h, token2, "GET", "/members/me/card", nil) }()
	}
	wg.Wait()
	close(results)
	var path string
	for rr := range results {
		assertStatus(t, rr, 200)
		var card map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &card)
		if path != "" && path != card["verification_path"] {
			t.Fatal("concurrent links differ")
		}
		path = card["verification_path"].(string)
	}
	var n int
	err = h.Pool.QueryRow(ctx, `SELECT count(*) FROM member_card_links WHERE user_id=$1`, id2).Scan(&n)
	if err != nil || n != 1 {
		t.Fatal("duplicate card links", n, err)
	}
	assertStatus(t, operationRequest(h, token2, "PUT", "/members/me/profile", map[string]string{"display_name": "Legacy repaired"}), 200)
	var profiles int
	if err = h.Pool.QueryRow(ctx, `SELECT count(*) FROM member_profiles WHERE user_id=$1`, id2).Scan(&profiles); err != nil || profiles != 1 {
		t.Fatal("legacy profile not repaired", profiles, err)
	}
	r := httptest.NewRequest("PUT", "/api/v1/members/me/profile", strings.NewReader(`{"display_name":"Blocked"}`))
	r.AddCookie(&http.Cookie{Name: "ratels_session", Value: token})
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, r)
	assertStatus(t, rr, 403)
}

func TestPhase71VerificationCannotReactivateRestrictedMembership(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	id, token := operationalUser(t, h, "member")
	var email string
	if err := h.Pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, id).Scan(&email); err != nil {
		t.Fatal(err)
	}
	// An unused, unexpired email link that predates a suspension must not activate it.
	raw := "phase71-email-verification-token-" + id
	_, err := h.Pool.Exec(ctx, `INSERT INTO email_verification_tokens(id,user_id,email,token_hash,expires_at) VALUES($1,$2,$3,$4,now()+interval '1 hour')`, security.GenerateUUID(), id, email, security.HashToken(raw))
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.Pool.Exec(ctx, `INSERT INTO contact_verifications(id,user_id,identifier,code,expires_at) VALUES($1,$2,$3,'123456',now()+interval '1 hour')`, security.GenerateUUID(), id, email)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"inactive", "suspended", "revoked"} {
		_, err = h.Pool.Exec(ctx, `UPDATE users SET status=$2 WHERE id=$1`, id, status)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, operationRequest(h, "", "POST", "/auth/verify-email", map[string]string{"token": raw}), 400)
		assertStatus(t, operationRequest(h, "", "POST", "/auth/verify-contact", map[string]string{"user_id": id, "identifier": email, "code": "123456"}), 400)
		assertStatus(t, operationRequest(h, token, "GET", "/members/me/card", nil), 401)
		var current string
		if err = h.Pool.QueryRow(ctx, `SELECT status FROM users WHERE id=$1`, id).Scan(&current); err != nil || current != status {
			t.Fatal("verification lifted restriction", current, err)
		}
	}
	// Login's legacy repair must not reactivate a restricted Super Admin either.
	root, _ := operationalUser(t, h, "super_admin")
	_, err = h.Pool.Exec(ctx, `UPDATE users SET status='suspended' WHERE id=$1`, root)
	if err != nil {
		t.Fatal(err)
	}
	var rootEmail string
	if err = h.Pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, root).Scan(&rootEmail); err != nil {
		t.Fatal(err)
	}
	if _, err = h.AuthService.Login(ctx, service.LoginRequest{Username: rootEmail, Password: "TestPass123!"}, "127.0.0.1", "test"); err != service.ErrAccountInactive {
		t.Fatal("restricted Super Admin login bypass", err)
	}
	var current string
	if err = h.Pool.QueryRow(ctx, `SELECT status FROM users WHERE id=$1`, root).Scan(&current); err != nil || current != "suspended" {
		t.Fatal("login lifted restriction", current, err)
	}
}

func TestPhase71PublicVerificationRateLimit(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	for i := 0; i < 60; i++ {
		assertStatus(t, operationRequest(h, "", "GET", "/members/verify/invalid", nil), 404)
	}
	w := operationRequest(h, "", "GET", "/members/verify/invalid", nil)
	assertStatus(t, w, 429)
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("rate limit missing retry delay")
	}
}

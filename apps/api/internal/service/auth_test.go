package service_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func TestAuthAndAntiPrivilegeRegistration(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Anti-privilege test: New user cannot self-appoint privileged roles
	maliciousEmail := fmt.Sprintf("hacker_%d@example.com", time.Now().UnixNano())
	_, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       maliciousEmail,
		Password:    "Password123!",
		DisplayName: "Privilege Escalation Tester",
		Role:        "state_coordinator", // Privileged role
	})
	if err != service.ErrPrivilegedRoleRefused {
		t.Fatalf("Expected ErrPrivilegedRoleRefused when selecting privileged role, got %v", err)
	}

	// 2. Legitimate registration
	userEmail := fmt.Sprintf("test_volunteer_%d@example.com", time.Now().UnixNano())
	regRes, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       userEmail,
		Password:    "Ratels2026Secure!",
		DisplayName: "Legitimate Volunteer",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if regRes.Status != "pending_verification" {
		t.Errorf("Expected user status 'pending_verification', got %s", regRes.Status)
	}

	// 3. Unverified login must fail
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: userEmail,
		Password: "Ratels2026Secure!",
	}, "127.0.0.1", "TestAgent")
	if err != service.ErrUserNotVerified {
		t.Fatalf("Expected ErrUserNotVerified for unverified user, got %v", err)
	}

	// 4. Retrieve OTP from dev-safe adapter
	otpCode := env.Delivery.GetLastCode(userEmail)
	if otpCode == "" {
		t.Fatalf("Expected OTP code in dev-safe delivery adapter for %s, got empty string", userEmail)
	}

	// 5. Verify contact
	verified, err := env.AuthService.VerifyContact(ctx, service.VerifyContactRequest{
		UserID:     regRes.UserID,
		Identifier: userEmail,
		Code:       otpCode,
	})
	if err != nil || !verified {
		t.Fatalf("VerifyContact failed: %v", err)
	}

	// 6. Login now succeeds
	loginRes, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: userEmail,
		Password: "Ratels2026Secure!",
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Login failed after verification: %v", err)
	}

	if loginRes.AccessToken == "" {
		t.Errorf("Expected valid JWT access token, got empty string")
	}
	if loginRes.User.PrimaryRole != "member" {
		t.Errorf("Expected primary role 'member', got %s", loginRes.User.PrimaryRole)
	}
	if !loginRes.User.IsVerified {
		t.Errorf("Expected IsVerified=true, got false")
	}
}

func TestEmailVerificationLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	email := fmt.Sprintf("verify_success_%d@example.com", time.Now().UnixNano())
	regRes, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       email,
		Password:    "SecurePassword2026!",
		DisplayName: "Lifecycle Volunteer",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if regRes.Status != "pending_verification" {
		t.Fatalf("Expected status pending_verification, got %s", regRes.Status)
	}

	// Unverified user cannot log in
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: email,
		Password: "SecurePassword2026!",
	}, "127.0.0.1", "TestAgent")
	if err != service.ErrUserNotVerified {
		t.Fatalf("Expected ErrUserNotVerified before verification, got %v", err)
	}

	// Retrieve verification URL
	rawURL := env.Delivery.GetLastVerificationURL(email)
	if rawURL == "" {
		t.Fatalf("Expected verification URL dispatched, got empty string")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Failed to parse verification URL: %v", err)
	}
	rawToken := u.Query().Get("token")
	if rawToken == "" {
		t.Fatalf("Verification token missing from URL: %s", rawURL)
	}

	// Verify email token
	verifyRes, err := env.AuthService.VerifyEmailToken(ctx, rawToken)
	if err != nil {
		t.Fatalf("VerifyEmailToken failed: %v", err)
	}
	if !verifyRes.IsVerified {
		t.Fatalf("Expected IsVerified=true, got false")
	}
	if verifyRes.AlreadyVerified {
		t.Fatalf("Expected AlreadyVerified=false on first verification")
	}

	// Token record in DB must be is_used=true
	var isUsed bool
	err = env.Pool.QueryRow(ctx, "SELECT is_used FROM email_verification_tokens WHERE token_hash = $1", security.HashToken(rawToken)).Scan(&isUsed)
	if err != nil || !isUsed {
		t.Fatalf("Expected token to be marked is_used in DB, got isUsed=%v, err=%v", isUsed, err)
	}

	// User must now be active and verified
	var status string
	var isUserVerified bool
	err = env.Pool.QueryRow(ctx, "SELECT status, is_verified FROM users WHERE id = $1", regRes.UserID).Scan(&status, &isUserVerified)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}
	if status != "active" || !isUserVerified {
		t.Fatalf("Expected active and verified user, got status=%s, is_verified=%v", status, isUserVerified)
	}

	// Login must now succeed
	loginRes, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: email,
		Password: "SecurePassword2026!",
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Login failed after email verification: %v", err)
	}
	if loginRes.AccessToken == "" {
		t.Errorf("Expected valid JWT token")
	}
	if !loginRes.User.IsVerified {
		t.Errorf("Expected user.IsVerified=true in login response")
	}
}

func TestEmailVerificationExpiredToken(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	email := fmt.Sprintf("verify_expired_%d@example.com", time.Now().UnixNano())
	_, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       email,
		Password:    "SecurePassword2026!",
		DisplayName: "Expired Token Volunteer",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	rawURL := env.Delivery.GetLastVerificationURL(email)
	u, _ := url.Parse(rawURL)
	rawToken := u.Query().Get("token")
	tokenHash := security.HashToken(rawToken)

	// Artificially expire the token in database
	_, err = env.Pool.Exec(ctx, "UPDATE email_verification_tokens SET expires_at = NOW() - INTERVAL '2 hours' WHERE token_hash = $1", tokenHash)
	if err != nil {
		t.Fatalf("Failed to expire token in DB: %v", err)
	}

	// Verification must fail with ErrTokenExpired
	_, err = env.AuthService.VerifyEmailToken(ctx, rawToken)
	if err != service.ErrTokenExpired {
		t.Fatalf("Expected ErrTokenExpired, got %v", err)
	}

	// User remains unverified
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: email,
		Password: "SecurePassword2026!",
	}, "127.0.0.1", "TestAgent")
	if err != service.ErrUserNotVerified {
		t.Fatalf("Expected ErrUserNotVerified after expired token attempt, got %v", err)
	}
}

func TestEmailVerificationTokenReuse(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	email := fmt.Sprintf("verify_reuse_%d@example.com", time.Now().UnixNano())
	_, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       email,
		Password:    "SecurePassword2026!",
		DisplayName: "Reuse Volunteer",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	rawURL := env.Delivery.GetLastVerificationURL(email)
	u, _ := url.Parse(rawURL)
	rawToken := u.Query().Get("token")

	// First verification succeeds
	res1, err := env.AuthService.VerifyEmailToken(ctx, rawToken)
	if err != nil || !res1.IsVerified {
		t.Fatalf("First verification failed: %v", err)
	}

	// Second verification with the same token should report already verified gracefully
	res2, err := env.AuthService.VerifyEmailToken(ctx, rawToken)
	if err != nil {
		t.Fatalf("Second verification returned unexpected error: %v", err)
	}
	if !res2.AlreadyVerified || !res2.IsVerified {
		t.Fatalf("Expected AlreadyVerified=true, got %+v", res2)
	}
}

func TestEmailVerificationResendInvalidation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	email := fmt.Sprintf("verify_resend_%d@example.com", time.Now().UnixNano())
	_, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       email,
		Password:    "SecurePassword2026!",
		DisplayName: "Resend Volunteer",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	rawURL1 := env.Delivery.GetLastVerificationURL(email)
	u1, _ := url.Parse(rawURL1)
	token1 := u1.Query().Get("token")

	// Fast-forward cooldown in database for this email and IP
	testIP := "10.0.0.1"
	_, _ = env.Pool.Exec(ctx, "DELETE FROM email_verification_rate_limits WHERE identifier = $1 OR identifier = $2", email, testIP)

	// Resend verification
	resendResp, err := env.AuthService.ResendVerificationEmail(ctx, email, testIP)
	if err != nil {
		t.Fatalf("ResendVerificationEmail failed: %v", err)
	}
	if !resendResp.Sent {
		t.Fatalf("Expected Sent=true from resend")
	}

	rawURL2 := env.Delivery.GetLastVerificationURL(email)
	if rawURL1 == rawURL2 {
		t.Fatalf("Expected new verification URL upon resend, got identical URL")
	}
	u2, _ := url.Parse(rawURL2)
	token2 := u2.Query().Get("token")

	// Older token 1 must now be rejected because it was invalidated
	_, err = env.AuthService.VerifyEmailToken(ctx, token1)
	if err != service.ErrTokenAlreadyUsed {
		t.Fatalf("Expected ErrTokenAlreadyUsed for invalidated older token, got %v", err)
	}

	// Newer token 2 must succeed
	res, err := env.AuthService.VerifyEmailToken(ctx, token2)
	if err != nil || !res.IsVerified {
		t.Fatalf("Verification with fresh token failed: %v", err)
	}

	// Login succeeds
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: email,
		Password: "SecurePassword2026!",
	}, testIP, "TestAgent")
	if err != nil {
		t.Fatalf("Login failed after token 2 verification: %v", err)
	}
}

func TestEmailVerificationDuplicateRegistration(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// Scenario A: Verified account duplicate registration attempt
	verifiedEmail := fmt.Sprintf("dup_verified_%d@example.com", time.Now().UnixNano())
	_, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       verifiedEmail,
		Password:    "OriginalPassword123!",
		DisplayName: "Verified User",
	})
	if err != nil {
		t.Fatalf("Initial register failed: %v", err)
	}
	rawURL := env.Delivery.GetLastVerificationURL(verifiedEmail)
	u, _ := url.Parse(rawURL)
	token := u.Query().Get("token")
	if _, err := env.AuthService.VerifyEmailToken(ctx, token); err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	// Now try to register again with same email and different password
	_, err = env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       verifiedEmail,
		Password:    "AttackerPassword999!",
		DisplayName: "Impostor",
	})
	if err != nil {
		t.Fatalf("Register should return generic success without revealing existence, got error: %v", err)
	}

	// Verify database record remains active and verified (not reverted or changed)
	var dbStatus string
	var dbIsVerified bool
	err = env.Pool.QueryRow(ctx, "SELECT status, is_verified FROM users WHERE email = $1", verifiedEmail).Scan(&dbStatus, &dbIsVerified)
	if err != nil {
		t.Fatalf("Failed to query DB user: %v", err)
	}
	if dbStatus != "active" || !dbIsVerified {
		t.Errorf("Expected DB user to remain active and verified, got status=%s, is_verified=%v", dbStatus, dbIsVerified)
	}

	// Original password must STILL WORK (not overwritten)
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: verifiedEmail,
		Password: "OriginalPassword123!",
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Original password was compromised or overwritten by duplicate registration: %v", err)
	}

	// New password must NOT work
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: verifiedEmail,
		Password: "AttackerPassword999!",
	}, "127.0.0.1", "TestAgent")
	if err != service.ErrInvalidCredentials {
		t.Fatalf("Attacker password should be rejected, got %v", err)
	}

	// Scenario B: Unverified account duplicate registration refreshes token
	unverifiedEmail := fmt.Sprintf("dup_unverified_%d@example.com", time.Now().UnixNano())
	reg1, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       unverifiedEmail,
		Password:    "UnverifiedPass123!",
		DisplayName: "Unverified Person",
	})
	if err != nil {
		t.Fatalf("Register 1 failed: %v", err)
	}
	u1, _ := url.Parse(env.Delivery.GetLastVerificationURL(unverifiedEmail))
	tokenA := u1.Query().Get("token")

	// Registering again refreshes verification token
	reg2, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       unverifiedEmail,
		Password:    "UnverifiedPass123!",
		DisplayName: "Unverified Person",
	})
	if err != nil {
		t.Fatalf("Register 2 failed: %v", err)
	}
	if reg2.UserID != reg1.UserID {
		t.Fatalf("Expected same user ID, got %s vs %s", reg1.UserID, reg2.UserID)
	}
	u2, _ := url.Parse(env.Delivery.GetLastVerificationURL(unverifiedEmail))
	tokenB := u2.Query().Get("token")
	if tokenA == tokenB {
		t.Fatalf("Expected token to be refreshed on re-registration")
	}

	// Old token is invalidated
	_, err = env.AuthService.VerifyEmailToken(ctx, tokenA)
	if err != service.ErrTokenAlreadyUsed {
		t.Fatalf("Expected older token to be invalidated, got %v", err)
	}

	// New token verifies successfully
	if _, err := env.AuthService.VerifyEmailToken(ctx, tokenB); err != nil {
		t.Fatalf("Token B verification failed: %v", err)
	}
}

func TestEmailVerificationProviderFailure(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	email := fmt.Sprintf("provider_fail_%d@example.com", time.Now().UnixNano())

	// Simulate delivery failure
	env.Delivery.SimulateFail = true

	regRes, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       email,
		Password:    "FailoverTest123!",
		DisplayName: "Failover Tester",
	})
	if err != nil {
		t.Fatalf("Register should succeed creating account even if delivery fails, got error: %v", err)
	}
	if regRes.DeliveryStatus != "failed" {
		t.Errorf("Expected delivery_status='failed', got %s", regRes.DeliveryStatus)
	}

	// User cannot log in yet
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: email,
		Password: "FailoverTest123!",
	}, "127.0.0.1", "TestAgent")
	if err != service.ErrUserNotVerified {
		t.Fatalf("Expected ErrUserNotVerified, got %v", err)
	}

	// Restore delivery adapter
	env.Delivery.SimulateFail = false

	// Clear cooldown for test
	_, _ = env.Pool.Exec(ctx, "DELETE FROM email_verification_rate_limits WHERE identifier = $1", email)

	// Resend succeeds
	resendRes, err := env.AuthService.ResendVerificationEmail(ctx, email, "127.0.0.1")
	if err != nil || !resendRes.Sent {
		t.Fatalf("Resend failed after provider restoration: %v", err)
	}

	u, _ := url.Parse(env.Delivery.GetLastVerificationURL(email))
	token := u.Query().Get("token")
	if _, err := env.AuthService.VerifyEmailToken(ctx, token); err != nil {
		t.Fatalf("Verification after resend failed: %v", err)
	}

	// Now can login
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: email,
		Password: "FailoverTest123!",
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
}

func TestEmailVerificationCooldownAndRateLimits(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	email := fmt.Sprintf("rate_limit_%d@example.com", time.Now().UnixNano())
	testIP := fmt.Sprintf("192.168.10.%d", time.Now().Unix()%250+1)

	_, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       email,
		Password:    "RateLimitTest123!",
		DisplayName: "Rate Limit Tester",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 1. Immediate resend within 60s should fail with cooldown
	_, err = env.AuthService.ResendVerificationEmail(ctx, email, testIP)
	if err == nil || !errors.Is(err, service.ErrResendCooldownActive) {
		t.Fatalf("Expected ErrResendCooldownActive on immediate resend, got %v", err)
	}

	// 2. IP rate limit test: Hit max resends (10 per hour for IP)
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO email_verification_rate_limits (id, identifier, action, request_count, first_requested_at, last_requested_at)
		VALUES ($1, $2, 'resend_email', 10, NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '5 minutes')
		ON CONFLICT (identifier, action) DO UPDATE SET request_count = 10, first_requested_at = NOW() - INTERVAL '10 minutes', last_requested_at = NOW() - INTERVAL '5 minutes'
	`, security.GenerateUUID(), testIP)
	if err != nil {
		t.Fatalf("Failed to seed IP rate limit: %v", err)
	}

	// Now any resend from this IP should fail with ErrRateLimitExceeded
	diffEmail := fmt.Sprintf("rate_diff_%d@example.com", time.Now().UnixNano())
	_, err = env.AuthService.ResendVerificationEmail(ctx, diffEmail, testIP)
	if err != service.ErrRateLimitExceeded {
		t.Fatalf("Expected ErrRateLimitExceeded for throttled IP, got %v", err)
	}
}

func TestEmailVerificationAntiEnumeration(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	nonExistentEmail := fmt.Sprintf("nonexistent_%d@example.com", time.Now().UnixNano())
	res, err := env.AuthService.ResendVerificationEmail(ctx, nonExistentEmail, "10.10.10.10")
	if err != nil {
		t.Fatalf("Resend on non-existent email should not return error, got %v", err)
	}
	if !res.Sent {
		t.Fatalf("Resend should return neutral sent=true to prevent user enumeration")
	}
}


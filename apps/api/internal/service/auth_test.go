package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

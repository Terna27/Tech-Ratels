package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func TestLogin_RegressionAndAuthenticationInvariants(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Password hash compatibility test (bcrypt standard hash)
	plainPassword := "NigerianRatels2026!Secure"
	knownBcryptHash, err := security.HashPassword(plainPassword)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	if !security.CheckPassword(plainPassword, knownBcryptHash) {
		t.Fatalf("Password verification failed on standard bcrypt hash")
	}
	if security.CheckPassword("WrongPassword123!", knownBcryptHash) {
		t.Fatalf("CheckPassword incorrectly accepted wrong password")
	}

	// 2. Setup active verified user with email and phone
	uniqueID := time.Now().UnixNano()
	userEmail := fmt.Sprintf("Existing_User_%d@Example.Org", uniqueID) // Mixed case email
	userPhone := fmt.Sprintf("+23480%08d", uniqueID%100000000)
	localPhone := "080" + userPhone[6:]
	memRef := fmt.Sprintf("RATEL-2026-%06d", uniqueID%1000000)
	userID := security.GenerateUUID()

	_, err = env.Repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  userID,
		Email:               pgtype.Text{String: userEmail, Valid: true},
		Phone:               pgtype.Text{String: userPhone, Valid: true},
		PasswordHash:        knownBcryptHash,
		DisplayName:         "Test Existing User",
		MembershipReference: memRef,
		Status:              "active",
		PrimaryRole:         "member",
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("Failed to create test existing user: %v", err)
	}

	// A. Valid user login via lowercase email
	res, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: strings.ToLower(userEmail),
		Password: plainPassword,
	}, "127.0.0.1", "Mozilla/5.0 Test Suite")
	if err != nil {
		t.Fatalf("Login failed for valid lowercase email: %v", err)
	}
	if res.AccessToken == "" || res.User.ID != userID {
		t.Fatalf("Invalid login response token or user ID: %+v", res)
	}

	// B. Valid user login via uppercase email (case insensitivity)
	resUpper, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: strings.ToUpper(userEmail),
		Password: plainPassword,
	}, "127.0.0.1", "Mozilla/5.0 Test Suite")
	if err != nil {
		t.Fatalf("Login failed for valid uppercase email: %v", err)
	}
	if resUpper.User.ID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, resUpper.User.ID)
	}

	// C. Valid user login via local phone format (080...) when registered as +23480...
	resPhone, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: localPhone,
		Password: plainPassword,
	}, "127.0.0.1", "Mozilla/5.0 Test Suite")
	if err != nil {
		t.Fatalf("Login failed for phone in local format %s: %v", localPhone, err)
	}
	if resPhone.User.ID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, resPhone.User.ID)
	}

	// D. Valid user login via lowercase membership reference
	resMem, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: strings.ToLower(memRef),
		Password: plainPassword,
	}, "127.0.0.1", "Mozilla/5.0 Test Suite")
	if err != nil {
		t.Fatalf("Login failed for lowercase membership reference %s: %v", strings.ToLower(memRef), err)
	}
	if resMem.User.ID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, resMem.User.ID)
	}

	// E. Invalid password rejection
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: userEmail,
		Password: "IncorrectPassword123!",
	}, "127.0.0.1", "Mozilla/5.0 Test Suite")
	if err != service.ErrInvalidCredentials {
		t.Fatalf("Expected ErrInvalidCredentials on wrong password, got %v", err)
	}

	// F. Unknown account rejection
	_, err = env.AuthService.Login(ctx, service.LoginRequest{
		Username: "nonexistent_ratels_user@example.org",
		Password: plainPassword,
	}, "127.0.0.1", "Mozilla/5.0 Test Suite")
	if err != service.ErrInvalidCredentials {
		t.Fatalf("Expected ErrInvalidCredentials on unknown account, got %v", err)
	}

	// G. Very long IP address and User-Agent safety (clamp overflow prevention)
	veryLongIP := "2001:0db8:85a3:0000:0000:8a2e:0370:7334, 102.89.47.12, 172.71.147.20, 10.0.0.1"
	veryLongUA := strings.Repeat("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 ", 10)
	resLong, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: userEmail,
		Password: plainPassword,
	}, veryLongIP, veryLongUA)
	if err != nil {
		t.Fatalf("Login failed with very long IP and User-Agent: %v", err)
	}
	if resLong.AccessToken == "" {
		t.Errorf("Expected valid token on login with clamped metadata")
	}

	// H. Legacy active account auto-healing: status='active', is_verified=false
	legacyUserID := security.GenerateUUID()
	legacyEmail := fmt.Sprintf("legacy_active_%d@example.org", uniqueID)
	_, err = env.Repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  legacyUserID,
		Email:               pgtype.Text{String: legacyEmail, Valid: true},
		Phone:               pgtype.Text{Valid: false},
		PasswordHash:        knownBcryptHash,
		DisplayName:         "Legacy Active Member",
		MembershipReference: fmt.Sprintf("RATEL-LEGACY-%06d", uniqueID%1000000),
		Status:              "active",
		PrimaryRole:         "member",
		IsVerified:          false, // Legacy state before email verification
	})
	if err != nil {
		t.Fatalf("Failed to create legacy user: %v", err)
	}

	resLegacy, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: legacyEmail,
		Password: plainPassword,
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Legacy active user failed to login: %v", err)
	}
	if !resLegacy.User.IsVerified {
		t.Errorf("Expected legacy user to be auto-healed to is_verified=true")
	}

	// I. Promoted Super Admin auto-healing: primary_role='super_admin', is_verified=false
	saUserID := security.GenerateUUID()
	saEmail := fmt.Sprintf("promoted_sa_%d@example.org", uniqueID)
	_, err = env.Repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  saUserID,
		Email:               pgtype.Text{String: saEmail, Valid: true},
		Phone:               pgtype.Text{Valid: false},
		PasswordHash:        knownBcryptHash,
		DisplayName:         "Promoted Super Admin",
		MembershipReference: fmt.Sprintf("RATEL-SA-%06d", uniqueID%1000000),
		Status:              "pending_verification", // Left by promote flag
		PrimaryRole:         "super_admin",
		IsVerified:          false, // Left by promote flag
	})
	if err != nil {
		t.Fatalf("Failed to create promoted SA: %v", err)
	}

	resSA, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: saEmail,
		Password: plainPassword,
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Promoted Super Admin failed to login: %v", err)
	}
	if resSA.User.Status != "active" || !resSA.User.IsVerified {
		t.Errorf("Expected Super Admin to be auto-healed to active and verified")
	}
}

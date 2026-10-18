package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ratels-api/internal/config"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrUserAlreadyExists       = errors.New("user with this email or phone already exists")
	ErrInvalidCredentials      = errors.New("invalid email or password")
	ErrUserNotVerified         = errors.New("user account is pending contact verification")
	ErrAccountInactive         = errors.New("user account is suspended or inactive")
	ErrInvalidVerificationCode = errors.New("invalid or expired verification code")
	ErrPrivilegedRoleRefused   = errors.New("privileged roles cannot be self-selected during registration")
)

type AuthService struct {
	repo     *repository.Repository
	cfg      *config.Config
	delivery delivery.DeliveryAdapter
	audit    *AuditService
}

func NewAuthService(repo *repository.Repository, cfg *config.Config, del delivery.DeliveryAdapter, audit *AuditService) *AuthService {
	return &AuthService{
		repo:     repo,
		cfg:      cfg,
		delivery: del,
		audit:    audit,
	}
}

type RegisterRequest struct {
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	Password    string  `json:"password"`
	DisplayName string  `json:"display_name"`
	HomeStateID *string `json:"home_state_id"`
	HomeLGAID   *string `json:"home_lga_id"`
	Role        string  `json:"role,omitempty"` // For testing anti-privilege rule
}

type RegisterResponse struct {
	UserID              string `json:"user_id"`
	MembershipReference string `json:"membership_reference"`
	Status              string `json:"status"`
	Message             string `json:"message"`
}

type VerifyContactRequest struct {
	UserID     string `json:"user_id"`
	Identifier string `json:"identifier"`
	Code       string `json:"code"`
}

type LoginRequest struct {
	Username   string `json:"username"`   // email, phone, or membership reference
	Identifier string `json:"identifier"` // alternative alias
	Password   string `json:"password"`
}

type LoginResponse struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresIn   int         `json:"expires_in"`
	User        UserSummary `json:"user"`
}

type UserSummary struct {
	ID                  string              `json:"id"`
	Email               *string             `json:"email"`
	Phone               *string             `json:"phone"`
	DisplayName         string              `json:"display_name"`
	MembershipReference string              `json:"membership_reference"`
	Status              string              `json:"status"`
	PrimaryRole         string              `json:"primary_role"`
	IsVerified          bool                `json:"is_verified"`
	Profile             *db.MemberProfile   `json:"profile,omitempty"`
	Appointments        []db.Appointment    `json:"appointments,omitempty"`
	Badges              []BadgeAwardSummary `json:"badges,omitempty"`
}

type BadgeAwardSummary struct {
	ID          string    `json:"id"`
	BadgeCode   string    `json:"badge_code"`
	BadgeName   string    `json:"badge_name"`
	Description string    `json:"description"`
	AwardedAt   time.Time `json:"awarded_at"`
}

func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	// Anti-privilege rule: Registration cannot grant privileged roles
	if req.Role != "" && req.Role != "member" {
		return nil, ErrPrivilegedRoleRefused
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	phone := strings.TrimSpace(req.Phone)
	if email == "" && phone == "" {
		return nil, errors.New("either email or phone must be provided")
	}

	if len(req.Password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	// Check existing user
	if email != "" {
		if existing, _ := s.repo.Queries().GetUserByEmail(ctx, TextFromString(email)); existing.ID != "" {
			return nil, ErrUserAlreadyExists
		}
	}
	if phone != "" {
		if existing, _ := s.repo.Queries().GetUserByPhone(ctx, TextFromString(phone)); existing.ID != "" {
			return nil, ErrUserAlreadyExists
		}
	}

	pwHash, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userID := security.GenerateUUID()
	var createdUser db.User
	for attempt := 0; attempt < 5; attempt++ {
		memRef := security.GenerateMembershipReference()
		err = s.repo.WithTx(ctx, func(q *db.Queries) error {
			u, err := q.CreateUser(ctx, db.CreateUserParams{
				ID:                  userID,
				Email:               TextFromString(email),
				Phone:               TextFromString(phone),
				PasswordHash:        pwHash,
				DisplayName:         req.DisplayName,
				MembershipReference: memRef,
				Status:              "pending_verification",
				PrimaryRole:         "member", // Strictly forced to member
				IsVerified:          false,
			})
			if err != nil {
				return err
			}
			createdUser = u

			// Create default profile
			_, err = q.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
				ID:             security.GenerateUUID(),
				UserID:         userID,
				HomeStateID:    Text(req.HomeStateID),
				HomeLgaID:      Text(req.HomeLGAID),
				IsPublicBadges: true,
			})
			if err != nil {
				return err
			}

			// Generate 6-digit OTP
			otpCode := security.GenerateVerificationCode()
			identifier := email
			if identifier == "" {
				identifier = phone
			}

			_, err = q.CreateContactVerification(ctx, db.CreateContactVerificationParams{
				ID:         security.GenerateUUID(),
				UserID:     userID,
				Identifier: identifier,
				Code:       otpCode,
				ExpiresAt:  TimestamptzFromTime(time.Now().Add(15 * time.Minute)),
				IsUsed:     false,
			})
			if err != nil {
				return err
			}

			// Dispatch via safe adapter
			_ = s.delivery.SendVerificationCode(identifier, otpCode)
			return nil
		})

		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "users_membership_reference_key") {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	return &RegisterResponse{
		UserID:              createdUser.ID,
		MembershipReference: createdUser.MembershipReference,
		Status:              createdUser.Status,
		Message:             "Registration initiated. Verification code dispatched via development adapter.",
	}, nil
}

func (s *AuthService) VerifyContact(ctx context.Context, req VerifyContactRequest) (bool, error) {
	record, err := s.repo.Queries().GetActiveVerification(ctx, db.GetActiveVerificationParams{
		UserID:     req.UserID,
		Identifier: req.Identifier,
	})
	if err != nil {
		return false, ErrInvalidVerificationCode
	}

	if record.Code != req.Code {
		_, _ = s.repo.Queries().IncrementVerificationAttempt(ctx, record.ID)
		return false, ErrInvalidVerificationCode
	}

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		if _, err := q.MarkVerificationUsed(ctx, record.ID); err != nil {
			return err
		}
		if _, err := q.UpdateUserStatus(ctx, db.UpdateUserStatusParams{
			ID:         req.UserID,
			Status:     "active",
			IsVerified: true,
		}); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest, ip, userAgent string) (*LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = strings.TrimSpace(req.Identifier)
	}
	var user db.User
	var err error

	if strings.Contains(username, "@") {
		email := strings.ToLower(username)
		user, err = s.repo.Queries().GetUserByEmail(ctx, TextFromString(email))
	} else {
		user, err = s.repo.Queries().GetUserByPhone(ctx, TextFromString(username))
		if err != nil {
			user, err = s.repo.Queries().GetUserByMembershipReference(ctx, username)
		}
	}

	if err != nil || user.ID == "" {
		return nil, ErrInvalidCredentials
	}

	if !security.CheckPassword(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	if user.Status == "pending_verification" {
		return nil, ErrUserNotVerified
	}
	if user.Status != "active" {
		return nil, ErrAccountInactive
	}

	jti := security.GenerateJTI()
	token, expiresAt, err := security.GenerateJWT(user.ID, username, user.PrimaryRole, jti, s.cfg.JWTSecret, s.cfg.JWTExpiryHours)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	_, err = s.repo.Queries().CreateUserSession(ctx, db.CreateUserSessionParams{
		ID:        security.GenerateUUID(),
		UserID:    user.ID,
		TokenJti:  jti,
		ExpiresAt: TimestamptzFromTime(expiresAt),
		IpAddress: TextFromString(ip),
		UserAgent: TextFromString(userAgent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record user session: %w", err)
	}

	summary, err := s.GetMe(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken: token,
		TokenType:   "bearer",
		ExpiresIn:   s.cfg.JWTExpiryHours * 3600,
		User:        *summary,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, jti string) error {
	return s.repo.Queries().RevokeUserSession(ctx, jti)
}

func (s *AuthService) GetMe(ctx context.Context, userID string) (*UserSummary, error) {
	user, err := s.repo.Queries().GetUserByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	profile, _ := s.repo.Queries().GetMemberProfileByUserID(ctx, userID)
	appointments, _ := s.repo.Queries().ListActiveAppointmentsByUser(ctx, userID)
	badgeRows, _ := s.repo.Queries().ListBadgeAwardsByUser(ctx, userID)

	badgeDTOs := make([]BadgeAwardSummary, len(badgeRows))
	for i, b := range badgeRows {
		badgeDTOs[i] = BadgeAwardSummary{
			ID:          b.ID,
			BadgeCode:   b.BadgeCode,
			BadgeName:   b.BadgeName,
			Description: b.BadgeDescription,
			AwardedAt:   b.AwardedAt.Time,
		}
	}

	var pProfile *db.MemberProfile
	if profile.ID != "" {
		pProfile = &profile
	}

	return &UserSummary{
		ID:                  user.ID,
		Email:               StringFromText(user.Email),
		Phone:               StringFromText(user.Phone),
		DisplayName:         user.DisplayName,
		MembershipReference: user.MembershipReference,
		Status:              user.Status,
		PrimaryRole:         user.PrimaryRole,
		IsVerified:          user.IsVerified,
		Profile:             pProfile,
		Appointments:        appointments,
		Badges:              badgeDTOs,
	}, nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID string, homeStateID, homeLGAID, address, bio, avatarURL *string) (*db.MemberProfile, error) {
	p, err := s.repo.Queries().UpdateMemberProfile(ctx, db.UpdateMemberProfileParams{
		UserID:      userID,
		HomeStateID: Text(homeStateID),
		HomeLgaID:   Text(homeLGAID),
		Address:     Text(address),
		Bio:         Text(bio),
		AvatarUrl:   Text(avatarURL),
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

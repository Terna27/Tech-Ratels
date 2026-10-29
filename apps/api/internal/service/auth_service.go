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
	ErrInvalidToken            = errors.New("invalid or malformed verification link")
	ErrTokenExpired            = errors.New("this verification link has expired. Please request a new one")
	ErrTokenAlreadyUsed        = errors.New("this verification link has already been used")
	ErrResendCooldownActive    = errors.New("please wait before requesting another verification email")
	ErrRateLimitExceeded       = errors.New("too many verification requests. Please try again later")
	ErrInvalidEmail            = errors.New("a valid email address is required")
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
	DeliveryStatus      string `json:"delivery_status"`
	DevVerificationURL  string `json:"_dev_verification_url,omitempty"`
	DevVerificationCode string `json:"_dev_verification_code,omitempty"`
}

type VerifyContactRequest struct {
	UserID     string `json:"user_id"`
	Identifier string `json:"identifier"`
	Code       string `json:"code"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type VerifyEmailResponse struct {
	Message         string `json:"message"`
	IsVerified      bool   `json:"is_verified"`
	AlreadyVerified bool   `json:"already_verified"`
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

type ResendVerificationResponse struct {
	Message         string `json:"message"`
	Sent            bool   `json:"sent"`
	AlreadyVerified bool   `json:"already_verified,omitempty"`
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

	// Check existing user: Avoid revealing whether an email is already registered (anti-enumeration)
	if email != "" {
		if existing, _ := s.repo.Queries().GetUserByEmail(ctx, email); existing.ID != "" {
			if existing.IsVerified && existing.Status == "active" {
				return &RegisterResponse{
					UserID:              existing.ID,
					MembershipReference: existing.MembershipReference,
					Status:              "pending_verification",
					Message:             "Registration initiated. If this email is eligible, a verification link has been sent.",
					DeliveryStatus:      "sent",
				}, nil
			}

			// User is unverified: refresh verification token and resend verification link
			rawToken, err := security.GenerateSecureToken()
			if err != nil {
				return nil, err
			}
			tokenHash := security.HashToken(rawToken)

			_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
				_ = q.InvalidatePendingEmailVerificationTokens(ctx, existing.ID)
				_, err := q.CreateEmailVerificationToken(ctx, db.CreateEmailVerificationTokenParams{
					ID:        security.GenerateUUID(),
					UserID:    existing.ID,
					Email:     email,
					TokenHash: tokenHash,
					ExpiresAt: TimestamptzFromTime(time.Now().Add(24 * time.Hour)),
				})
				_, _ = q.UpsertVerificationRateLimit(ctx, db.UpsertVerificationRateLimitParams{
					ID:         security.GenerateUUID(),
					Identifier: email,
					Action:     "resend_email",
				})
				return err
			})

			verificationURL := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.FrontendBaseURL, rawToken)
			deliveryErr := s.delivery.SendVerificationEmail(email, existing.DisplayName, verificationURL)

			deliveryStatus := "sent"
			msg := "Registration initiated. A verification link has been sent to your email."
			if deliveryErr != nil {
				deliveryStatus = "failed"
				msg = "Registration initiated, but verification email could not be delivered. Please use the resend option."
			}

			return &RegisterResponse{
				UserID:              existing.ID,
				MembershipReference: existing.MembershipReference,
				Status:              "pending_verification",
				Message:             msg,
				DeliveryStatus:      deliveryStatus,
				DevVerificationURL:  verificationURL,
				DevVerificationCode: s.delivery.GetLastCode(email),
			}, nil
		}
	}
	if phone != "" {
		if existing, _ := s.repo.Queries().GetUserByPhone(ctx, phone); existing.ID != "" {
			return nil, ErrUserAlreadyExists
		}
	}

	pwHash, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userID := security.GenerateUUID()
	var createdUser db.User
	var rawEmailToken string
	var verificationURL string

	for attempt := 0; attempt < 5; attempt++ {
		memRef := security.GenerateMembershipReference()
		rawToken, tokenGenErr := security.GenerateSecureToken()
		if tokenGenErr != nil {
			return nil, tokenGenErr
		}
		rawEmailToken = rawToken
		tokenHash := security.HashToken(rawToken)

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

			// Store email verification token with 24-hour expiry if email provided
			if email != "" {
				_, err = q.CreateEmailVerificationToken(ctx, db.CreateEmailVerificationTokenParams{
					ID:        security.GenerateUUID(),
					UserID:    userID,
					Email:     email,
					TokenHash: tokenHash,
					ExpiresAt: TimestamptzFromTime(time.Now().Add(24 * time.Hour)),
				})
				if err != nil {
					return err
				}

				// Initialize rate limit record so cooldown applies immediately
				_, _ = q.UpsertVerificationRateLimit(ctx, db.UpsertVerificationRateLimitParams{
					ID:         security.GenerateUUID(),
					Identifier: email,
					Action:     "resend_email",
				})
			}

			// Generate 6-digit OTP in contact_verifications for backward compatibility
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
				ExpiresAt:  TimestamptzFromTime(time.Now().Add(24 * time.Hour)),
				IsUsed:     false,
			})
			if err != nil {
				return err
			}

			// Dispatch verification code via adapter
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

	deliveryStatus := "sent"
	msg := "Registration successful! Please check your email to verify your account."

	if email != "" {
		verificationURL = fmt.Sprintf("%s/verify-email?token=%s", s.cfg.FrontendBaseURL, rawEmailToken)
		deliveryErr := s.delivery.SendVerificationEmail(email, req.DisplayName, verificationURL)
		if deliveryErr != nil {
			deliveryStatus = "failed"
			msg = "Account registered, but verification email could not be dispatched. Please use the resend option."
		}
	}

	return &RegisterResponse{
		UserID:              createdUser.ID,
		MembershipReference: createdUser.MembershipReference,
		Status:              createdUser.Status,
		Message:             msg,
		DeliveryStatus:      deliveryStatus,
		DevVerificationURL:  verificationURL,
		DevVerificationCode: s.delivery.GetLastCode(email),
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
		if err := q.ActivateVerifiedMember(ctx, req.UserID); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return false, err
	}

	return true, nil
}

// VerifyEmailToken validates a single-use token, activates the account, and consumes the token atomically.
func (s *AuthService) VerifyEmailToken(ctx context.Context, rawToken string) (*VerifyEmailResponse, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || len(rawToken) < 16 {
		return nil, ErrInvalidToken
	}

	tokenHash := security.HashToken(rawToken)

	record, err := s.repo.Queries().GetEmailVerificationTokenByHash(ctx, tokenHash)
	if err != nil || record.ID == "" {
		return nil, ErrInvalidToken
	}

	// If token was already used, check whether the user is already verified
	if record.IsUsed {
		user, uErr := s.repo.Queries().GetUserByID(ctx, record.UserID)
		if uErr == nil && user.IsVerified && user.Status == "active" {
			return &VerifyEmailResponse{
				Message:         "Email already verified. Your account is active. Please proceed to login.",
				IsVerified:      true,
				AlreadyVerified: true,
			}, nil
		}
		return nil, ErrTokenAlreadyUsed
	}

	// Check expiry (24-hour lifetime)
	if record.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrTokenExpired
	}

	// Atomic consumption of token and user activation
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Mark this token used
		if _, err := q.MarkEmailVerificationTokenUsed(ctx, record.ID); err != nil {
			return err
		}
		// Invalidate any other pending tokens for this user
		if err := q.InvalidatePendingEmailVerificationTokens(ctx, record.UserID); err != nil {
			return err
		}
		// Mark user verified and active
		if err := q.ActivateVerifiedMember(ctx, record.UserID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to complete email verification: %w", err)
	}

	return &VerifyEmailResponse{
		Message:         "Email verified successfully. Your account is active. You may now log in.",
		IsVerified:      true,
		AlreadyVerified: false,
	}, nil
}

const (
	ResendCooldownDuration = 60 * time.Second
	MaxResendsPerHourEmail = 5
	MaxResendsPerHourIP    = 10
)

func (s *AuthService) checkAndEnforceRateLimits(ctx context.Context, email, ip string) error {
	now := time.Now()

	// 1. Check IP rate limit
	if ip != "" {
		lim, err := s.repo.Queries().GetVerificationRateLimit(ctx, db.GetVerificationRateLimitParams{
			Identifier: ip,
			Action:     "resend_email",
		})
		if err == nil && lim.ID != "" {
			if now.Sub(lim.FirstRequestedAt.Time) < time.Hour {
				if lim.RequestCount >= MaxResendsPerHourIP {
					return ErrRateLimitExceeded
				}
			} else {
				_, _ = s.repo.Queries().ResetVerificationRateLimit(ctx, db.ResetVerificationRateLimitParams{
					Identifier: ip,
					Action:     "resend_email",
				})
			}
		}
	}

	// 2. Check Email cooldown and rate limit
	if email != "" {
		lim, err := s.repo.Queries().GetVerificationRateLimit(ctx, db.GetVerificationRateLimitParams{
			Identifier: email,
			Action:     "resend_email",
		})
		if err == nil && lim.ID != "" {
			timeSinceLast := now.Sub(lim.LastRequestedAt.Time)
			if timeSinceLast < ResendCooldownDuration {
				secondsRemaining := int((ResendCooldownDuration - timeSinceLast).Seconds())
				if secondsRemaining < 1 {
					secondsRemaining = 1
				}
				return fmt.Errorf("%w: please wait %d seconds before requesting another email", ErrResendCooldownActive, secondsRemaining)
			}

			if now.Sub(lim.FirstRequestedAt.Time) < time.Hour {
				if lim.RequestCount >= MaxResendsPerHourEmail {
					return ErrRateLimitExceeded
				}
			} else {
				_, _ = s.repo.Queries().ResetVerificationRateLimit(ctx, db.ResetVerificationRateLimitParams{
					Identifier: email,
					Action:     "resend_email",
				})
			}
		}
	}

	// Record request in rate limit table for both IP and email
	if ip != "" {
		_, _ = s.repo.Queries().UpsertVerificationRateLimit(ctx, db.UpsertVerificationRateLimitParams{
			ID:         security.GenerateUUID(),
			Identifier: ip,
			Action:     "resend_email",
		})
	}
	if email != "" {
		_, _ = s.repo.Queries().UpsertVerificationRateLimit(ctx, db.UpsertVerificationRateLimitParams{
			ID:         security.GenerateUUID(),
			Identifier: email,
			Action:     "resend_email",
		})
	}

	return nil
}

// ResendVerificationEmail applies rate limiting/cooldown, invalidates older tokens, generates a fresh token, and dispatches an email.
func (s *AuthService) ResendVerificationEmail(ctx context.Context, email, ip string) (*ResendVerificationResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidEmail
	}

	// Check rate limits & cooldown
	if err := s.checkAndEnforceRateLimits(ctx, email, ip); err != nil {
		return nil, err
	}

	user, err := s.repo.Queries().GetUserByEmail(ctx, email)
	if err != nil || user.ID == "" {
		// Neutral response: Do not disclose whether email exists (anti-enumeration)
		return &ResendVerificationResponse{
			Message: "If an unverified account with this email exists, a new verification link has been sent.",
			Sent:    true,
		}, nil
	}

	if user.IsVerified && user.Status == "active" {
		return &ResendVerificationResponse{
			Message:         "This email address is already verified. Please proceed to login.",
			AlreadyVerified: true,
			Sent:            false,
		}, nil
	}

	// Generate fresh token
	rawToken, err := security.GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate verification token: %w", err)
	}
	tokenHash := security.HashToken(rawToken)

	// Atomically invalidate old tokens and create new token
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		if err := q.InvalidatePendingEmailVerificationTokens(ctx, user.ID); err != nil {
			return err
		}
		_, err := q.CreateEmailVerificationToken(ctx, db.CreateEmailVerificationTokenParams{
			ID:        security.GenerateUUID(),
			UserID:    user.ID,
			Email:     email,
			TokenHash: tokenHash,
			ExpiresAt: TimestamptzFromTime(time.Now().Add(24 * time.Hour)),
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store verification token: %w", err)
	}

	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.FrontendBaseURL, rawToken)

	deliveryErr := s.delivery.SendVerificationEmail(email, user.DisplayName, verificationURL)
	if deliveryErr != nil {
		return nil, deliveryErr
	}

	return &ResendVerificationResponse{
		Message: "A new verification link has been sent to your email address.",
		Sent:    true,
	}, nil
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
		user, err = s.repo.Queries().GetUserByEmail(ctx, email)
		// Check alias between example.org and org.ng for seamless demo logins
		if err != nil || user.ID == "" {
			if strings.HasSuffix(email, "@ratels.example.org") {
				altEmail := strings.TrimSuffix(email, "@ratels.example.org") + "@ratels.org.ng"
				user, err = s.repo.Queries().GetUserByEmail(ctx, altEmail)
			} else if strings.HasSuffix(email, "@ratels.org.ng") {
				altEmail := strings.TrimSuffix(email, "@ratels.org.ng") + "@ratels.example.org"
				user, err = s.repo.Queries().GetUserByEmail(ctx, altEmail)
			}
		}
	} else {
		// Clean phone input (strip spaces, dashes)
		cleanPhone := strings.ReplaceAll(strings.ReplaceAll(username, " ", ""), "-", "")
		user, err = s.repo.Queries().GetUserByPhone(ctx, cleanPhone)
		if (err != nil || user.ID == "") && strings.HasPrefix(cleanPhone, "+234") && len(cleanPhone) == 14 {
			// Try 0-prefixed local format
			localPhone := "0" + cleanPhone[4:]
			user, err = s.repo.Queries().GetUserByPhone(ctx, localPhone)
		} else if (err != nil || user.ID == "") && strings.HasPrefix(cleanPhone, "0") && len(cleanPhone) == 11 {
			// Try +234-prefixed international format
			intlPhone := "+234" + cleanPhone[1:]
			user, err = s.repo.Queries().GetUserByPhone(ctx, intlPhone)
		}
		// If phone lookup still didn't find the user, try membership reference
		if err != nil || user.ID == "" {
			user, err = s.repo.Queries().GetUserByMembershipReference(ctx, username)
		}
	}

	if err != nil || user.ID == "" {
		return nil, ErrInvalidCredentials
	}

	if !security.CheckPassword(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Contact verification and legacy account repair must never lift a restriction.
	if user.Status != "active" && user.Status != "pending_verification" {
		return nil, ErrAccountInactive
	}
	if (user.PrimaryRole == "super_admin" && (!user.IsVerified || user.Status != "active")) || (user.Status == "active" && !user.IsVerified) {
		if err := s.repo.WithTx(ctx, func(q *db.Queries) error { return q.ActivateVerifiedMember(ctx, user.ID) }); err != nil {
			return nil, ErrAccountInactive
		}
		user.Status = "active"
		user.IsVerified = true
	} else if user.Status == "pending_verification" || !user.IsVerified {
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

	// Clamp IP and User-Agent to prevent database column overflow
	safeIP := strings.TrimSpace(ip)
	if idx := strings.Index(safeIP, ","); idx != -1 {
		safeIP = strings.TrimSpace(safeIP[:idx])
	}
	if len(safeIP) > 45 {
		safeIP = safeIP[:45]
	}

	safeUA := strings.TrimSpace(userAgent)
	if len(safeUA) > 255 {
		safeUA = safeUA[:255]
	}

	_, err = s.repo.Queries().CreateUserSession(ctx, db.CreateUserSessionParams{
		ID:        security.GenerateUUID(),
		UserID:    user.ID,
		TokenJti:  jti,
		ExpiresAt: TimestamptzFromTime(expiresAt),
		IpAddress: TextFromString(safeIP),
		UserAgent: TextFromString(safeUA),
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
	return s.EditMemberProfile(ctx, userID, ProfileUpdate{HomeStateID: homeStateID, HomeLGAID: homeLGAID, Address: address, Bio: bio, AvatarURL: avatarURL})
}

// ListMembers returns a paginated list of members for administrative overview, appointments, and authorizations.
func (s *AuthService) ListMembers(ctx context.Context, page, limit int32) ([]UserSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	users, err := s.repo.Queries().ListUsers(ctx, db.ListUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	res := make([]UserSummary, 0, len(users))
	for _, u := range users {
		res = append(res, UserSummary{
			ID:                  u.ID,
			Email:               StringFromText(u.Email),
			Phone:               StringFromText(u.Phone),
			DisplayName:         u.DisplayName,
			MembershipReference: u.MembershipReference,
			Status:              u.Status,
			PrimaryRole:         u.PrimaryRole,
			IsVerified:          u.IsVerified,
		})
	}
	return res, nil
}

// SearchMembers returns users matching the search query (name, email, phone, reference) or paginated list if empty.
func (s *AuthService) SearchMembers(ctx context.Context, query string, page, limit int32) ([]UserSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	trimmed := strings.TrimSpace(query)
	users, err := s.repo.Queries().SearchUsers(ctx, db.SearchUsersParams{
		Column1: trimmed,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}

	res := make([]UserSummary, 0, len(users))
	for _, u := range users {
		res = append(res, UserSummary{
			ID:                  u.ID,
			Email:               StringFromText(u.Email),
			Phone:               StringFromText(u.Phone),
			DisplayName:         u.DisplayName,
			MembershipReference: u.MembershipReference,
			Status:              u.Status,
			PrimaryRole:         u.PrimaryRole,
			IsVerified:          u.IsVerified,
		})
	}
	return res, nil
}

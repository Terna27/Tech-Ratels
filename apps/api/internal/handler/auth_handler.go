package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"ratels-api/internal/config"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	cfg         *config.Config
}

func NewAuthHandler(authService *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		cfg:         cfg,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req service.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	res, err := h.authService.Register(r.Context(), req)
	if err != nil {
		if err == service.ErrPrivilegedRoleRefused {
			ErrorJSON(w, http.StatusForbidden, "ROLE_SELECTION_FORBIDDEN", err.Error(), nil)
			return
		}
		if err == service.ErrUserAlreadyExists {
			ErrorJSON(w, http.StatusConflict, "USER_EXISTS", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REGISTRATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, res)
}

func (h *AuthHandler) VerifyContact(w http.ResponseWriter, r *http.Request) {
	var req service.VerifyContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	ok, err := h.authService.VerifyContact(r.Context(), req)
	if err != nil || !ok {
		ErrorJSON(w, http.StatusBadRequest, "VERIFICATION_FAILED", "Invalid or expired verification code", nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"message":     "Contact verified successfully. Account is now active.",
		"is_verified": true,
	})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req service.VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	res, err := h.authService.VerifyEmailToken(r.Context(), req.Token)
	if err != nil {
		if err == service.ErrTokenExpired {
			ErrorJSON(w, http.StatusBadRequest, "TOKEN_EXPIRED", err.Error(), nil)
			return
		}
		if err == service.ErrTokenAlreadyUsed {
			ErrorJSON(w, http.StatusBadRequest, "TOKEN_ALREADY_USED", err.Error(), nil)
			return
		}
		if err == service.ErrInvalidToken {
			ErrorJSON(w, http.StatusBadRequest, "TOKEN_INVALID", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "VERIFICATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req service.ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	ip := r.RemoteAddr
	res, err := h.authService.ResendVerificationEmail(r.Context(), req.Email, ip)
	if err != nil {
		if errors.Is(err, service.ErrResendCooldownActive) {
			ErrorJSON(w, http.StatusTooManyRequests, "COOLDOWN_ACTIVE", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrRateLimitExceeded) {
			ErrorJSON(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInvalidEmail) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_EMAIL", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadGateway, "EMAIL_DELIVERY_FAILED", "Email delivery provider rejected message. Please check the address or try again.", nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

type ExtendedLoginResponse struct {
	service.LoginResponse
	CSRFToken string `json:"csrf_token"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req service.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	res, err := h.authService.Login(r.Context(), req, ip, ua)
	if err != nil {
		if err == service.ErrUserNotVerified {
			ErrorJSON(w, http.StatusForbidden, "USER_UNVERIFIED", err.Error(), nil)
			return
		}
		if err == service.ErrAccountInactive {
			ErrorJSON(w, http.StatusForbidden, "ACCOUNT_INACTIVE", err.Error(), nil)
			return
		}
		if err == service.ErrInvalidCredentials {
			ErrorJSON(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred during login. Please try again.", nil)
		return
	}

	csrfToken := security.GenerateUUID()
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || strings.ToLower(h.cfg.Environment) == "production"
	maxAge := h.cfg.JWTExpiryHours * 3600

	// Set HttpOnly browser session cookie (SameSite=Lax for same-origin proxy architecture)
	http.SetCookie(w, &http.Cookie{
		Name:     "ratels_session",
		Value:    res.AccessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  time.Now().Add(time.Duration(h.cfg.JWTExpiryHours) * time.Hour),
	})

	// Set non-HttpOnly CSRF token cookie for double-submit verification
	http.SetCookie(w, &http.Cookie{
		Name:     "ratels_csrf",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  time.Now().Add(time.Duration(h.cfg.JWTExpiryHours) * time.Hour),
	})

	JSON(w, http.StatusOK, ExtendedLoginResponse{
		LoginResponse: *res,
		CSRFToken:     csrfToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	jti := GetCurrentJTI(r.Context())
	if jti != "" {
		_ = h.authService.Logout(r.Context(), jti)
	}

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || strings.ToLower(h.cfg.Environment) == "production"

	// Invalidate session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "ratels_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	// Invalidate CSRF cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "ratels_csrf",
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	JSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

func (h *AuthHandler) GetCSRF(w http.ResponseWriter, r *http.Request) {
	csrfToken := security.GenerateUUID()
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || strings.ToLower(h.cfg.Environment) == "production"

	http.SetCookie(w, &http.Cookie{
		Name:     "ratels_csrf",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   h.cfg.JWTExpiryHours * 3600,
	})

	JSON(w, http.StatusOK, map[string]string{
		"csrf_token": csrfToken,
	})
}

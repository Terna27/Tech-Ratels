package handler

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"ratels-api/internal/config"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

type contextKey string

const (
	UserContextKey         contextKey = "auth_user"
	AppointmentsContextKey contextKey = "auth_appointments"
	JTIContextKey          contextKey = "auth_jti"
	AuthTypeContextKey     contextKey = "auth_type" // "bearer" or "cookie"
)

func AuthMiddleware(cfg *config.Config, repo *repository.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string
			var authType string

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				authType = "bearer"
			} else if cookie, err := r.Cookie("ratels_session"); err == nil && cookie.Value != "" {
				tokenStr = cookie.Value
				authType = "cookie"
			}

			if tokenStr == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := security.ParseJWT(tokenStr, cfg.JWTSecret)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Validate session in database (ensure not revoked and not expired)
			session, err := repo.Queries().GetUserSession(r.Context(), claims.JTI)
			if err != nil || session.ID == "" || session.IsRevoked || session.UserID != claims.UserID {
				next.ServeHTTP(w, r)
				return
			}

			// Fetch user (verifying account is active)
			user, err := repo.Queries().GetUserByID(r.Context(), claims.UserID)
			if err != nil || user.ID == "" || user.Status != "active" {
				next.ServeHTTP(w, r)
				return
			}

			// Fetch active appointments (query filters status='active' AND (end_date IS NULL OR end_date > now))
			appointments, _ := repo.Queries().ListActiveAppointmentsByUser(r.Context(), user.ID)

			// Double-check appointment start_date and end_date promptly
			now := time.Now()
			var validAppointments []db.Appointment
			for _, a := range appointments {
				if a.Status == "active" {
					if (!a.StartDate.Valid || a.StartDate.Time.Before(now) || a.StartDate.Time.Equal(now)) && (!a.EndDate.Valid || a.EndDate.Time.After(now)) {
						validAppointments = append(validAppointments, a)
					}
				}
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, AppointmentsContextKey, validAppointments)
			ctx = context.WithValue(ctx, JTIContextKey, claims.JTI)
			ctx = context.WithValue(ctx, AuthTypeContextKey, authType)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CSRFProtectionMiddleware enforces CSRF validation for cookie-authenticated mutating requests.
func CSRFProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		isMutating := method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch

		authType, _ := r.Context().Value(AuthTypeContextKey).(string)

		// CSRF protection is strictly required for cookie-authenticated mutating requests
		if isMutating && authType == "cookie" && r.URL.Path != "/api/v1/marketplace/subscriptions/webhooks/paystack" && r.URL.Path != "/api/v1/marketplace/webhooks/paystack" {
			csrfCookie, err := r.Cookie("ratels_csrf")
			if err != nil || csrfCookie.Value == "" {
				ErrorJSON(w, http.StatusForbidden, "CSRF_TOKEN_MISSING", "CSRF token cookie missing", nil)
				return
			}

			csrfHeader := r.Header.Get("X-CSRF-Token")
			if csrfHeader == "" {
				ErrorJSON(w, http.StatusForbidden, "CSRF_TOKEN_REQUIRED", "X-CSRF-Token header required for cookie-authenticated mutations", nil)
				return
			}

			if subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(csrfHeader)) != 1 {
				ErrorJSON(w, http.StatusForbidden, "CSRF_TOKEN_INVALID", "Invalid CSRF token", nil)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(UserContextKey).(db.User)
		if !ok || user.ID == "" {
			ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(UserContextKey).(db.User)
			if !ok || user.ID == "" {
				ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
				return
			}

			userRoles := map[string]bool{
				user.PrimaryRole: true,
			}

			if apps, ok := r.Context().Value(AppointmentsContextKey).([]db.Appointment); ok {
				for _, a := range apps {
					if a.Status == "active" {
						userRoles[a.RoleType] = true
					}
				}
			}

			// Super Admin always passes any administrative or role gate
			if userRoles["super_admin"] {
				next.ServeHTTP(w, r)
				return
			}

			for _, role := range allowedRoles {
				if userRoles[role] {
					next.ServeHTTP(w, r)
					return
				}
			}

			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions for this action", nil)
		})
	}
}

func GetUserEffectiveRole(ctx context.Context) string {
	user := GetCurrentUser(ctx)
	if user == nil {
		return ""
	}
	if user.PrimaryRole == "super_admin" {
		return "super_admin"
	}
	apps := GetCurrentAppointments(ctx)
	for _, a := range apps {
		if a.Status == "active" && a.RoleType == "super_admin" {
			return "super_admin"
		}
	}
	if user.PrimaryRole == "national_admin" {
		return "national_admin"
	}
	for _, a := range apps {
		if a.Status == "active" && a.RoleType == "national_admin" {
			return "national_admin"
		}
	}
	for _, a := range apps {
		if a.Status == "active" {
			return a.RoleType
		}
	}
	return user.PrimaryRole
}

func GetCurrentUser(ctx context.Context) *db.User {
	if u, ok := ctx.Value(UserContextKey).(db.User); ok && u.ID != "" {
		return &u
	}
	return nil
}

func GetCurrentAppointments(ctx context.Context) []db.Appointment {
	if apps, ok := ctx.Value(AppointmentsContextKey).([]db.Appointment); ok {
		return apps
	}
	return nil
}

func GetCurrentJTI(ctx context.Context) string {
	if jti, ok := ctx.Value(JTIContextKey).(string); ok {
		return jti
	}
	return ""
}

func GetAuthType(ctx context.Context) string {
	if at, ok := ctx.Value(AuthTypeContextKey).(string); ok {
		return at
	}
	return ""
}

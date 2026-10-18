package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"ratels-api/internal/config"
	"ratels-api/internal/repository"
)

type RouterParams struct {
	Config               *config.Config
	Repo                 *repository.Repository
	HealthHandler        *HealthHandler
	AuthHandler          *AuthHandler
	MemberHandler        *MemberHandler
	GeographyHandler     *GeographyHandler
	LeadershipHandler    *LeadershipHandler
	ActivityHandler      *ActivityHandler
	ParticipationHandler *ParticipationHandler
	FoundationHandler    *FoundationHandler
	AdminHandler         *AdminHandler
	FinanceHandler       *FinanceHandler
}

func NewRouter(p RouterParams) http.Handler {
	r := chi.NewRouter()

	// Base middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:8000", "http://127.0.0.1:8000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Auth token & cookie extractor middleware
	r.Use(AuthMiddleware(p.Config, p.Repo))

	// CSRF protection for cookie-authenticated mutating requests
	r.Use(CSRFProtectionMiddleware)

	// Root health checks
	r.Get("/health", p.HealthHandler.HealthCheck)

	// API v1 routes
	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/health", p.HealthHandler.HealthCheck)

		// Auth
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", p.AuthHandler.Register)
			auth.Post("/verify-contact", p.AuthHandler.VerifyContact)
			auth.Post("/login", p.AuthHandler.Login)
			auth.Get("/csrf", p.AuthHandler.GetCSRF)
			auth.With(RequireAuth).Post("/logout", p.AuthHandler.Logout)
		})

		// Members
		api.Route("/members", func(members chi.Router) {
			members.With(RequireAuth).Get("/me", p.MemberHandler.GetMe)
			members.With(RequireAuth).Put("/me/profile", p.MemberHandler.UpdateProfile)
		})

		// Geography
		api.Route("/geography", func(geo chi.Router) {
			geo.Get("/zones", p.GeographyHandler.ListZones)
			geo.Get("/states", p.GeographyHandler.ListStates)
			geo.Get("/states/{id}/lgas", p.GeographyHandler.ListLGAsByState)
		})

		// Leadership
		api.Route("/leadership", func(lead chi.Router) {
			lead.Get("/directory", p.LeadershipHandler.ListDirectory)
			lead.With(RequireRole("national_admin", "super_admin")).Get("/appointments", p.LeadershipHandler.ListAppointments)
			lead.With(RequireRole("national_admin", "super_admin")).Post("/appointments", p.LeadershipHandler.CreateAppointment)
			lead.With(RequireRole("super_admin")).Post("/appointments/{id}/replace", p.LeadershipHandler.ReplaceAppointment)
			lead.With(RequireRole("super_admin")).Patch("/appointments/{id}/scope-dates", p.LeadershipHandler.UpdateScopeDates)
			lead.With(RequireRole("super_admin")).Put("/appointments/{id}/scope-dates", p.LeadershipHandler.UpdateScopeDates)
			lead.With(RequireRole("super_admin")).Post("/appointments/{id}/suspend", p.LeadershipHandler.SuspendAppointment)
			lead.With(RequireRole("super_admin")).Post("/appointments/{id}/revoke", p.LeadershipHandler.RevokeAppointment)
			lead.With(RequireRole("national_admin", "super_admin")).Get("/appointments/{id}/history", p.LeadershipHandler.ListAppointmentHistory)
		})

		// Financial Records & Fund Movements
		api.Route("/finance", func(fin chi.Router) {
			fin.With(RequireAuth).Get("/funds", p.FinanceHandler.ListFunds)
			fin.With(RequireAuth).Get("/transactions", p.FinanceHandler.ListTransactions)
			fin.With(RequireAuth).Get("/transactions/{id}", p.FinanceHandler.GetTransaction)
			fin.With(RequireAuth).Post("/transactions", p.FinanceHandler.RecordTransaction)
			fin.With(RequireRole("super_admin")).Post("/transactions/{id}/approve", p.FinanceHandler.ApproveTransaction)
			fin.With(RequireRole("super_admin")).Post("/transactions/{id}/reverse", p.FinanceHandler.ReverseTransaction)
			fin.With(RequireAuth).Get("/statement", p.FinanceHandler.GenerateStatement)
			fin.With(RequireAuth).Get("/statements", p.FinanceHandler.GenerateStatement)
			fin.With(RequireAuth).Get("/export", p.FinanceHandler.ExportCSV)
			fin.With(RequireAuth).Get("/export/csv", p.FinanceHandler.ExportCSV)
			fin.With(RequireRole("super_admin")).Get("/authorizations", p.FinanceHandler.ListAuthorizations)
			fin.With(RequireRole("super_admin")).Post("/authorizations", p.FinanceHandler.AuthorizeStaff)
			fin.With(RequireRole("super_admin")).Post("/authorizations/{id}/revoke", p.FinanceHandler.RevokeStaff)
			fin.With(RequireRole("super_admin")).Delete("/authorizations/{id}", p.FinanceHandler.RevokeStaff)
		})

		// Activities & Cleanups
		api.Route("/activities", func(act chi.Router) {
			act.Get("/campaigns/current", p.ActivityHandler.GetCurrentCampaign)
			act.Get("/events", p.ActivityHandler.ListEvents)
			act.Get("/events/{id}", p.ActivityHandler.GetEvent)

			// Venue & Event mutations
			act.With(RequireRole("national_admin", "state_coordinator", "lga_representative")).Post("/events", p.ActivityHandler.CreateEvent)
			act.With(RequireRole("national_admin", "state_coordinator")).Post("/events/{id}/publish", p.ActivityHandler.PublishEvent)
			act.With(RequireRole("national_admin", "state_coordinator")).Post("/events/{id}/reschedule", p.ActivityHandler.RescheduleEvent)

			// Registration
			act.With(RequireAuth).Post("/events/{id}/register", p.ActivityHandler.RegisterForEvent)
			act.With(RequireAuth).Post("/events/{id}/cancel-registration", p.ActivityHandler.CancelRegistration)
		})

		// Participations & Badges
		api.Route("/participations", func(part chi.Router) {
			part.With(RequireAuth).Post("/claims", p.ParticipationHandler.SubmitClaim)
			part.With(RequireRole("national_admin", "state_coordinator")).Get("/review-queue", p.ParticipationHandler.ListReviewQueue)
			part.With(RequireRole("national_admin", "state_coordinator")).Post("/claims/{id}/review", p.ParticipationHandler.ReviewClaim)
			part.With(RequireAuth).Post("/claims/{id}/appeal", p.ParticipationHandler.SubmitAppeal)
			part.With(RequireAuth).Get("/my-badges", p.ParticipationHandler.ListMyBadges)
		})

		// Foundation
		api.Route("/foundation", func(found chi.Router) {
			found.Get("/", p.FoundationHandler.GetPublicInfo)
			found.With(RequireRole("national_admin")).Get("/bank-accounts", p.FoundationHandler.ListBankAccounts)
			found.With(RequireRole("national_admin")).Post("/bank-accounts", p.FoundationHandler.CreateBankAccount)
			found.With(RequireRole("national_admin")).Post("/bank-accounts/{id}/verify", p.FoundationHandler.VerifyBankAccount)
		})

		// Admin Workspace
		api.Route("/admin", func(adm chi.Router) {
			adm.With(RequireRole("national_admin", "state_coordinator", "zonal_coordinator", "lga_representative")).Get("/overview", p.AdminHandler.GetOverview)
			adm.With(RequireRole("national_admin")).Get("/audit-logs", p.AdminHandler.ListAuditLogs)
		})
	})

	return r
}

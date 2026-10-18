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
	InventoryHandler     *InventoryHandler
	MarketplaceHandler   *MarketplaceHandler
	CareersHandler       *CareersHandler
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
		AllowedOrigins:   p.Config.AllowedOrigins,
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
			auth.Post("/verify-email", p.AuthHandler.VerifyEmail)
			auth.Post("/resend-verification", p.AuthHandler.ResendVerification)
			auth.Post("/login", p.AuthHandler.Login)
			auth.Get("/csrf", p.AuthHandler.GetCSRF)
			auth.With(RequireAuth).Post("/logout", p.AuthHandler.Logout)
		})

		// Members
		api.Route("/members", func(members chi.Router) {
			members.With(RequireAuth).Get("/me", p.MemberHandler.GetMe)
			members.With(RequireAuth).Put("/me/profile", p.MemberHandler.UpdateProfile)
			members.With(RequireRole("national_admin", "super_admin")).Get("/", p.MemberHandler.ListMembers)
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

		// Inventory & Material Distribution (Blueprint v2.0 Section 05)
		api.Route("/inventory", func(inv chi.Router) {
			// Catalogue
			inv.With(RequireAuth).Get("/items", p.InventoryHandler.ListItems)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Post("/items", p.InventoryHandler.CreateItem)
			inv.With(RequireAuth).Get("/items/{id}", p.InventoryHandler.GetItem)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Put("/items/{id}", p.InventoryHandler.UpdateItem)

			// Storage Locations (Warehouses)
			inv.With(RequireAuth).Get("/locations", p.InventoryHandler.ListLocations)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Post("/locations", p.InventoryHandler.CreateLocation)
			inv.With(RequireAuth).Get("/locations/{id}", p.InventoryHandler.GetLocation)

			// Balances & Initial Receipts
			inv.With(RequireAuth).Get("/balances", p.InventoryHandler.ListBalances)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Post("/balances/receive", p.InventoryHandler.ReceiveStock)
			inv.With(RequireAuth).Get("/balances/low-stock", p.InventoryHandler.ListLowStockAlerts)

			// Material Requests
			inv.With(RequireAuth).Get("/requests", p.InventoryHandler.ListRequests)
			inv.With(RequireAuth).Post("/requests", p.InventoryHandler.CreateRequest)
			inv.With(RequireAuth).Get("/requests/{id}", p.InventoryHandler.GetRequest)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator", "lga_representative")).Post("/requests/{id}/review", p.InventoryHandler.ReviewRequest)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Post("/requests/{id}/allocate", p.InventoryHandler.AllocateRequest)

			// Transfers & Dispatches
			inv.With(RequireAuth).Get("/transfers", p.InventoryHandler.ListTransfers)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Post("/transfers", p.InventoryHandler.CreateTransfer)
			inv.With(RequireAuth).Get("/transfers/{id}", p.InventoryHandler.GetTransfer)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Post("/transfers/{id}/dispatch", p.InventoryHandler.DispatchTransfer)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator", "lga_representative")).Post("/transfers/{id}/receive", p.InventoryHandler.ConfirmReceipt)

			// Discrepancies
			inv.With(RequireAuth).Get("/discrepancies", p.InventoryHandler.ListDiscrepancies)
			inv.With(RequireAuth).Get("/discrepancies/{id}", p.InventoryHandler.GetDiscrepancy)
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Post("/discrepancies/{id}/resolve", p.InventoryHandler.ResolveDiscrepancy)

			// Stock Ledger & Reporting
			inv.With(RequireRole("national_admin", "zonal_coordinator", "state_coordinator")).Get("/ledger", p.InventoryHandler.ListStockLedger)
			inv.With(RequireAuth).Get("/summary", p.InventoryHandler.GetSummary)
		})

		// Ratels Marketplace (Blueprint v2.0 Section 07)
		api.Route("/marketplace", func(mkt chi.Router) {
			// Public Storefront
			mkt.Get("/checkout/status", p.MarketplaceHandler.GetCheckoutStatus)
			mkt.Get("/products", p.MarketplaceHandler.ListPublicProducts)
			mkt.Get("/products/{id}", p.MarketplaceHandler.GetProduct)
			mkt.Get("/products/slug/{slug}", p.MarketplaceHandler.GetProductBySlug)
			mkt.Get("/businesses", p.MarketplaceHandler.ListBusinesses)
			mkt.Get("/businesses/{id}", p.MarketplaceHandler.GetBusiness)
			mkt.Get("/businesses/{id}/branches", p.MarketplaceHandler.ListBusinessBranches)
			mkt.Post("/cart/validate", p.MarketplaceHandler.ValidateCart)
			mkt.Post("/checkout", p.MarketplaceHandler.Checkout)
			mkt.Get("/orders/lookup/{orderNumber}", p.MarketplaceHandler.LookupOrder)
			mkt.Post("/payments/verify", p.MarketplaceHandler.VerifyPayment)
			mkt.Post("/webhooks/paystack", p.MarketplaceHandler.PaystackWebhook)

			// Authenticated Buyer Orders
			mkt.With(RequireAuth).Get("/orders", p.MarketplaceHandler.ListMyOrders)
			mkt.With(RequireAuth).Get("/orders/{id}", p.MarketplaceHandler.GetOrder)
			mkt.With(RequireAuth).Post("/orders/{id}/cancel", p.MarketplaceHandler.CancelOrder)
			mkt.With(RequireAuth).Post("/orders/{id}/confirm-delivery", p.MarketplaceHandler.ConfirmDelivery)
			mkt.With(RequireAuth).Post("/orders/{id}/dispute", p.MarketplaceHandler.RaiseDispute)

			// Seller / Vendor Management
			mkt.Route("/seller", func(seller chi.Router) {
				seller.Use(RequireAuth)
				seller.Post("/business", p.MarketplaceHandler.RegisterBusiness)
				seller.Get("/business", p.MarketplaceHandler.GetMyBusiness)
				seller.Put("/business", p.MarketplaceHandler.UpdateMyBusiness)
				seller.Post("/branches", p.MarketplaceHandler.CreateBranch)
				seller.Get("/products", p.MarketplaceHandler.ListMyProducts)
				seller.Post("/products", p.MarketplaceHandler.CreateProduct)
				seller.Put("/products/{id}", p.MarketplaceHandler.UpdateProduct)
				seller.Delete("/products/{id}", p.MarketplaceHandler.DeleteProduct)
				seller.Post("/products/images/presign", p.MarketplaceHandler.AuthorizeImageUpload)
				seller.Post("/products/images/upload", p.MarketplaceHandler.UploadProductImage)
				seller.Delete("/products/images", p.MarketplaceHandler.DeleteProductImage)
				seller.Get("/orders", p.MarketplaceHandler.ListSellerOrders)
				seller.Post("/orders/{id}/dispatch", p.MarketplaceHandler.DispatchOrder)
				seller.Post("/orders/{id}/delivered", p.MarketplaceHandler.ConfirmDelivery)
				seller.Get("/metrics", p.MarketplaceHandler.GetSellerMetrics)
			})

			// Dev sandbox mock presigned upload endpoint
			mkt.Put("/uploads/mock-presign", p.MarketplaceHandler.MockDevUpload)
			mkt.Post("/uploads/mock-presign", p.MarketplaceHandler.MockDevUpload)

			// Marketplace Administration Workspace
			mkt.Route("/admin", func(mktAdm chi.Router) {
				mktAdm.Use(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator"))
				mktAdm.Get("/overview", p.MarketplaceHandler.GetAdminOverview)
				mktAdm.Get("/businesses", p.MarketplaceHandler.ListAllBusinessesAdmin)
				mktAdm.Post("/businesses/{id}/approve", p.MarketplaceHandler.ApproveBusiness)
				mktAdm.Get("/products", p.MarketplaceHandler.ListAllProductsAdmin)
				mktAdm.Put("/products/{id}/status", p.MarketplaceHandler.ModerateProduct)
				mktAdm.Get("/orders", p.MarketplaceHandler.ListAllOrdersAdmin)
				mktAdm.Post("/orders/{id}/cancel", p.MarketplaceHandler.CancelOrder)
				mktAdm.Post("/orders/{id}/refund", p.MarketplaceHandler.AdminRefundOrder)
				mktAdm.Get("/payments", p.MarketplaceHandler.ListPaymentsAdmin)
				mktAdm.Get("/disputes", p.MarketplaceHandler.ListDisputesAdmin)
				mktAdm.Post("/disputes/{id}/resolve", p.MarketplaceHandler.ResolveDispute)
			})
		})

		// Ratels Careers (Blueprint v2.0 Section 09)
		api.Route("/careers", func(car chi.Router) {
			// Public Job Discovery & Employers
			car.Get("/jobs", p.CareersHandler.ListPublicJobs)
			car.Get("/jobs/{id}", p.CareersHandler.GetPublicJob)
			car.Get("/employers/{id}", p.CareersHandler.GetEmployer)

			// Candidate Portal (Authenticated Members)
			car.With(RequireAuth).Get("/candidate/profile", p.CareersHandler.GetMyProfile)
			car.With(RequireAuth).Put("/candidate/profile", p.CareersHandler.UpsertMyProfile)
			car.With(RequireAuth).Post("/jobs/{id}/apply", p.CareersHandler.ApplyForJob)
			car.With(RequireAuth).Get("/applications", p.CareersHandler.ListMyApplications)
			car.With(RequireAuth).Get("/applications/{id}", p.CareersHandler.GetApplication)
			car.With(RequireAuth).Post("/applications/{id}/withdraw", p.CareersHandler.WithdrawApplication)
			car.With(RequireAuth).Get("/applications/{id}/history", p.CareersHandler.GetApplicationHistory)
			car.With(RequireAuth).Get("/interviews", p.CareersHandler.ListMyInterviews)

			// Employer Portal (Recruiters)
			car.Route("/employer", func(emp chi.Router) {
				emp.Use(RequireAuth)
				emp.Post("/register", p.CareersHandler.RegisterEmployer)
				emp.Get("/me", p.CareersHandler.GetMyEmployer)
				emp.Put("/{id}", p.CareersHandler.UpdateEmployer)
				emp.Get("/{id}/overview", p.CareersHandler.GetEmployerOverview)
				emp.Get("/{id}/recruiters", p.CareersHandler.ListRecruiters)
				emp.Post("/{id}/recruiters", p.CareersHandler.AddRecruiter)
				emp.Get("/{id}/jobs", p.CareersHandler.ListEmployerJobs)
				emp.Post("/{id}/jobs", p.CareersHandler.CreateJob)
				emp.Put("/jobs/{jobId}", p.CareersHandler.UpdateJob)
				emp.Patch("/jobs/{jobId}/status", p.CareersHandler.UpdateJobStatus)
				emp.Get("/jobs/{jobId}/applications", p.CareersHandler.ListJobApplications)
				emp.Get("/applications/{appId}/candidate-profile", p.CareersHandler.GetApplicantProfile)
				emp.Patch("/applications/{appId}/status", p.CareersHandler.UpdateApplicationStatus)
				emp.Post("/applications/{appId}/interviews", p.CareersHandler.ScheduleInterview)
				emp.Get("/{id}/interviews", p.CareersHandler.ListEmployerInterviews)
			})

			// Admin Careers Workspace
			car.Route("/admin", func(carAdm chi.Router) {
				carAdm.Use(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator"))
				carAdm.Get("/overview", p.CareersHandler.GetAdminOverview)
				carAdm.Get("/employers", p.CareersHandler.ListEmployersAdmin)
				carAdm.Post("/employers/{id}/verify", p.CareersHandler.VerifyEmployer)
				carAdm.Get("/jobs", p.CareersHandler.ListJobsAdmin)
				carAdm.Post("/jobs/{id}/moderate", p.CareersHandler.ModerateJob)
				carAdm.Get("/applications", p.CareersHandler.ListApplicationsAdmin)
			})
		})

		// Admin Workspace
		api.Route("/admin", func(adm chi.Router) {
			adm.With(RequireRole("national_admin", "state_coordinator", "zonal_coordinator", "lga_representative")).Get("/overview", p.AdminHandler.GetOverview)
			adm.With(RequireRole("national_admin")).Get("/audit-logs", p.AdminHandler.ListAuditLogs)
		})
	})

	return r
}

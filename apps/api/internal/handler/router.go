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
	Config                  *config.Config
	Repo                    *repository.Repository
	HealthHandler           *HealthHandler
	AuthHandler             *AuthHandler
	MemberHandler           *MemberHandler
	GeographyHandler        *GeographyHandler
	LeadershipHandler       *LeadershipHandler
	ActivityHandler         *ActivityHandler
	ParticipationHandler    *ParticipationHandler
	FoundationHandler       *FoundationHandler
	AdminHandler            *AdminHandler
	FinanceHandler          *FinanceHandler
	InventoryHandler        *InventoryHandler
	MarketplaceHandler      *MarketplaceHandler
	CareersHandler          *CareersHandler
	HeroMediaHandler        *HeroMediaHandler
	CommunityProjectHandler *CommunityProjectHandler
	GalleryHandler          *GalleryHandler
	NotificationHandler     *NotificationHandler
	SafeguardingHandler     *SafeguardingHandler
	ReportHandler           *ReportHandler
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
			// Dashboard & Summary
			fin.With(RequireAuth).Get("/dashboard/summary", p.FinanceHandler.GetDashboardSummary)
			fin.With(RequireAuth).Get("/overview", p.FinanceHandler.GetDashboardSummary)

			// Funds & Core Ledger
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

			// Income Management
			fin.With(RequireAuth).Post("/income", p.FinanceHandler.RecordIncome)
			fin.With(RequireAuth).Post("/income/{id}/verify", p.FinanceHandler.VerifyIncome)

			// Expense Management
			fin.With(RequireAuth).Get("/expenses", p.FinanceHandler.ListExpenses)
			fin.With(RequireAuth).Get("/expenses/{id}", p.FinanceHandler.GetExpense)
			fin.With(RequireAuth).Post("/expenses", p.FinanceHandler.CreateExpense)
			fin.With(RequireAuth).Post("/expenses/{id}/review", p.FinanceHandler.ReviewExpense)
			fin.With(RequireAuth).Post("/expenses/{id}/authorize", p.FinanceHandler.AuthorizeExpense)
			fin.With(RequireAuth).Post("/expenses/{id}/pay", p.FinanceHandler.RecordExpensePayment)

			// Budget Management
			fin.With(RequireAuth).Get("/budgets", p.FinanceHandler.ListBudgets)
			fin.With(RequireAuth).Get("/budgets/{id}", p.FinanceHandler.GetBudget)
			fin.With(RequireAuth).Post("/budgets", p.FinanceHandler.CreateBudget)
			fin.With(RequireRole("super_admin")).Post("/budgets/{id}/approve", p.FinanceHandler.ApproveBudget)
			fin.With(RequireAuth).Post("/budgets/{id}/revisions", p.FinanceHandler.RequestBudgetRevision)
			fin.With(RequireRole("super_admin")).Post("/budgets/revisions/{id}/approve", p.FinanceHandler.ApproveBudgetRevision)

			// Allocations & Transfers
			fin.With(RequireAuth).Get("/allocations", p.FinanceHandler.ListAllocations)
			fin.With(RequireAuth).Post("/allocations", p.FinanceHandler.CreateAllocation)

			// Donations & Public Transparency
			fin.Get("/donations", p.FinanceHandler.ListDonations)
			fin.Get("/donations/{id}", p.FinanceHandler.GetDonation)
			fin.Post("/donations/initiate", p.FinanceHandler.InitiateDonation)
			fin.Post("/donations/verify", p.FinanceHandler.VerifyDonation)
			fin.Get("/transparency", p.FinanceHandler.GetPublicTransparencySummary)
			fin.Get("/transparency/summary", p.FinanceHandler.GetPublicTransparencySummary)

			// Bank Accounts
			fin.Get("/bank-accounts", p.FinanceHandler.ListBankAccounts)
			fin.With(RequireRole("super_admin")).Post("/bank-accounts", p.FinanceHandler.CreateBankAccount)
			fin.With(RequireRole("super_admin")).Post("/bank-accounts/{id}/activate", p.FinanceHandler.ActivateBankAccount)

			// Reconciliation
			fin.With(RequireAuth).Get("/reconciliation", p.FinanceHandler.ListReconciliationPeriods)
			fin.With(RequireAuth).Get("/reconciliation/periods", p.FinanceHandler.ListReconciliationPeriods)
			fin.With(RequireAuth).Get("/reconciliation/periods/{id}", p.FinanceHandler.GetReconciliationPeriod)
			fin.With(RequireAuth).Post("/reconciliation/periods", p.FinanceHandler.CreateReconciliationPeriod)
			fin.With(RequireAuth).Post("/reconciliation/periods/{id}/import-csv", p.FinanceHandler.ImportBankStatementCSV)
			fin.With(RequireAuth).Post("/reconciliation/periods/{id}/auto-match", p.FinanceHandler.AutoMatchStatement)
			fin.With(RequireAuth).Post("/reconciliation/statement-items/{id}/match", p.FinanceHandler.ManualMatchStatementItem)
			fin.With(RequireRole("super_admin")).Post("/reconciliation/periods/{id}/certify", p.FinanceHandler.CertifyReconciliation)

			// Official Printable Documents
			fin.With(RequireAuth).Get("/documents/{type}/{id}", p.FinanceHandler.GetFinancialDocument)
			fin.With(RequireAuth).Get("/documents/{type}/{id}/print", p.FinanceHandler.GetDocumentPrintableHTML)
		})

		// Activities & Cleanups
		api.Route("/activities", func(act chi.Router) {
			act.Get("/campaigns/current", p.ActivityHandler.GetCurrentCampaign)
			act.Get("/events", p.ActivityHandler.ListEvents)
			act.Get("/events/{id}", p.ActivityHandler.GetEvent)
			act.Get("/events/{id}/gallery", p.GalleryHandler.ListEventGallery)

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
			mkt.With(RequireAuth).Get("/orders/{id}/shipments", p.MarketplaceHandler.ListOrderShipments)
			mkt.With(RequireAuth).Post("/orders/{id}/cancel", p.MarketplaceHandler.CancelOrder)
			mkt.With(RequireAuth).Post("/orders/{id}/confirm-delivery", p.MarketplaceHandler.ConfirmDelivery)
			mkt.With(RequireAuth).Post("/shipments/{id}/confirm-delivery", p.MarketplaceHandler.ConfirmShipmentDelivery)
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
				seller.Get("/shipments", p.MarketplaceHandler.ListSellerShipments)
				seller.Get("/shipments/{id}", p.MarketplaceHandler.GetShipment)
				seller.Post("/shipments/{id}/dispatch", p.MarketplaceHandler.DispatchShipment)
				seller.Put("/shipments/{id}/tracking", p.MarketplaceHandler.UpdateShipmentTracking)
				seller.Post("/shipments/{id}/delivered", p.MarketplaceHandler.ConfirmShipmentDelivery)
				seller.Get("/metrics", p.MarketplaceHandler.GetSellerMetrics)
				seller.Post("/products/{id}/adjust-stock", p.MarketplaceHandler.AdjustProductStock)
				seller.Get("/products/{id}/inventory-logs", p.MarketplaceHandler.ListProductInventoryLogs)
				seller.Get("/inventory-logs", p.MarketplaceHandler.ListBusinessInventoryLogs)
			})

			// Dev sandbox mock presigned upload endpoint
			mkt.Put("/uploads/mock-presign", p.MarketplaceHandler.MockDevUpload)
			mkt.Post("/uploads/mock-presign", p.MarketplaceHandler.MockDevUpload)

			// Marketplace Administration Workspace
			mkt.Route("/admin", func(mktAdm chi.Router) {
				mktAdm.Use(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator", "lga_coordinator", "lga_representative"))
				mktAdm.Get("/overview", p.MarketplaceHandler.GetAdminOverview)
				mktAdm.Get("/businesses", p.MarketplaceHandler.ListAllBusinessesAdmin)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Post("/businesses/{id}/approve", p.MarketplaceHandler.ApproveBusiness)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Post("/businesses/{id}/suspend", p.MarketplaceHandler.SuspendBusiness)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Post("/businesses/{id}/restore", p.MarketplaceHandler.RestoreBusiness)
				mktAdm.Get("/products", p.MarketplaceHandler.ListAllProductsAdmin)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Put("/products/{id}/status", p.MarketplaceHandler.ModerateProduct)
				mktAdm.Get("/orders", p.MarketplaceHandler.ListAllOrdersAdmin)
				mktAdm.Get("/orders/{id}/shipments", p.MarketplaceHandler.ListOrderShipments)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Post("/orders/{id}/cancel", p.MarketplaceHandler.CancelOrder)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Post("/orders/{id}/refund", p.MarketplaceHandler.AdminRefundOrder)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Post("/refunds/{refundId}/confirm", p.MarketplaceHandler.AdminConfirmRefund)
				mktAdm.With(RequireRole("super_admin")).Post("/refunds/{refundId}/manual-settle", p.MarketplaceHandler.AdminManualSettleRefund)
				mktAdm.Get("/payments", p.MarketplaceHandler.ListPaymentsAdmin)
				mktAdm.Get("/shipments", p.MarketplaceHandler.ListAdminShipments)
				mktAdm.Get("/shipments/{id}", p.MarketplaceHandler.GetShipment)
				mktAdm.Post("/shipments/{id}/dispatch", p.MarketplaceHandler.DispatchShipment)
				mktAdm.Put("/shipments/{id}/tracking", p.MarketplaceHandler.UpdateShipmentTracking)
				mktAdm.Post("/shipments/{id}/delivered", p.MarketplaceHandler.ConfirmShipmentDelivery)
				mktAdm.Get("/disputes", p.MarketplaceHandler.ListDisputesAdmin)
				mktAdm.With(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator")).Post("/disputes/{id}/resolve", p.MarketplaceHandler.ResolveDispute)
				mktAdm.With(RequireRole("national_admin", "super_admin")).Post("/inventory/expire-stale-orders", p.MarketplaceHandler.AdminTriggerOrderExpiry)
				mktAdm.With(RequireRole("national_admin", "super_admin")).Post("/refunds/retry-pending", p.MarketplaceHandler.AdminTriggerRefundRetry)
				mktAdm.Get("/products/{id}/inventory-logs", p.MarketplaceHandler.ListProductInventoryLogs)
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

		// Homepage Media (Public)
		api.Route("/homepage", func(home chi.Router) {
			home.Get("/hero-media", p.HeroMediaHandler.GetActiveHeroMedia)
		})

		// Community Projects & Requests (Blueprint Section 10)
		api.Route("/community", func(comm chi.Router) {
			// Public project discovery & details
			comm.Get("/projects", p.CommunityProjectHandler.ListPublicProjects)
			comm.Get("/projects/{id}", p.CommunityProjectHandler.GetPublicProject)
			comm.Get("/projects/categories", p.CommunityProjectHandler.GetCategories)
			comm.Get("/projects/stats", p.CommunityProjectHandler.GetOverviewStats)

			// Member request submissions & tracking
			comm.Route("/requests", func(reqRouter chi.Router) {
				reqRouter.Use(RequireAuth)
				reqRouter.Post("/", p.CommunityProjectHandler.CreateCommunityRequest)
				reqRouter.Get("/mine", p.CommunityProjectHandler.ListMyCommunityRequests)
				reqRouter.Get("/{id}", p.CommunityProjectHandler.GetCommunityRequest)
				reqRouter.Post("/{id}/resubmit", p.CommunityProjectHandler.ResubmitCommunityRequest)
				reqRouter.Post("/presign", p.CommunityProjectHandler.PresignRequestUpload)
				reqRouter.Post("/upload", p.CommunityProjectHandler.UploadRequestImageDirect)
			})
		})

		// Event Media Gallery & Moderation System (Phase A)
		api.Route("/gallery", func(gal chi.Router) {
			gal.Get("/", p.GalleryHandler.ListPublicGallery)
			gal.Get("/featured", p.GalleryHandler.ListFeatured)
			gal.Get("/{id}", p.GalleryHandler.GetPublicMediaDetail)
			gal.Post("/{id}/report", p.GalleryHandler.ReportMedia)
			gal.Post("/{id}/takedown", p.GalleryHandler.SubmitTakedown)

			// Authenticated member gallery actions
			gal.With(RequireAuth).Get("/my-submissions", p.GalleryHandler.ListMySubmissions)
			gal.With(RequireAuth).Post("/presign", p.GalleryHandler.AuthorizeUpload)
			gal.With(RequireAuth).Post("/submit", p.GalleryHandler.SubmitMedia)
			gal.With(RequireAuth).Post("/upload", p.GalleryHandler.SubmitDirectUpload)
			gal.With(RequireAuth).Post("/{id}/withdraw", p.GalleryHandler.WithdrawSubmission)
		})

		// Member gallery shortcuts (/me/gallery)
		api.Route("/me/gallery", func(meGal chi.Router) {
			meGal.Use(RequireAuth)
			meGal.Get("/", p.GalleryHandler.ListMySubmissions)
			meGal.Post("/presign", p.GalleryHandler.AuthorizeUpload)
			meGal.Post("/submit", p.GalleryHandler.SubmitMedia)
			meGal.Post("/upload", p.GalleryHandler.SubmitDirectUpload)
			meGal.Post("/{id}/withdraw", p.GalleryHandler.WithdrawSubmission)
			meGal.Post("/{id}/takedown", p.GalleryHandler.SubmitTakedown)
		})

		// Member notifications & preferences
		api.Route("/me/notifications", func(meNotif chi.Router) {
			meNotif.Use(RequireAuth)
			meNotif.Get("/", p.NotificationHandler.ListNotifications)
			meNotif.Get("/unread-count", p.NotificationHandler.GetUnreadCount)
			meNotif.Post("/read-all", p.NotificationHandler.MarkAllAsRead)
			meNotif.Post("/{id}/read", p.NotificationHandler.MarkAsRead)
			meNotif.Delete("/{id}", p.NotificationHandler.DeleteNotification)
		})
		api.Route("/me/notification-preferences", func(mePrefs chi.Router) {
			mePrefs.Use(RequireAuth)
			mePrefs.Get("/", p.NotificationHandler.GetPreferences)
			mePrefs.Put("/", p.NotificationHandler.UpdatePreferences)
		})

		// Safeguarding & Confidential Incident Reporting (Public)
		if p.SafeguardingHandler != nil {
			api.Route("/safeguarding", func(sg chi.Router) {
				sg.Get("/public-config", p.SafeguardingHandler.GetPublicConfig)
				sg.Post("/reports", p.SafeguardingHandler.SubmitReport)
				sg.Post("/reports/status", p.SafeguardingHandler.GetReportStatus)
				sg.Post("/reports/messages", p.SafeguardingHandler.PostReporterMessage)
			})
		}

		// Public Transparency, Reports & Accountability Portal
		if p.ReportHandler != nil {
			mountReports := func(rep chi.Router) {
				rep.Get("/overview", p.ReportHandler.GetOverview)
				rep.Get("/cleanups", p.ReportHandler.GetCleanups)
				rep.Get("/participation", p.ReportHandler.GetParticipation)
				rep.Get("/finance", p.ReportHandler.GetFinance)
				rep.Get("/foundation", p.ReportHandler.GetFoundation)
				rep.Get("/projects", p.ReportHandler.GetCommunityProjects)
				rep.Get("/publications", p.ReportHandler.ListPublications)
				rep.Get("/publications/{id}", p.ReportHandler.GetPublicationByID)
				rep.Get("/publications/{id}/pdf", p.ReportHandler.DownloadPDF)
				rep.Get("/publications/{id}/csv", p.ReportHandler.DownloadCSV)
				rep.Get("/methodology", p.ReportHandler.GetMethodology)
			}
			api.Route("/reports", mountReports)
			api.Route("/public/reports", mountReports)

			// Internal Automated Triggers (Render Free / External Cron)
			api.Route("/internal", func(in chi.Router) {
				in.Post("/cron/reports", p.ReportHandler.RunSchedulerCron)
				in.Post("/cron/expire-orders", p.MarketplaceHandler.ExpireStaleOrdersCron)
				in.Post("/cron/retry-refunds", p.MarketplaceHandler.RetryPendingRefundsCron)
			})
		}

		// Admin Workspace
		api.Route("/admin", func(adm chi.Router) {
			adm.With(RequireRole("national_admin", "state_coordinator", "zonal_coordinator", "lga_representative")).Get("/overview", p.AdminHandler.GetOverview)
			adm.With(RequireRole("national_admin")).Get("/audit-logs", p.AdminHandler.ListAuditLogs)
			adm.With(RequireRole("super_admin", "national_admin")).Post("/geography/seed", p.AdminHandler.SeedGeography)

			// Event Media Gallery & Moderation Administration
			adm.Route("/gallery", func(admGal chi.Router) {
				admGal.Use(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator", "lga_representative"))
				admGal.Get("/", p.GalleryHandler.ListAdminGallery)
				admGal.Get("/overview", p.GalleryHandler.GetAdminOverview)
				admGal.Get("/reports", p.GalleryHandler.ListReports)
				admGal.Post("/reports/{id}/resolve", p.GalleryHandler.ResolveReport)
				admGal.Get("/takedowns", p.GalleryHandler.ListTakedowns)
				admGal.Post("/takedowns/{id}/resolve", p.GalleryHandler.ResolveTakedown)
				admGal.Get("/{id}", p.GalleryHandler.GetAdminMediaDetail)
				admGal.Post("/{id}/approve", p.GalleryHandler.ApproveMedia)
				admGal.Post("/{id}/reject", p.GalleryHandler.RejectMedia)
				admGal.Post("/{id}/request-revision", p.GalleryHandler.RequestRevision)
				admGal.Post("/{id}/remove", p.GalleryHandler.RemoveMedia)
				admGal.Patch("/{id}/toggle-hide", p.GalleryHandler.ToggleHideMedia)
				admGal.Post("/{id}/toggle-hide", p.GalleryHandler.ToggleHideMedia)
			})

			// Homepage Hero Media Administration (Super Admin & National Admin)
			adm.Route("/hero-media", func(admHero chi.Router) {
				admHero.Use(RequireRole("national_admin", "super_admin"))
				admHero.Get("/", p.HeroMediaHandler.ListHeroMedia)
				admHero.Post("/presign", p.HeroMediaHandler.AuthorizeHeroMediaUpload)
				admHero.Post("/upload", p.HeroMediaHandler.UploadHeroMediaDirect)
				admHero.Post("/publish", p.HeroMediaHandler.PublishHeroMedia)
				admHero.Post("/{id}/publish", p.HeroMediaHandler.PublishHeroMedia)
				admHero.Patch("/toggle", p.HeroMediaHandler.ToggleHeroMedia)
				admHero.Post("/{id}/poster", p.HeroMediaHandler.UploadHeroPoster)
				admHero.Delete("/{id}", p.HeroMediaHandler.DeleteHeroMedia)
			})

			// Community Projects & Requests Administration
			adm.Route("/community", func(admComm chi.Router) {
				admComm.Use(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator", "lga_representative"))
				admComm.Get("/overview", p.CommunityProjectHandler.GetOverviewStats)
				admComm.Get("/requests", p.CommunityProjectHandler.ListAdminRequests)
				admComm.Post("/requests/{id}/review", p.CommunityProjectHandler.ReviewCommunityRequest)
				admComm.Post("/requests/{id}/convert", p.CommunityProjectHandler.ConvertCommunityRequest)
				admComm.Get("/projects", p.CommunityProjectHandler.ListAdminProjects)
				admComm.Post("/projects", p.CommunityProjectHandler.CreateCommunityProject)
				admComm.Get("/projects/{id}", p.CommunityProjectHandler.GetAdminProject)
				admComm.Patch("/projects/{id}", p.CommunityProjectHandler.UpdateProjectDetails)
				admComm.Patch("/projects/{id}/execution", p.CommunityProjectHandler.UpdateProjectExecution)
				admComm.Patch("/projects/{id}/status", p.CommunityProjectHandler.UpdateProjectExecution)
				admComm.Post("/projects/{id}/milestones", p.CommunityProjectHandler.AddMilestone)
				admComm.Patch("/projects/{id}/milestones/{milestone_id}", p.CommunityProjectHandler.UpdateMilestone)
				admComm.Post("/projects/{id}/milestones/{milestone_id}/verify", p.CommunityProjectHandler.VerifyMilestone)
				admComm.Delete("/projects/{id}/milestones/{milestone_id}", p.CommunityProjectHandler.DeleteMilestone)
				admComm.Post("/projects/{id}/funding", p.CommunityProjectHandler.RecordProjectFunding)
				admComm.Post("/projects/{id}/updates", p.CommunityProjectHandler.CreateProjectUpdate)
				admComm.Post("/projects/{id}/presign", p.CommunityProjectHandler.PresignProjectUpload)
				admComm.Post("/projects/{id}/upload", p.CommunityProjectHandler.UploadProjectImageDirect)
			})

			// Communications, Outbox & Delivery Administration
			adm.Route("/communications", func(admComms chi.Router) {
				admComms.Use(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator", "lga_representative"))
				admComms.Get("/overview", p.NotificationHandler.GetOverviewStats)
				admComms.Get("/deliveries", p.NotificationHandler.ListOutbox)
				admComms.Get("/failed", p.NotificationHandler.ListFailed)
				admComms.Post("/deliveries/{id}/retry", p.NotificationHandler.RetryDelivery)
				admComms.Get("/logs", p.NotificationHandler.ListDeliveries)
				admComms.Get("/templates", p.NotificationHandler.ListTemplates)
				admComms.Put("/templates/{id}", p.NotificationHandler.UpdateTemplate)
				admComms.Get("/schedules", p.NotificationHandler.ListSchedules)
				admComms.Get("/provider-health", p.NotificationHandler.GetProviderHealth)
			})

			// Confidential Safeguarding Administration
			if p.SafeguardingHandler != nil {
				adm.Route("/safeguarding", func(admSg chi.Router) {
					admSg.Use(RequireAuth)
					admSg.Get("/cases", p.SafeguardingHandler.ListCases)
					admSg.Get("/cases/{id}", p.SafeguardingHandler.GetCaseDetails)
					admSg.Post("/cases/{id}/triage", p.SafeguardingHandler.UpdateTriage)
					admSg.Post("/cases/{id}/assign", p.SafeguardingHandler.AssignOfficer)
					admSg.Post("/cases/{id}/conflict", p.SafeguardingHandler.DeclareConflict)
					admSg.Post("/cases/{id}/escalate", p.SafeguardingHandler.EscalateCase)
					admSg.Post("/cases/{id}/resolve", p.SafeguardingHandler.ResolveCase)
					admSg.Post("/cases/{id}/close", p.SafeguardingHandler.CloseCase)
					admSg.Post("/cases/{id}/notes", p.SafeguardingHandler.AddInternalNote)
					admSg.Post("/cases/{id}/reporter-message", p.SafeguardingHandler.SendReporterUpdate)
					admSg.Get("/cases/{id}/evidence/{evidence_id}/download", p.SafeguardingHandler.GetEvidenceDownloadURL)
					admSg.Get("/evidence/sandbox-download", p.SafeguardingHandler.SandboxDownloadEvidence)
					admSg.Post("/cases/{id}/legal-hold", p.SafeguardingHandler.SetLegalHold)
					admSg.Get("/access-logs", p.SafeguardingHandler.ListAccessLogs)
					admSg.Get("/officers", p.SafeguardingHandler.ListActiveOfficers)
					admSg.Post("/officers", p.SafeguardingHandler.AppointOfficer)
				})
			}

			// Public Transparency & Reports Administration Workspace
			if p.ReportHandler != nil {
				adm.Route("/reports", func(admRep chi.Router) {
					admRep.Use(RequireRole("national_admin", "super_admin", "zonal_coordinator", "state_coordinator", "lga_representative"))
					admRep.Get("/", p.ReportHandler.ListSnapshotsAdmin)
					admRep.Get("/dashboard/stats", p.ReportHandler.GetDashboardStats)
					admRep.Post("/generate", p.ReportHandler.GenerateSnapshotAdmin)
					admRep.Get("/scheduler/runs", p.ReportHandler.ListSchedulerRuns)
					admRep.Get("/scheduler/runs/{id}", p.ReportHandler.GetSchedulerRun)
					admRep.Get("/exports", p.ReportHandler.ListExportsAdmin)
					admRep.Get("/audit", p.ReportHandler.ListAuditsAdmin)
					admRep.Get("/settings", p.ReportHandler.GetSettingsAdmin)
					admRep.Get("/{id}", p.ReportHandler.GetSnapshotDetailAdmin)
					admRep.Post("/{id}/submit-review", p.ReportHandler.SubmitReviewAdmin)
					admRep.Post("/{id}/submit", p.ReportHandler.SubmitReviewAdmin)
					admRep.Post("/{id}/approve", p.ReportHandler.ApproveSnapshotAdmin)
					admRep.Post("/{id}/reject", p.ReportHandler.RejectSnapshotAdmin)
					admRep.With(RequireRole("super_admin")).Post("/{id}/publish", p.ReportHandler.PublishSnapshotAdmin)
					admRep.With(RequireRole("super_admin")).Post("/{id}/withdraw", p.ReportHandler.WithdrawSnapshotAdmin)
					admRep.Post("/{id}/revise", p.ReportHandler.ReviseSnapshotAdmin)
					admRep.With(RequireRole("super_admin", "national_admin")).Post("/{id}/exports/{format}/regenerate", p.ReportHandler.RegenerateExportAdmin)
					admRep.Get("/{id}/exports/{format}/download", p.ReportHandler.DownloadExportAdmin)
					admRep.With(RequireRole("super_admin", "national_admin")).Post("/scheduler/run", p.ReportHandler.RunSchedulerAdmin)
				})
			}
		})
	})

	return r
}

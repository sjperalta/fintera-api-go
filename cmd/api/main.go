package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/sjperalta/fintera-api/docs" // Swagger docs
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/database"
	"github.com/sjperalta/fintera-api/internal/handlers"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
	"github.com/sjperalta/fintera-api/pkg/logger"

	"github.com/gin-gonic/gin"
)

// @title Fintera API
// @version 1.0
// @description REST API for Fintera Real Estate Management System
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8081
// @BasePath /api/v1
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Setup(cfg.Environment)

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	logger.Info("Connected to database")

	// Initialize storage
	store, err := storage.NewLocalStorage(cfg.StoragePath)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	logger.Info("Initialized local storage")

	// Initialize repositories
	repos := repository.NewRepositories(db)

	// Initialize background worker
	worker := jobs.NewWorker(cfg.WorkerCount)
	logger.Info("Started background worker", "goroutines", cfg.WorkerCount)

	// Initialize services
	svcs := services.NewServices(repos, worker, store, cfg, db)

	// Schedule recurring jobs
	scheduleJobs(worker, svcs)

	// Initialize handlers
	h := handlers.NewHandlers(svcs, store)

	// Setup router
	router := setupRouter(h, cfg)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	// Start server in a goroutine
	go func() {
		logger.Info("Server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	// Shutdown background worker
	worker.Shutdown()
	logger.Info("Background worker stopped")

	logger.Info("Server exited gracefully")
}

func setupRouter(h *handlers.Handlers, cfg *config.Config) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORS(cfg.AllowedOrigins))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Redirect root to swagger
		router.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
		})

		// Swagger documentation
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// Health check (public)
		v1.GET("/health", h.Health.Index)

		// Authentication (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/login", h.Auth.Login)
			auth.POST("/refresh", h.Auth.Refresh)
			auth.POST("/logout", h.Auth.Logout)
		}

		// Password recovery (public)
		v1.POST("/users/send_recovery_code", h.User.SendRecoveryCode)
		v1.POST("/users/verify_recovery_code", h.User.VerifyRecoveryCode)
		v1.POST("/users/update_password_with_code", h.User.UpdatePasswordWithCode)

		// Protected routes (requires authentication)
		protected := v1.Group("")
		protected.Use(middleware.Auth(cfg.JWTSecret))
		{
			// Admin-only routes
			admin := protected.Group("")
			admin.Use(middleware.RequireAdmin())
			{
				// User management (admin only)
				admin.POST("/users", h.User.Create)
				admin.PUT("/users/:user_id", h.User.Update)
				admin.DELETE("/users/:user_id", h.User.Delete)
				admin.PUT("/users/:user_id/toggle_status", h.User.ToggleStatus)
				admin.POST("/users/:user_id/restore", h.User.Restore)

				// Contract approval/rejection/cancellation (admin only)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/approve", h.Contract.Approve)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/reject", h.Contract.Reject)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/cancel", h.Contract.Cancel)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/reopen", h.Contract.Reopen)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/capital_repayment", h.Contract.CapitalRepayment)

				// Payment approval/rejection/undo (admin only)
				admin.POST("/payments/:payment_id/approve", h.Payment.Approve)
				admin.POST("/payments/:payment_id/reject", h.Payment.Reject)
				admin.POST("/payments/:payment_id/undo", h.Payment.Undo)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/approve", h.Payment.ApproveByContract)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/reject", h.Payment.RejectByContract)

				// Project management (admin only)
				admin.POST("/projects", h.Project.Create)
				admin.PUT("/projects/:project_id", h.Project.Update)
				admin.DELETE("/projects/:project_id", h.Project.Delete)
				admin.POST("/projects/:project_id/approve", h.Project.Approve)
				admin.POST("/projects/import", h.Project.Import)

				// Lot management (admin only)
				admin.POST("/projects/:project_id/lots", h.Lot.Create)
				admin.PUT("/projects/:project_id/lots/:lot_id", h.Lot.Update)
				admin.DELETE("/projects/:project_id/lots/:lot_id", h.Lot.Delete)

			}

			// Seller + Admin routes (viewing and managing sales)
			sellerAdmin := protected.Group("")
			sellerAdmin.Use(middleware.RequireRole("admin", "seller"))
			{
				// User viewing (seller can view users for sales purposes)
				sellerAdmin.GET("/users", h.User.Index)
				sellerAdmin.GET("/users/:user_id", h.User.Show)
				sellerAdmin.GET("/users/:user_id/contracts", h.User.Contracts)
				sellerAdmin.GET("/users/:user_id/payments", h.User.Payments)
				sellerAdmin.GET("/users/:user_id/payment_history", h.User.PaymentHistory)
				sellerAdmin.GET("/users/:user_id/summary", h.User.Summary)

				// Contract viewing (seller can view all contracts)
				sellerAdmin.GET("/contracts", h.Contract.Index)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id", h.Contract.Show)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id/ledger", h.Contract.Ledger)

				// Payment viewing (seller can view all payments)
				sellerAdmin.GET("/payments", h.Payment.Index)
				sellerAdmin.GET("/payments/statistics", h.Payment.Statistics)
				sellerAdmin.GET("/payments/stats", h.Payment.Stats)
				sellerAdmin.GET("/payments/:payment_id", h.Payment.Show)
				sellerAdmin.GET("/payments/:payment_id/download_receipt", h.Payment.DownloadReceipt)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments", h.Payment.IndexByContract)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id", h.Payment.ShowByContract)

				// Project/Lot viewing (seller can view all)
				sellerAdmin.GET("/projects", h.Project.Index)
				sellerAdmin.GET("/projects/:project_id", h.Project.Show)
				sellerAdmin.GET("/projects/:project_id/lots", h.Lot.Index)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id", h.Lot.Show)

				// Analytics (seller can view analytics)
				analytics := sellerAdmin.Group("/analytics")
				{
					analytics.GET("/overview", h.Analytics.Overview)
					analytics.GET("/distribution", h.Analytics.Distribution)
					analytics.GET("/performance", h.Analytics.Performance)
					analytics.GET("/export", h.Analytics.Export)
				}

				// Reports (seller can generate reports)
				sellerAdmin.GET("/reports/commissions_csv", h.Report.CommissionsCSV)
				sellerAdmin.GET("/reports/total_revenue_csv", h.Report.TotalRevenueCSV)
				sellerAdmin.GET("/reports/overdue_payments_csv", h.Report.OverduePaymentsCSV)
				sellerAdmin.GET("/reports/user_balance_pdf", h.Report.UserBalancePDF)
				sellerAdmin.GET("/reports/user_promise_contract_pdf", h.Report.UserPromiseContractPDF)
				sellerAdmin.GET("/reports/user_rescission_contract_pdf", h.Report.UserRescissionContractPDF)
				sellerAdmin.GET("/reports/user_information_pdf", h.Report.UserInformationPDF)
				sellerAdmin.GET("/reports/customer_record_pdf", h.Report.CustomerRecordPDF)

				// Audits (seller can view audit logs)
				sellerAdmin.GET("/audits", h.Audit.Index)
			}

			// All authenticated users (personal data access)
			// User can change their own password
			protected.PATCH("/users/:user_id/change_password", h.User.ChangePassword)
			protected.POST("/users/:user_id/resend_confirmation", h.User.ResendConfirmation)
			protected.PATCH("/users/:user_id/update_locale", h.User.UpdateLocale)

			// Contract creation (users can create their own contracts)
			protected.POST("/projects/:project_id/lots/:lot_id/contracts", h.Contract.Create)
			protected.PATCH("/projects/:project_id/lots/:lot_id/contracts/:contract_id", h.Contract.Update)

			// Payment receipt upload (users can upload their own receipts)
			protected.POST("/payments/:payment_id/upload_receipt", h.Payment.UploadReceipt)
			protected.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/upload_receipt", h.Payment.UploadReceiptByContract)

			// Notifications (users can manage their own notifications)
			notifications := protected.Group("/notifications")
			{
				notifications.GET("", h.Notification.Index)
				notifications.GET("/:notification_id", h.Notification.Show)
				notifications.PUT("/:notification_id", h.Notification.Update)
				notifications.DELETE("/:notification_id", h.Notification.Delete)
				notifications.POST("/mark_all_as_read", h.Notification.MarkAllAsRead)
			}
		}
	}

	return router
}

func scheduleJobs(worker *jobs.Worker, svcs *services.Services) {
	// Check overdue payments every hour
	worker.ScheduleEvery(1*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Checking overdue payments...")
		// 1. Calculate and apply overdue interest
		if err := svcs.Payment.CalculateOverdueInterest(ctx); err != nil {
			logger.Error("Error calculating overdue interest", "error", err)
		}

		// 2. Send notifications
		return svcs.Payment.CheckOverduePayments(ctx)
	})

	// Update credit scores every 6 hours
	worker.ScheduleEvery(6*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Updating credit scores...")
		return svcs.CreditScore.UpdateAllScores(ctx)
	})

	// Refresh analytics cache every 15 minutes
	worker.ScheduleEvery(15*time.Minute, func(ctx context.Context) error {
		logger.Info("[Job] Refreshing analytics cache...")
		return svcs.Analytics.RefreshCache(ctx)
	})

	// Release unpaid reservations every hour
	worker.ScheduleEvery(1*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Releasing unpaid reservations...")
		return svcs.Contract.ReleaseUnpaidReservations(ctx)
	})

	logger.Info("Scheduled recurring jobs")
}

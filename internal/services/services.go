package services

import (
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/storage"
	"gorm.io/gorm"
)

// Services holds all service instances
type Services struct {
	Auth         *AuthService
	User         *UserService
	Project      *ProjectService
	Lot          *LotService
	Contract     *ContractService
	Payment      *PaymentService
	Notification *NotificationService
	Report       *ReportService
	Audit        *AuditService
	CreditScore  *CreditScoreService
	Email        *EmailService
	Analytics    *AnalyticsService
	Export       *ExportService
	Job          *JobService
}

// NewServices creates all service instances
func NewServices(repos *repository.Repositories, worker *jobs.Worker, storage *storage.LocalStorage, cfg *config.Config, db *gorm.DB) *Services {
	notificationSvc := NewNotificationService(repos.Notification, repos.User)
	emailSvc := NewEmailService(cfg)
	auditSvc := NewAuditService(db) // Create AuditService instance

	// Create ImageService
	imageSvc := NewImageService(cfg.StoragePath + "/uploads")

	analyticsSvc := NewAnalyticsService(repos.Analytics, repos.Project, notificationSvc, repos.User)
	jobSvc := NewJobService(worker)

	return &Services{
		Auth:         NewAuthService(repos.User, repos.RefreshToken, cfg),
		User:         NewUserService(repos.User, repos.Contract, worker, emailSvc, auditSvc, imageSvc),
		Project:      NewProjectService(repos.Project, repos.Lot, auditSvc),
		Lot:          NewLotService(repos.Lot, repos.Project, auditSvc),
		Contract:     NewContractService(repos.Contract, repos.Lot, repos.User, repos.Payment, repos.Ledger, notificationSvc, emailSvc, auditSvc, worker),
		Payment:      NewPaymentService(repos.Payment, repos.Contract, repos.Lot, repos.Ledger, notificationSvc, emailSvc, auditSvc, storage, worker),
		Notification: notificationSvc,
		Report:       NewReportService(repos.Payment, repos.Contract, repos.User),
		Audit:        auditSvc, // Assign AuditService
		CreditScore:  NewCreditScoreService(repos.User, repos.Contract, repos.Payment),
		Email:        emailSvc,
		Analytics:    analyticsSvc,
		Export:       NewExportService(analyticsSvc), // AnalyticsSvc passed to ExportSvc
		Job:          jobSvc,
	}
}

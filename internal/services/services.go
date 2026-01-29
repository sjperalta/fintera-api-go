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
}

// NewServices creates all service instances
func NewServices(repos *repository.Repositories, worker *jobs.Worker, storage *storage.LocalStorage, cfg *config.Config, db *gorm.DB) *Services {
	notificationSvc := NewNotificationService(repos.Notification, repos.User)
	emailSvc := NewEmailService(cfg)
	auditSvc := NewAuditService(db) // Create AuditService instance

	return &Services{
		Auth:         NewAuthService(repos.User, repos.RefreshToken, cfg),
		User:         NewUserService(repos.User, worker, emailSvc),
		Project:      NewProjectService(repos.Project),
		Lot:          NewLotService(repos.Lot, repos.Project),
		Contract:     NewContractService(repos.Contract, repos.Lot, repos.User, repos.Payment, repos.Ledger, notificationSvc, emailSvc, auditSvc, worker),
		Payment:      NewPaymentService(repos.Payment, repos.Contract, repos.Lot, repos.Ledger, notificationSvc, auditSvc, storage, worker),
		Notification: notificationSvc,
		Report:       NewReportService(repos.Payment, repos.Contract, repos.User),
		Audit:        auditSvc, // Assign AuditService
		CreditScore:  NewCreditScoreService(repos.User, repos.Contract, repos.Payment),
		Email:        emailSvc,
		Analytics:    NewAnalyticsService(repos.Analytics, repos.Project, notificationSvc, repos.User),
		Export:       NewExportService(NewAnalyticsService(repos.Analytics, repos.Project, notificationSvc, repos.User)), // AnalyticsSvc passed to ExportSvc
	}
}

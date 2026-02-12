package repository

import (
	"gorm.io/gorm"
)

// Repositories holds all repository instances
type Repositories struct {
	User         UserRepository
	Project      ProjectRepository
	Lot          LotRepository
	Contract     ContractRepository
	Payment      PaymentRepository
	Notification NotificationRepository
	RefreshToken RefreshTokenRepository
	Ledger       LedgerRepository
	Analytics    AnalyticsRepository
}

// NewRepositories creates all repository instances
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		User:         NewUserRepository(db),
		Project:      NewProjectRepository(db),
		Lot:          NewLotRepository(db),
		Contract:     NewContractRepository(db),
		Payment:      NewPaymentRepository(db),
		Notification: NewNotificationRepository(db),
		RefreshToken: NewRefreshTokenRepository(db),
		Ledger:       NewLedgerRepository(db),
		Analytics:    NewAnalyticsRepository(db),
	}
}

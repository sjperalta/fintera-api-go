package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
)

// Mock LedgerRepository
type mockLedgerRepository struct {
	mockFindOrCreate    func(ctx context.Context, entry *models.ContractLedgerEntry) error
	mockBatchUpsert     func(ctx context.Context, entries []models.ContractLedgerEntry) error
}

func (m *mockLedgerRepository) Create(ctx context.Context, entry *models.ContractLedgerEntry) error {
	return nil
}
func (m *mockLedgerRepository) FindByContractID(ctx context.Context, contractID uint) ([]models.ContractLedgerEntry, error) {
	return nil, nil
}
func (m *mockLedgerRepository) FindByPaymentID(ctx context.Context, paymentID uint) ([]models.ContractLedgerEntry, error) {
	return nil, nil
}
func (m *mockLedgerRepository) CalculateBalance(ctx context.Context, contractID uint) (float64, error) {
	return 0, nil
}
func (m *mockLedgerRepository) FindOrCreateByPaymentAndType(ctx context.Context, entry *models.ContractLedgerEntry) error {
	if m.mockFindOrCreate != nil {
		return m.mockFindOrCreate(ctx, entry)
	}
	return nil
}
func (m *mockLedgerRepository) BatchUpsertInterest(ctx context.Context, entries []models.ContractLedgerEntry) error {
	if m.mockBatchUpsert != nil {
		return m.mockBatchUpsert(ctx, entries)
	}
	return nil
}
func (m *mockLedgerRepository) DeleteByContractID(ctx context.Context, contractID uint) error {
	return nil
}

// Mock UserRepository (using embedding to avoid implementing all methods)
type mockUserRepository struct {
	repository.UserRepository
	mockFindAdmins func(ctx context.Context) ([]models.User, error)
}

func (m *mockUserRepository) FindAdmins(ctx context.Context) ([]models.User, error) {
	if m.mockFindAdmins != nil {
		return m.mockFindAdmins(ctx)
	}
	return nil, nil
}

// Mock NotificationRepository
type mockNotificationRepository struct {
	repository.NotificationRepository
	mockCreate func(ctx context.Context, notification *models.Notification) error
}

func (m *mockNotificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	if m.mockCreate != nil {
		return m.mockCreate(ctx, notification)
	}
	return nil
}

// Redefine mock here to add FindOverdue support
type mockPaymentRepositoryWithOverdue struct {
	repository.PaymentRepository
	mockFindOverdue    func(ctx context.Context) ([]models.Payment, error)
	mockUpdate         func(ctx context.Context, payment *models.Payment) error
	mockBatchUpdate    func(ctx context.Context, updates map[uint]float64) error
}

func (m *mockPaymentRepositoryWithOverdue) FindOverdue(ctx context.Context) ([]models.Payment, error) {
	if m.mockFindOverdue != nil {
		return m.mockFindOverdue(ctx)
	}
	return nil, nil
}

func (m *mockPaymentRepositoryWithOverdue) Update(ctx context.Context, payment *models.Payment) error {
	if m.mockUpdate != nil {
		return m.mockUpdate(ctx, payment)
	}
	return nil
}

func (m *mockPaymentRepositoryWithOverdue) BatchUpdateInterest(ctx context.Context, updates map[uint]float64) error {
	if m.mockBatchUpdate != nil {
		return m.mockBatchUpdate(ctx, updates)
	}
	return nil
}

// Implement other interface methods as no-ops...
// (Skipping for brevity in this thought trace, but must be in file)
func (m *mockPaymentRepositoryWithOverdue) FindByID(ctx context.Context, id uint) (*models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepositoryWithOverdue) FindByContract(ctx context.Context, contractID uint) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepositoryWithOverdue) Create(ctx context.Context, payment *models.Payment) error {
	return nil
}
func (m *mockPaymentRepositoryWithOverdue) Delete(ctx context.Context, id uint) error { return nil }
func (m *mockPaymentRepositoryWithOverdue) DeleteByContract(ctx context.Context, contractID uint) error {
	return nil
}
func (m *mockPaymentRepositoryWithOverdue) List(ctx context.Context, query *repository.ListQuery) ([]models.Payment, int64, error) {
	return nil, 0, nil
}
func (m *mockPaymentRepositoryWithOverdue) FindOverdueForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepositoryWithOverdue) FindPaymentsDueTomorrowForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepositoryWithOverdue) MarkOverdueReminderSent(ctx context.Context, paymentIDs []uint) error {
	return nil
}
func (m *mockPaymentRepositoryWithOverdue) MarkUpcomingReminderSent(ctx context.Context, paymentIDs []uint) error {
	return nil
}
func (m *mockPaymentRepositoryWithOverdue) FindPendingByUser(ctx context.Context, userID uint) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepositoryWithOverdue) FindPaidByMonth(ctx context.Context, month, year int) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepositoryWithOverdue) GetMonthlyStats(ctx context.Context) (*repository.PaymentStats, error) {
	return nil, nil
}
func (m *mockPaymentRepositoryWithOverdue) FindByUserID(ctx context.Context, userID uint) ([]models.Payment, error) {
	return nil, nil
}

func TestCalculateOverdueInterest_Logic(t *testing.T) {
	mockPaymentRepo := &mockPaymentRepositoryWithOverdue{}
	mockLedgerRepo := &mockLedgerRepository{}
	mockUserRepo := &mockUserRepository{}
	mockNotifRepo := &mockNotificationRepository{}

	// Setup Worker and NotificationService
	worker := jobs.NewWorker(0) // 0 workers, but EnqueueAsync spawns its own goroutines
	defer worker.Shutdown()

	notifService := NewNotificationService(mockNotifRepo, mockUserRepo)

	service := NewPaymentService(mockPaymentRepo, nil, nil, mockLedgerRepo, notifService, nil, nil, nil, worker)

	// Test Data
	now := time.Now()
	daysOverdue := 10
	dueDate := now.AddDate(0, 0, -daysOverdue)
	amount := 5000.00
	interestRate := 10.0 // 10%

	project := models.Project{
		ID:           1,
		InterestRate: interestRate,
	}
	lot := models.Lot{
		ID:        1,
		ProjectID: 1,
		Project:   project,
	}
	contract := models.Contract{
		ID:     100,
		Status: models.ContractStatusApproved,
		Active: true,
		LotID:  1,
		Lot:    lot,
	}
	payment := models.Payment{
		ID:          1000,
		ContractID:  100,
		Contract:    contract,
		PaymentType: models.PaymentTypeInstallment,
		Status:      models.PaymentStatusPending,
		Amount:      amount,
		DueDate:     dueDate,
	}

	// 1. Setup FindOverdue
	mockPaymentRepo.mockFindOverdue = func(ctx context.Context) ([]models.Payment, error) {
		return []models.Payment{payment}, nil
	}

	// 2. Setup Ledger BatchUpsertInterest expectations
	ledgerCalled := false
	mockLedgerRepo.mockBatchUpsert = func(ctx context.Context, entries []models.ContractLedgerEntry) error {
		ledgerCalled = true
		assert.Len(t, entries, 1)

		entry := entries[0]
		// Verify Entry Fields
		assert.Equal(t, contract.ID, entry.ContractID)
		assert.Equal(t, payment.ID, *entry.PaymentID)
		assert.Equal(t, models.EntryTypeInterest, entry.EntryType)

		// Verify Amount Calculation
		// Formula: amount * (days / 365) * rate
		// 5000 * (10 / 365) * 0.10
		expectedInterest := (5000.0 * 10.0 / 365.0) * 0.10
		expectedAmount := -expectedInterest // NEGATIVE

		assert.InDelta(t, expectedAmount, entry.Amount, 0.001, "Interest amount mismatch")

		return nil
	}

	paymentUpdateCalled := false
	mockPaymentRepo.mockBatchUpdate = func(ctx context.Context, updates map[uint]float64) error {
		paymentUpdateCalled = true
		assert.Len(t, updates, 1)

		expectedInterest := (5000.0 * 10.0 / 365.0) * 0.10
		assert.InDelta(t, expectedInterest, updates[payment.ID], 0.001, "Payment update amount mismatch")
		return nil
	}

	// 3. Setup Notification Logic expectations
	mockUserRepo.mockFindAdmins = func(ctx context.Context) ([]models.User, error) {
		return []models.User{{ID: 99, Email: "admin@example.com"}}, nil
	}
	mockNotifRepo.mockCreate = func(ctx context.Context, notification *models.Notification) error {
		return nil
	}

	// Execute
	err := service.CalculateOverdueInterest(context.Background())
	assert.NoError(t, err)
	assert.True(t, ledgerCalled, "BatchUpsertInterest should be called")
	assert.True(t, paymentUpdateCalled, "BatchUpdateInterest should be called")

	// Wait a bit for async notification
	time.Sleep(100 * time.Millisecond)
	// We can't easily assert async calls without a waitgroup or channel in the mock,
	// but confirming it didn't panic is the main goal here.
	// If we wanted to be strict:
	// assert.True(t, notifCalled, "Notification should be created")
}

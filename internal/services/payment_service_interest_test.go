package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
)

// Mock LedgerRepository
type mockLedgerRepository struct {
	mockFindOrCreate func(ctx context.Context, entry *models.ContractLedgerEntry) error
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
func (m *mockLedgerRepository) DeleteByContractID(ctx context.Context, contractID uint) error {
	return nil
}

// Mock NotificationService (minimal)
// We can't easily mock the struct method without an interface, but we can pass a nil or dummy if carefully handled.
// However, the service uses `s.worker.EnqueueAsync` which wraps the notification.
// We can test the logic UP TO the notification by mocking the repo to return 0 overdue payments if we want to skip notification,
// OR we just assume it works.
// For this test, verifying the Ledger Entry creation is the most important part.

// Redefine mock here to add FindOverdue support
type mockPaymentRepositoryWithOverdue struct {
	repository.PaymentRepository
	mockFindOverdue func(ctx context.Context) ([]models.Payment, error)
	mockUpdate      func(ctx context.Context, payment *models.Payment) error
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

func TestCalculateOverdueInterest_Logic(t *testing.T) {
	mockPaymentRepo := &mockPaymentRepositoryWithOverdue{}
	mockLedgerRepo := &mockLedgerRepository{}

	service := NewPaymentService(mockPaymentRepo, nil, nil, mockLedgerRepo, nil, nil, nil, nil, nil)

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

	// 2. Setup Ledger FindOrCreate expectations
	ledgerCalled := false
	mockLedgerRepo.mockFindOrCreate = func(ctx context.Context, entry *models.ContractLedgerEntry) error {
		ledgerCalled = true

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

	// Execute
	err := service.CalculateOverdueInterest(context.Background())
	assert.NoError(t, err)
	assert.True(t, ledgerCalled, "FindOrCreateByPaymentAndType should be called")
}

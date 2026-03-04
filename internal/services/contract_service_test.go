package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Repositories

type mockContractRepo struct {
	mock.Mock
}

func (m *mockContractRepo) FindByID(ctx context.Context, id uint) (*models.Contract, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindByIDWithDetails(ctx context.Context, id uint) (*models.Contract, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindByLot(ctx context.Context, lotID uint) ([]models.Contract, error) {
	args := m.Called(ctx, lotID)
	return args.Get(0).([]models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindByUser(ctx context.Context, userID uint) ([]models.Contract, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Contract), args.Error(1)
}

func (m *mockContractRepo) Create(ctx context.Context, contract *models.Contract) error {
	args := m.Called(ctx, contract)
	return args.Error(0)
}

func (m *mockContractRepo) Update(ctx context.Context, contract *models.Contract) error {
	args := m.Called(ctx, contract)
	return args.Error(0)
}

func (m *mockContractRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockContractRepo) List(ctx context.Context, query *repository.ContractQuery) ([]models.Contract, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.Contract), args.Get(1).(int64), args.Error(2)
}

func (m *mockContractRepo) FindActiveByLot(ctx context.Context, lotID uint) (*models.Contract, error) {
	args := m.Called(ctx, lotID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contract), args.Error(1)
}

func (m *mockContractRepo) FindPendingReservations(ctx context.Context, olderThan int) ([]models.Contract, error) {
	args := m.Called(ctx, olderThan)
	return args.Get(0).([]models.Contract), args.Error(1)
}

func (m *mockContractRepo) GetStats(ctx context.Context) (*repository.ContractStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ContractStats), args.Error(1)
}

func (m *mockContractRepo) HasActiveContracts(ctx context.Context, userID uint) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

type mockUserRepoForContract struct {
	mock.Mock
}

func (m *mockUserRepoForContract) FindByID(ctx context.Context, id uint) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForContract) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForContract) FindByIdentity(ctx context.Context, identity string) (*models.User, error) {
	args := m.Called(ctx, identity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepoForContract) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepoForContract) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepoForContract) SetRecoveryCode(ctx context.Context, userID uint, code string, sentAt time.Time) error {
	args := m.Called(ctx, userID, code, sentAt)
	return args.Error(0)
}

func (m *mockUserRepoForContract) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepoForContract) SoftDelete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepoForContract) Restore(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepoForContract) List(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.User), args.Get(1).(int64), args.Error(2)
}

func (m *mockUserRepoForContract) FindAdmins(ctx context.Context) ([]models.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.User), args.Error(1)
}

func (m *mockUserRepoForContract) FindAll(ctx context.Context) ([]models.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.User), args.Error(1)
}

type mockPaymentRepo struct {
	mock.Mock
}

func (m *mockPaymentRepo) FindByID(ctx context.Context, id uint) (*models.Payment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindByContract(ctx context.Context, contractID uint) ([]models.Payment, error) {
	args := m.Called(ctx, contractID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *models.Payment) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *mockPaymentRepo) Update(ctx context.Context, payment *models.Payment) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *mockPaymentRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockPaymentRepo) DeleteByContract(ctx context.Context, contractID uint) error {
	args := m.Called(ctx, contractID)
	return args.Error(0)
}

func (m *mockPaymentRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.Payment, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.Payment), args.Get(1).(int64), args.Error(2)
}

func (m *mockPaymentRepo) FindOverdue(ctx context.Context) ([]models.Payment, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindOverdueForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindPaymentsDueTomorrowForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) MarkOverdueReminderSent(ctx context.Context, paymentIDs []uint) error {
	args := m.Called(ctx, paymentIDs)
	return args.Error(0)
}

func (m *mockPaymentRepo) MarkUpcomingReminderSent(ctx context.Context, paymentIDs []uint) error {
	args := m.Called(ctx, paymentIDs)
	return args.Error(0)
}

func (m *mockPaymentRepo) FindPendingByUser(ctx context.Context, userID uint) ([]models.Payment, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) FindPaidByMonth(ctx context.Context, month, year int) ([]models.Payment, error) {
	args := m.Called(ctx, month, year)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetMonthlyStats(ctx context.Context) (*repository.PaymentStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaymentStats), args.Error(1)
}

func (m *mockPaymentRepo) FindByUserID(ctx context.Context, userID uint) ([]models.Payment, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) BatchUpdateInterest(ctx context.Context, updates map[uint]float64) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

type mockLedgerRepo struct {
	mock.Mock
}

func (m *mockLedgerRepo) Create(ctx context.Context, entry *models.ContractLedgerEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *mockLedgerRepo) FindByContractID(ctx context.Context, contractID uint) ([]models.ContractLedgerEntry, error) {
	args := m.Called(ctx, contractID)
	return args.Get(0).([]models.ContractLedgerEntry), args.Error(1)
}

func (m *mockLedgerRepo) FindByPaymentID(ctx context.Context, paymentID uint) ([]models.ContractLedgerEntry, error) {
	args := m.Called(ctx, paymentID)
	return args.Get(0).([]models.ContractLedgerEntry), args.Error(1)
}

func (m *mockLedgerRepo) CalculateBalance(ctx context.Context, contractID uint) (float64, error) {
	args := m.Called(ctx, contractID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockLedgerRepo) FindOrCreateByPaymentAndType(ctx context.Context, entry *models.ContractLedgerEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *mockLedgerRepo) BatchUpsertInterest(ctx context.Context, entries []models.ContractLedgerEntry) error {
	args := m.Called(ctx, entries)
	return args.Error(0)
}

func (m *mockLedgerRepo) DeleteByContractID(ctx context.Context, contractID uint) error {
	args := m.Called(ctx, contractID)
	return args.Error(0)
}

func TestContractService_Create(t *testing.T) {
	// Initialize logger to avoid nil pointer dereference
	logger.Setup("test")

	ctx := context.Background()
	mockContractRepo := new(mockContractRepo)
	mockLotRepo := new(mockLotRepo)
	mockUserRepo := new(mockUserRepoForContract)
	mockPaymentRepo := new(mockPaymentRepo)
	mockLedgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)

	// Create notification and email services with their respective stubs
	// For simplicity, we can pass nil for services that aren't strictly accessed if we mock well,
	// but emailSvc and notificationSvc methods are called in background jobs.
	// Since we are mocking worker.EnqueueAsync or letting the background job run and fail,
	// we should probably just test the synchronous part and ignore background job errors
	// or provide proper stubs. Let's pass nil and intercept worker if needed.

	notificationSvc := NewNotificationService(nil, nil)

	// EmailSvc requires auth config
	cfg := &config.Config{ResendAPIKey: "mock-key"}
	emailSvc := NewEmailService(cfg)

	service := NewContractService(mockContractRepo, mockLotRepo, mockUserRepo, mockPaymentRepo, mockLedgerRepo, notificationSvc, emailSvc, auditSvc, worker)

	lot := &models.Lot{
		ID:     1,
		Status: models.LotStatusAvailable,
		Price:  10000,
		Project: models.Project{
			Name: "Test Project",
		},
	}

	contract := &models.Contract{
		LotID:           1,
		ApplicantUserID: 2,
	}

	applicant := &models.User{
		ID:       2,
		FullName: "Test User",
	}

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusReserved
	})).Return(nil)

	mockContractRepo.On("Create", ctx, mock.AnythingOfType("*models.Contract")).Return(nil)
	mockUserRepo.On("FindByID", ctx, uint(2)).Return(applicant, nil)

	err := service.Create(ctx, contract)

	assert.NoError(t, err)
	assert.NotEmpty(t, contract.GUID)
	assert.Equal(t, 10000.0, *contract.Amount)
	assert.Equal(t, -10000.0, *contract.Balance)

	mockLotRepo.AssertExpectations(t)
	mockContractRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}

func TestContractService_Reject(t *testing.T) {
	// Initialize logger to avoid nil pointer dereference
	logger.Setup("test")

	ctx := context.Background()
	mockContractRepo := new(mockContractRepo)
	mockLotRepo := new(mockLotRepo)
	mockUserRepo := new(mockUserRepoForContract)
	mockPaymentRepo := new(mockPaymentRepo)
	mockLedgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)

	cfg := &config.Config{ResendAPIKey: "mock-key"}
	emailSvc := NewEmailService(cfg)
	service := NewContractService(mockContractRepo, mockLotRepo, mockUserRepo, mockPaymentRepo, mockLedgerRepo, nil, emailSvc, auditSvc, worker)

	contract := &models.Contract{
		ID:              1,
		LotID:           1,
		Status:          models.ContractStatusPending,
		ApplicantUserID: 2,
	}

	lot := &models.Lot{
		ID:     1,
		Status: models.LotStatusReserved,
	}

	mockContractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	mockContractRepo.On("Update", ctx, mock.MatchedBy(func(c *models.Contract) bool {
		return c.Status == models.ContractStatusRejected && *c.RejectionReason == "Bad credit"
	})).Return(nil)

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusAvailable
	})).Return(nil)

	result, err := service.Reject(ctx, 1, "Bad credit")

	assert.NoError(t, err)
	assert.Equal(t, models.ContractStatusRejected, result.Status)

	mockContractRepo.AssertExpectations(t)
	mockLotRepo.AssertExpectations(t)
}

func TestContractService_Cancel(t *testing.T) {
	// Initialize logger to avoid nil pointer dereference
	logger.Setup("test")

	ctx := context.Background()
	mockContractRepo := new(mockContractRepo)
	mockLotRepo := new(mockLotRepo)
	mockUserRepo := new(mockUserRepoForContract)
	mockPaymentRepo := new(mockPaymentRepo)
	mockLedgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)

	cfg := &config.Config{ResendAPIKey: "mock-key"}
	emailSvc := NewEmailService(cfg)
	service := NewContractService(mockContractRepo, mockLotRepo, mockUserRepo, mockPaymentRepo, mockLedgerRepo, nil, emailSvc, auditSvc, worker)

	contract := &models.Contract{
		ID:              1,
		LotID:           1,
		Status:          models.ContractStatusSubmitted,
		ApplicantUserID: 2,
		Payments: []models.Payment{
			{ID: 1, Status: models.PaymentStatusPending},
			{ID: 2, Status: models.PaymentStatusPaid},
		},
	}

	lot := &models.Lot{
		ID:     1,
		Status: models.LotStatusFinanced,
	}

	mockContractRepo.On("FindByIDWithDetails", ctx, uint(1)).Return(contract, nil)
	mockContractRepo.On("Update", ctx, mock.MatchedBy(func(c *models.Contract) bool {
		return c.Status == models.ContractStatusCancelled && *c.Note == "User requested"
	})).Return(nil)

	mockPaymentRepo.On("Delete", ctx, uint(1)).Return(nil)
	mockLedgerRepo.On("DeleteByContractID", ctx, uint(1)).Return(nil)

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.Status == models.LotStatusAvailable
	})).Return(nil)

	result, err := service.Cancel(ctx, 1, "User requested")

	assert.NoError(t, err)
	assert.Equal(t, models.ContractStatusCancelled, result.Status)

	mockContractRepo.AssertExpectations(t)
	mockLotRepo.AssertExpectations(t)
	mockPaymentRepo.AssertExpectations(t)
	mockLedgerRepo.AssertExpectations(t)
}

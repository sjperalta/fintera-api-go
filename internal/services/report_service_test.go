package services

import (
	"context"
	"encoding/csv"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
)

// Mock PaymentRepository
type mockPaymentRepository struct {
	repository.PaymentRepository
	mockList func(ctx context.Context, query *repository.ListQuery) ([]models.Payment, int64, error)
}

func (m *mockPaymentRepository) List(ctx context.Context, query *repository.ListQuery) ([]models.Payment, int64, error) {
	if m.mockList != nil {
		return m.mockList(ctx, query)
	}
	return nil, 0, nil
}

// Satisfy other interface methods with no-ops or panics if not used
func (m *mockPaymentRepository) FindByID(ctx context.Context, id uint) (*models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepository) FindByContract(ctx context.Context, contractID uint) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	return nil
}
func (m *mockPaymentRepository) Update(ctx context.Context, payment *models.Payment) error {
	return nil
}
func (m *mockPaymentRepository) Delete(ctx context.Context, id uint) error { return nil }
func (m *mockPaymentRepository) DeleteByContract(ctx context.Context, contractID uint) error {
	return nil
}
func (m *mockPaymentRepository) FindOverdue(ctx context.Context) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepository) FindPendingByUser(ctx context.Context, userID uint) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepository) FindPaidByMonth(ctx context.Context, month, year int) ([]models.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepository) GetMonthlyStats(ctx context.Context) (*repository.PaymentStats, error) {
	return nil, nil
}

func TestGenerateRevenueCSV(t *testing.T) {
	mockRepo := &mockPaymentRepository{}
	service := NewReportService(mockRepo, nil, nil) // We only use paymentRepo for this method

	// Setup mock data
	now := time.Now()
	amount := 1000.00
	mockRepo.mockList = func(ctx context.Context, query *repository.ListQuery) ([]models.Payment, int64, error) {
		payments := []models.Payment{
			{
				ID:          1,
				ContractID:  101,
				PaymentType: models.PaymentTypeInstallment,
				PaidAmount:  &amount,
				PaymentDate: &now,
				Contract: models.Contract{
					ID:            101,
					FinancingType: models.FinancingTypeDirect,
					PaymentTerm:   12,
					ApplicantUser: models.User{
						ID:       10,
						FullName: "Juan Perez",
						Identity: "0801-1990-12345",
					},
					Lot: models.Lot{
						ID:   50,
						Name: "Lote 5",
						Project: models.Project{
							ID:   5,
							Name: "Residencial Las Colinas",
						},
					},
				},
			},
			{
				ID:          2,
				ContractID:  102,
				PaymentType: models.PaymentTypeDownPayment,
				PaidAmount:  &amount,
				PaymentDate: &now,
				Contract: models.Contract{
					ID:            102,
					FinancingType: models.FinancingTypeBank,
					PaymentTerm:   24,
					ApplicantUser: models.User{
						ID:       11,
						FullName: "Maria Lopez",
						Identity: "0501-1995-67890",
					},
					Lot: models.Lot{
						ID:   51,
						Name: "Lote 10",
						Project: models.Project{
							ID:   5,
							Name: "Residencial Las Colinas",
						},
					},
				},
			},
		}
		return payments, int64(len(payments)), nil
	}

	// Execute
	buf, err := service.GenerateRevenueCSV(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, buf)

	// Verify CSV Content
	reader := csv.NewReader(buf)
	records, err := reader.ReadAll()
	assert.NoError(t, err)

	// Check Header
	expectedHeader := []string{
		"Pago ID", "Contrato", "Tipo", "Monto Pagado", "Fecha Pago",
		"Cliente", "Identidad", "Proyecto", "Lote",
		"Financiamiento", "Plazo",
	}
	assert.Equal(t, expectedHeader, records[0])

	// Check First Row (Juan Perez)
	row1 := records[1]
	assert.Equal(t, "1", row1[0])
	assert.Equal(t, "101", row1[1])
	assert.Equal(t, "Cuota", row1[2]) // Translated "installment"
	assert.Equal(t, "1000.00", row1[3])
	assert.Equal(t, now.Format("2006-01-02"), row1[4])
	assert.Equal(t, "Juan Perez", row1[5])
	assert.Equal(t, "0801-1990-12345", row1[6])
	assert.Equal(t, "Residencial Las Colinas", row1[7])
	assert.Equal(t, "Lote 5", row1[8])
	assert.Equal(t, "Directo", row1[9]) // Translated "direct"
	assert.Equal(t, "12 meses", row1[10])

	// Check Second Row (Maria Lopez)
	row2 := records[2]
	assert.Equal(t, "Prima", row2[2])    // Translated "down_payment"
	assert.Equal(t, "Bancario", row2[9]) // Translated "bank"
}

// Mock ContractRepository
type mockContractRepository struct {
	repository.ContractRepository
	mockFindByIDWithDetails func(ctx context.Context, id uint) (*models.Contract, error)
}

func (m *mockContractRepository) FindByID(ctx context.Context, id uint) (*models.Contract, error) {
	return nil, nil
}
func (m *mockContractRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.Contract, error) {
	if m.mockFindByIDWithDetails != nil {
		return m.mockFindByIDWithDetails(ctx, id)
	}
	return nil, nil
}
func (m *mockContractRepository) FindByLot(ctx context.Context, lotID uint) ([]models.Contract, error) {
	return nil, nil
}
func (m *mockContractRepository) FindByUser(ctx context.Context, userID uint) ([]models.Contract, error) {
	return nil, nil
}
func (m *mockContractRepository) Create(ctx context.Context, contract *models.Contract) error {
	return nil
}
func (m *mockContractRepository) Update(ctx context.Context, contract *models.Contract) error {
	return nil
}
func (m *mockContractRepository) Delete(ctx context.Context, id uint) error { return nil }
func (m *mockContractRepository) List(ctx context.Context, query *repository.ContractQuery) ([]models.Contract, int64, error) {
	return nil, 0, nil
}
func (m *mockContractRepository) FindActiveByLot(ctx context.Context, lotID uint) (*models.Contract, error) {
	return nil, nil
}
func (m *mockContractRepository) FindPendingReservations(ctx context.Context, olderThan int) ([]models.Contract, error) {
	return nil, nil
}

func TestGenerateCustomerRecordPDF(t *testing.T) {
	mockRepo := &mockContractRepository{}
	service := NewReportService(nil, mockRepo, nil)

	// Setup mock data
	mockRepo.mockFindByIDWithDetails = func(ctx context.Context, id uint) (*models.Contract, error) {
		reserve := 5000.00
		down := 20000.00
		amount := 1000.00
		measureUnit := "V2"

		now := time.Now()

		return &models.Contract{
			ID:            101,
			FinancingType: models.FinancingTypeDirect,
			PaymentTerm:   12,
			ReserveAmount: &reserve,
			DownPayment:   &down,
			CreatedAt:     now,
			ApplicantUser: models.User{
				ID:       10,
				FullName: "Juan Perez",
				Identity: "0801-1990-12345",
				Phone:    "9999-9999",
				Email:    "juan@example.com",
			},
			Lot: models.Lot{
				ID:              50,
				Name:            "Lote 5",
				Length:          20,
				Width:           10,
				Price:           100000,
				MeasurementUnit: &measureUnit,
				Project: models.Project{
					ID:              5,
					Name:            "Residencial Las Colinas",
					MeasurementUnit: "V2",
				},
			},
			Payments: []models.Payment{
				{
					PaymentType: models.PaymentTypeInstallment,
					Amount:      amount,
					DueDate:     now.AddDate(0, 1, 0),
				},
				{
					PaymentType: models.PaymentTypeInstallment,
					Amount:      amount,
					DueDate:     now.AddDate(0, 12, 0), // Last payment
				},
			},
		}, nil
	}

	// Execute
	buf, err := service.GenerateCustomerRecordPDF(context.Background(), 101)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, buf)
	assert.Greater(t, buf.Len(), 0, "PDF buffer should not be empty")
}

func TestGenerateRescissionContractPDF(t *testing.T) {
	mockRepo := &mockContractRepository{}
	service := NewReportService(nil, mockRepo, nil)

	// Setup mock data
	mockRepo.mockFindByIDWithDetails = func(ctx context.Context, id uint) (*models.Contract, error) {
		measureUnit := "V2"
		now := time.Now()
		return &models.Contract{
			ID:        101,
			CreatedAt: now,
			ApplicantUser: models.User{
				ID:       10,
				FullName: "Juan Perez",
				Identity: "0801-1990-12345",
				Address:  func() *string { s := "Col. Kennedy"; return &s }(),
			},
			Lot: models.Lot{
				ID:              50,
				Name:            "Lote 5",
				Length:          20,
				Width:           10,
				MeasurementUnit: &measureUnit,
				Project: models.Project{
					ID:      5,
					Name:    "Residencial Las Colinas",
					Address: "Tegucigalpa",
				},
			},
		}, nil
	}

	// Execute with sample refund and penalty
	buf, err := service.GenerateRescissionContractPDF(context.Background(), 101, 5000.0, 1000.0)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, buf)
	assert.Greater(t, buf.Len(), 0, "PDF buffer should not be empty")
}

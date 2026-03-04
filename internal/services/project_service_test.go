package services

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Repositories

type mockProjectRepo struct {
	mock.Mock
}

func (m *mockProjectRepo) FindByID(ctx context.Context, id uint) (*models.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *mockProjectRepo) List(ctx context.Context, query *repository.ListQuery) ([]models.Project, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]models.Project), args.Get(1).(int64), args.Error(2)
}

func (m *mockProjectRepo) FindAll(ctx context.Context) ([]models.Project, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Project), args.Error(1)
}

func (m *mockProjectRepo) Create(ctx context.Context, project *models.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *mockProjectRepo) Update(ctx context.Context, project *models.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *mockProjectRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockLotRepo struct {
	mock.Mock
}

func (m *mockLotRepo) FindByID(ctx context.Context, id uint) (*models.Lot, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Lot), args.Error(1)
}

func (m *mockLotRepo) FindByProject(ctx context.Context, projectID uint) ([]models.Lot, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]models.Lot), args.Error(1)
}

func (m *mockLotRepo) List(ctx context.Context, projectID uint, query *repository.ListQuery) ([]models.Lot, int64, error) {
	args := m.Called(ctx, projectID, query)
	return args.Get(0).([]models.Lot), args.Get(1).(int64), args.Error(2)
}

func (m *mockLotRepo) Create(ctx context.Context, lot *models.Lot) error {
	args := m.Called(ctx, lot)
	return args.Error(0)
}

func (m *mockLotRepo) Update(ctx context.Context, lot *models.Lot) error {
	args := m.Called(ctx, lot)
	return args.Error(0)
}

func (m *mockLotRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockLotRepo) BulkUpdate(ctx context.Context, lots []models.Lot) error {
	args := m.Called(ctx, lots)
	return args.Error(0)
}

func TestProjectService_Create(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewProjectService(mockProjRepo, mockLotRepo, auditSvc)

	project := &models.Project{
		Name:               "Test Project",
		LotCount:           2,
		PricePerSquareUnit: 100,
		MeasurementUnit:    "m2",
	}

	mockProjRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Project")).Return(nil)

	err := service.Create(ctx, project, 1)

	assert.NoError(t, err)
	assert.NotEmpty(t, project.GUID)
	assert.Len(t, project.Lots, 2)
	assert.Equal(t, "Lote 001", project.Lots[0].Name)
	assert.Equal(t, models.LotStatusAvailable, project.Lots[0].Status)
	assert.Equal(t, 20000.0, project.Lots[0].Price) // 20 * 10 * 100
	assert.Equal(t, "m2", *project.Lots[0].MeasurementUnit)

	mockProjRepo.AssertExpectations(t)
}

func TestProjectService_Update(t *testing.T) {
	ctx := context.Background()
	mockProjRepo := new(mockProjectRepo)
	mockLotRepo := new(mockLotRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewProjectService(mockProjRepo, mockLotRepo, auditSvc)

	existingProject := &models.Project{
		ID:                 1,
		GUID:               "existing-guid",
		MeasurementUnit:    "m2",
		PricePerSquareUnit: 100,
	}

	updatedProject := &models.Project{
		ID:                 1,
		MeasurementUnit:    "v2",
		PricePerSquareUnit: 150,
	}

	mu := "m2"
	lots := []models.Lot{
		{
			ID:              1,
			Status:          models.LotStatusAvailable,
			Length:          20,
			Width:           10,
			Price:           20000,
			MeasurementUnit: &mu,
		},
		{
			ID:              2,
			Status:          models.LotStatusReserved, // Should not be updated
			Length:          20,
			Width:           10,
			Price:           20000,
			MeasurementUnit: &mu,
		},
	}

	mockProjRepo.On("FindByID", ctx, uint(1)).Return(existingProject, nil)
	mockLotRepo.On("FindByProject", ctx, uint(1)).Return(lots, nil)
	mockLotRepo.On("Update", ctx, mock.MatchedBy(func(l *models.Lot) bool {
		return l.ID == 1 && *l.MeasurementUnit == "v2" && l.Price == 30000 // 200 * 150
	})).Return(nil)
	mockProjRepo.On("Update", ctx, updatedProject).Return(nil)

	err := service.Update(ctx, updatedProject, 1)

	assert.NoError(t, err)
	assert.Equal(t, "existing-guid", updatedProject.GUID)

	mockProjRepo.AssertExpectations(t)
	mockLotRepo.AssertExpectations(t)
}

func TestLotService_Create(t *testing.T) {
	ctx := context.Background()
	mockLotRepo := new(mockLotRepo)
	mockProjRepo := new(mockProjectRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewLotService(mockLotRepo, mockProjRepo, auditSvc)

	lot := &models.Lot{
		Name:      "New Lot",
		ProjectID: 1,
	}

	project := &models.Project{
		ID:       1,
		LotCount: 5,
		Name:     "Project 1",
	}

	mockLotRepo.On("Create", ctx, lot).Return(nil)
	mockProjRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	// Expect lot count to be incremented
	mockProjRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.LotCount == 6
	})).Return(nil)

	err := service.Create(ctx, lot, 1)

	assert.NoError(t, err)

	mockLotRepo.AssertExpectations(t)
	mockProjRepo.AssertExpectations(t)
}

func TestLotService_Delete(t *testing.T) {
	ctx := context.Background()
	mockLotRepo := new(mockLotRepo)
	mockProjRepo := new(mockProjectRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery("INSERT INTO \"audit_logs\"").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	auditSvc := NewAuditService(gdb)
	service := NewLotService(mockLotRepo, mockProjRepo, auditSvc)

	lot := &models.Lot{
		ID:        1,
		ProjectID: 1,
	}

	project := &models.Project{
		ID:       1,
		LotCount: 5,
	}

	mockLotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	mockLotRepo.On("Delete", ctx, uint(1)).Return(nil)
	mockProjRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	// Expect lot count to be decremented
	mockProjRepo.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.LotCount == 4
	})).Return(nil)

	err := service.Delete(ctx, 1, 1)

	assert.NoError(t, err)

	mockLotRepo.AssertExpectations(t)
	mockProjRepo.AssertExpectations(t)
}

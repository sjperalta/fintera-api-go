package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

type ProjectService struct {
	repo repository.ProjectRepository
}

func NewProjectService(repo repository.ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) FindByID(ctx context.Context, id uint) (*models.Project, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *ProjectService) List(ctx context.Context, query *repository.ListQuery) ([]models.Project, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *ProjectService) Create(ctx context.Context, project *models.Project) error {
	// Auto-generate GUID if not provided
	if project.GUID == "" {
		project.GUID = uuid.New().String()
	}

	// Auto-generate lots if lot count is specified
	if project.LotCount > 0 {
		project.Lots = make([]models.Lot, 0, project.LotCount)
		for i := 1; i <= project.LotCount; i++ {
			lot := models.Lot{
				ProjectID:       project.ID, // This will be handled by GORM on create
				Name:            "Lote " + fmt.Sprintf("%03d", i),
				Status:          models.LotStatusAvailable,
				Length:          20.0,
				Width:           10.0,
				Price:           project.PricePerSquareUnit * 200.0, // Assuming 20x10=200 area
				MeasurementUnit: &project.MeasurementUnit,
			}
			project.Lots = append(project.Lots, lot)
		}
	}
	return s.repo.Create(ctx, project)
}

func (s *ProjectService) Update(ctx context.Context, project *models.Project) error {
	return s.repo.Update(ctx, project)
}

func (s *ProjectService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

type LotService struct {
	repo        repository.LotRepository
	projectRepo repository.ProjectRepository
}

func NewLotService(repo repository.LotRepository, projectRepo repository.ProjectRepository) *LotService {
	return &LotService{repo: repo, projectRepo: projectRepo}
}

func (s *LotService) FindByID(ctx context.Context, id uint) (*models.Lot, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *LotService) FindByProject(ctx context.Context, projectID uint) ([]models.Lot, error) {
	return s.repo.FindByProject(ctx, projectID)
}

func (s *LotService) List(ctx context.Context, projectID uint, query *repository.ListQuery) ([]models.Lot, int64, error) {
	return s.repo.List(ctx, projectID, query)
}

func (s *LotService) Create(ctx context.Context, lot *models.Lot) error {
	return s.repo.Create(ctx, lot)
}

func (s *LotService) Update(ctx context.Context, lot *models.Lot) error {
	existingLot, err := s.repo.FindByID(ctx, lot.ID)
	if err != nil {
		return err
	}

	// Preserve critical fields if not provided
	if lot.ProjectID == 0 {
		lot.ProjectID = existingLot.ProjectID
	}
	if lot.Status == "" {
		lot.Status = existingLot.Status
	}
	// Preserve other fields if necessary, or rely on frontend to send them
	// Ideally we should use a PATCH approach, but for now this fixes the critical FK error

	return s.repo.Update(ctx, lot)
}

func (s *LotService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

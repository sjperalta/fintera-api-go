package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

type ProjectService struct {
	repo     repository.ProjectRepository
	lotRepo  repository.LotRepository
	auditSvc *AuditService
}

func NewProjectService(repo repository.ProjectRepository, lotRepo repository.LotRepository, auditSvc *AuditService) *ProjectService {
	return &ProjectService{repo: repo, lotRepo: lotRepo, auditSvc: auditSvc}
}

func (s *ProjectService) FindByID(ctx context.Context, id uint) (*models.Project, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *ProjectService) List(ctx context.Context, query *repository.ListQuery) ([]models.Project, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *ProjectService) Create(ctx context.Context, project *models.Project, actorID uint) error {
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
	if err := s.repo.Create(ctx, project); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "CREATE", "Project", project.ID, fmt.Sprintf("Proyecto creado: %s. %d lotes generados.", project.Name, project.LotCount), "", "")
}

func (s *ProjectService) Update(ctx context.Context, project *models.Project, actorID uint) error {
	existing, err := s.repo.FindByID(ctx, project.ID)
	if err != nil {
		return err
	}

	// Check if any of these fields changed: Unidad de Medida, Precio por Unidad, Tasa de Interés, Tasa de Comisión
	measurementUnitChanged := existing.MeasurementUnit != project.MeasurementUnit
	pricePerUnitChanged := existing.PricePerSquareUnit != project.PricePerSquareUnit
	interestRateChanged := existing.InterestRate != project.InterestRate
	commissionRateChanged := existing.CommissionRate != project.CommissionRate
	rateFieldsChanged := measurementUnitChanged || pricePerUnitChanged || interestRateChanged || commissionRateChanged

	if rateFieldsChanged {
		lots, err := s.lotRepo.FindByProject(ctx, project.ID)
		if err != nil {
			return err
		}
		mu := project.MeasurementUnit
		for i := range lots {
			if lots[i].Status != models.LotStatusAvailable {
				continue
			}
			if measurementUnitChanged {
				lots[i].MeasurementUnit = &mu
			}
			if pricePerUnitChanged && (lots[i].OverridePrice == nil || *lots[i].OverridePrice == 0) {
				lots[i].Price = lots[i].Area() * project.PricePerSquareUnit
			}
			if err := s.lotRepo.Update(ctx, &lots[i]); err != nil {
				return err
			}
		}
	}

	if err := s.repo.Update(ctx, project); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "UPDATE", "Project", project.ID, fmt.Sprintf("Proyecto actualizado: %s", project.Name), "", "")
}

func (s *ProjectService) Delete(ctx context.Context, id uint, actorID uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "DELETE", "Project", id, "Proyecto eliminado", "", "")
}

type LotService struct {
	repo        repository.LotRepository
	projectRepo repository.ProjectRepository
	auditSvc    *AuditService
}

func NewLotService(repo repository.LotRepository, projectRepo repository.ProjectRepository, auditSvc *AuditService) *LotService {
	return &LotService{repo: repo, projectRepo: projectRepo, auditSvc: auditSvc}
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

func (s *LotService) Create(ctx context.Context, lot *models.Lot, actorID uint) error {
	if err := s.repo.Create(ctx, lot); err != nil {
		return err
	}
	// Keep project lot_count in sync
	project, err := s.projectRepo.FindByID(ctx, lot.ProjectID)
	if err != nil {
		return err
	}
	project.LotCount++
	if err := s.projectRepo.Update(ctx, project); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "CREATE", "Lot", lot.ID, fmt.Sprintf("Lote creado: %s en Proyecto %s", lot.Name, project.Name), "", "")
}

func (s *LotService) Update(ctx context.Context, lot *models.Lot, actorID uint) error {
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

	// Preserve metadata fields if not provided (zero/nil)
	if lot.Address == nil {
		lot.Address = existingLot.Address
	}
	if lot.MeasurementUnit == nil {
		lot.MeasurementUnit = existingLot.MeasurementUnit
	}
	if lot.RegistrationNumber == nil {
		lot.RegistrationNumber = existingLot.RegistrationNumber
	}
	if lot.Note == nil {
		lot.Note = existingLot.Note
	}
	if lot.North == nil {
		lot.North = existingLot.North
	}
	if lot.South == nil {
		lot.South = existingLot.South
	}
	if lot.East == nil {
		lot.East = existingLot.East
	}
	if lot.West == nil {
		lot.West = existingLot.West
	}

	// Carry over project association for ToResponse
	lot.Project = existingLot.Project

	// Recalculate base price if 0
	if lot.Price == 0 && lot.Project.PricePerSquareUnit > 0 {
		lot.Price = lot.Area() * lot.Project.PricePerSquareUnit
	}

	// Ideally we should use a PATCH approach, but for now this fixes the critical data loss
	if err := s.repo.Update(ctx, lot); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "UPDATE", "Lot", lot.ID, fmt.Sprintf("Lote actualizado: %s", lot.Name), "", "")
}

func (s *LotService) Delete(ctx context.Context, id uint, actorID uint) error {
	lot, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	projectID := lot.ProjectID
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// Keep project lot_count in sync
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if project.LotCount > 0 {
		project.LotCount--
		if err := s.projectRepo.Update(ctx, project); err != nil {
			return err
		}
	}
	return s.auditSvc.Log(ctx, actorID, "DELETE", "Lot", id, "Lote eliminado", "", "")
}

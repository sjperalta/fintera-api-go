package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

type AnalyticsService struct {
	analyticsRepo   repository.AnalyticsRepository
	projectRepo     repository.ProjectRepository
	notificationSvc *NotificationService
	userRepo        repository.UserRepository
}

func NewAnalyticsService(
	analyticsRepo repository.AnalyticsRepository,
	projectRepo repository.ProjectRepository,
	notificationSvc *NotificationService,
	userRepo repository.UserRepository,
) *AnalyticsService {
	return &AnalyticsService{
		analyticsRepo:   analyticsRepo,
		projectRepo:     projectRepo,
		notificationSvc: notificationSvc,
		userRepo:        userRepo,
	}
}

type AnalyticsFilters struct {
	ProjectID        *uint
	StartDate        *time.Time
	EndDate          *time.Time
	RevenueTimeframe string
	Year             *int
}

func (s *AnalyticsService) GetOverview(ctx context.Context, filters AnalyticsFilters) (*models.AnalyticsOverview, error) {
	cacheKey := "analytics_overview"
	if filters.RevenueTimeframe != "" {
		cacheKey += "_" + filters.RevenueTimeframe
	}

	// Check cache
	cached, err := s.analyticsRepo.GetCache(ctx, cacheKey, filters.ProjectID)
	if err == nil && cached != nil {
		var overview models.AnalyticsOverview
		if err := json.Unmarshal(cached.Data, &overview); err == nil {
			return &overview, nil
		}
	}

	// Compute overview
	overview, err := s.computeOverview(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Set cache (15 min TTL)
	_ = s.analyticsRepo.SetCache(ctx, cacheKey, filters.ProjectID, overview, 15*time.Minute)

	return overview, nil
}

// calculatePercentageChange computes the percentage difference between current and previous values
func calculatePercentageChange(current, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	change := ((current - previous) / previous) * 100
	// Round to 1 decimal place
	return float64(int(change*10+0.5)) / 10
}

// getPreviousPeriod calculates the previous period date range based on current filters
func getPreviousPeriod(startDate, endDate *time.Time) (prevStart, prevEnd *time.Time) {
	if startDate == nil || endDate == nil {
		// Default to 30 days if no dates provided
		now := time.Now()
		currentStart := now.AddDate(0, 0, -30)

		prevEnd := currentStart.AddDate(0, 0, -1) // Day before current start
		prevStart := prevEnd.AddDate(0, 0, -30)

		return &prevStart, &prevEnd
	}

	// Calculate the duration of the current period
	duration := endDate.Sub(*startDate)

	// Previous period ends one day before current start
	prevEndTime := startDate.AddDate(0, 0, -1)
	prevStartTime := prevEndTime.Add(-duration)

	return &prevStartTime, &prevEndTime
}

func (s *AnalyticsService) computeOverview(ctx context.Context, filters AnalyticsFilters) (*models.AnalyticsOverview, error) {
	// Current period stats
	totalRevenue, err := s.analyticsRepo.GetTotalRevenue(ctx, filters.ProjectID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, err
	}

	activeContracts, err := s.analyticsRepo.GetActiveContractsCount(ctx, filters.ProjectID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, err
	}

	avgPayment, err := s.analyticsRepo.GetAveragePayment(ctx, filters.ProjectID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, err
	}

	occupancyRate, err := s.analyticsRepo.GetOccupancyRate(ctx, filters.ProjectID)
	if err != nil {
		return nil, err
	}

	revenueTrend, err := s.analyticsRepo.GetRevenueTrend(ctx, filters.ProjectID, filters.RevenueTimeframe, filters.Year)
	if err != nil {
		return nil, err
	}

	// Get previous period data for percentage calculations
	prevStart, prevEnd := getPreviousPeriod(filters.StartDate, filters.EndDate)

	prevRevenue, err := s.analyticsRepo.GetTotalRevenue(ctx, filters.ProjectID, prevStart, prevEnd)
	if err != nil {
		prevRevenue = 0
	}

	prevContracts, err := s.analyticsRepo.GetActiveContractsCount(ctx, filters.ProjectID, prevStart, prevEnd)
	if err != nil {
		prevContracts = 0
	}

	prevAvgPayment, err := s.analyticsRepo.GetAveragePayment(ctx, filters.ProjectID, prevStart, prevEnd)
	if err != nil {
		prevAvgPayment = 0
	}

	// Note: Occupancy rate is not time-based, so we compare against a baseline
	// For now, we'll use 0 as previous to show absolute percentage
	prevOccupancy := 0.0

	// Calculate percentage changes
	revenueChange := calculatePercentageChange(totalRevenue, prevRevenue)
	contractsChange := calculatePercentageChange(float64(activeContracts), float64(prevContracts))
	paymentChange := calculatePercentageChange(avgPayment, prevAvgPayment)
	occupancyChange := calculatePercentageChange(occupancyRate, prevOccupancy)

	// Default currency symbol (can be project-specific in the future)
	currencySymbol := "L"

	return &models.AnalyticsOverview{
		TotalRevenue:              totalRevenue,
		RevenueChangePercentage:   revenueChange,
		ActiveContracts:           activeContracts,
		ContractsChangePercentage: contractsChange,
		AveragePayment:            avgPayment,
		PaymentChangePercentage:   paymentChange,
		OccupancyRate:             occupancyRate,
		OccupancyChangePercentage: occupancyChange,
		CurrencySymbol:            currencySymbol,
		RevenueTrend:              revenueTrend,
	}, nil
}

func (s *AnalyticsService) GetDistribution(ctx context.Context, projectID *uint) (*models.LotDistribution, error) {
	cacheKey := "analytics_distribution"

	// Check cache
	cached, err := s.analyticsRepo.GetCache(ctx, cacheKey, projectID)
	if err == nil && cached != nil {
		var dist models.LotDistribution
		if err := json.Unmarshal(cached.Data, &dist); err == nil {
			return &dist, nil
		}
	}

	dist, err := s.analyticsRepo.GetLotDistribution(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Set cache (15 min TTL)
	_ = s.analyticsRepo.SetCache(ctx, cacheKey, projectID, dist, 15*time.Minute)

	return dist, nil
}

func (s *AnalyticsService) GetPerformance(ctx context.Context, filters AnalyticsFilters) ([]models.ProjectPerformance, error) {
	cacheKey := "analytics_performance"
	if filters.ProjectID != nil {
		cacheKey += fmt.Sprintf("_project_%d", *filters.ProjectID)
	}

	// Check cache
	cached, err := s.analyticsRepo.GetCache(ctx, cacheKey, filters.ProjectID)
	if err == nil && cached != nil {
		var perf []models.ProjectPerformance
		if err := json.Unmarshal(cached.Data, &perf); err == nil {
			return perf, nil
		}
	}

	perf, err := s.analyticsRepo.GetProjectPerformance(ctx, filters.ProjectID, filters.StartDate, filters.EndDate, filters.Year, filters.RevenueTimeframe)
	if err != nil {
		return nil, err
	}

	// Set cache (30 min TTL)
	_ = s.analyticsRepo.SetCache(ctx, cacheKey, filters.ProjectID, perf, 30*time.Minute)

	return perf, nil
}

func (s *AnalyticsService) RefreshCache(ctx context.Context) error {
	logger.Info("[AnalyticsService] Refreshing analytics cache in background...")

	// Notify admins about start
	s.notifyAdmins(ctx, "Actualización de Estadísticas", "Se ha iniciado el proceso de actualización de analíticas en segundo plano.")

	// Get all projects to refresh their specific caches
	projects, err := s.projectRepo.FindAll(ctx)
	if err != nil {
		logger.Error("Failed to fetch projects for cache refresh", "error", err)
		return err
	}

	// Refresh global stats
	_, _ = s.GetOverview(ctx, AnalyticsFilters{})
	_, _ = s.GetDistribution(ctx, nil)
	_, _ = s.GetPerformance(ctx, AnalyticsFilters{})

	// Refresh each project stats
	for _, p := range projects {
		pid := p.ID
		_, _ = s.GetOverview(ctx, AnalyticsFilters{ProjectID: &pid})
		_, _ = s.GetDistribution(ctx, &pid)
	}

	// Clean up old cache
	_ = s.analyticsRepo.CleanExpiredCache(ctx)

	logger.Info("[AnalyticsService] Analytics cache refresh completed.")

	// Notify admins about completion
	_ = s.notificationSvc.NotifyAdmins(ctx, "Estadísticas Actualizadas", "La actualización de analíticas ha finalizado correctamente.", "system")

	return nil
}

func (s *AnalyticsService) notifyAdmins(ctx context.Context, title, message string) {
	_ = s.notificationSvc.NotifyAdmins(ctx, title, message, "system")
}

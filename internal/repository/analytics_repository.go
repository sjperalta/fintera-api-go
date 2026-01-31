package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

type AnalyticsRepository interface {
	GetCache(ctx context.Context, key string, projectID *uint) (*models.AnalyticsCache, error)
	SetCache(ctx context.Context, key string, projectID *uint, data interface{}, ttl time.Duration) error
	InvalidateCache(ctx context.Context, key string, projectID *uint) error
	CleanExpiredCache(ctx context.Context) error

	// Data retrieval for analytics
	GetTotalRevenue(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error)
	GetActiveContractsCount(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (int, error)
	GetAveragePayment(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error)
	GetOccupancyRate(ctx context.Context, projectID *uint) (float64, error)
	GetRevenueTrend(ctx context.Context, projectID *uint, timeframe string, year *int) ([]models.RevenueTrendPoint, error)
	GetLotDistribution(ctx context.Context, projectID *uint) (*models.LotDistribution, error)
	GetProjectPerformance(ctx context.Context, projectID *uint, startDate, endDate *time.Time, year *int, revenueTimeframe string) ([]models.ProjectPerformance, error)
}

type analyticsRepository struct {
	db *gorm.DB
}

func NewAnalyticsRepository(db *gorm.DB) AnalyticsRepository {
	return &analyticsRepository{db: db}
}

func (r *analyticsRepository) GetCache(ctx context.Context, key string, projectID *uint) (*models.AnalyticsCache, error) {
	var cache models.AnalyticsCache
	db := r.db.WithContext(ctx).Where("cache_key = ?", key)
	if projectID != nil {
		db = db.Where("project_id = ?", *projectID)
	} else {
		db = db.Where("project_id IS NULL")
	}

	err := db.Where("expires_at > ?", time.Now()).First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

func (r *analyticsRepository) SetCache(ctx context.Context, key string, projectID *uint, data interface{}, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	cache := models.AnalyticsCache{
		CacheKey:  key,
		ProjectID: projectID,
		Data:      jsonData,
		ExpiresAt: time.Now().Add(ttl),
	}

	// Upsert strategy
	var existing models.AnalyticsCache
	db := r.db.WithContext(ctx).Where("cache_key = ?", key)
	if projectID != nil {
		db = db.Where("project_id = ?", *projectID)
	} else {
		db = db.Where("project_id IS NULL")
	}

	err = db.First(&existing).Error
	if err == nil {
		return r.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"data":       jsonData,
			"expires_at": cache.ExpiresAt,
			"updated_at": time.Now(),
		}).Error
	}

	return r.db.WithContext(ctx).Create(&cache).Error
}

func (r *analyticsRepository) InvalidateCache(ctx context.Context, key string, projectID *uint) error {
	db := r.db.WithContext(ctx).Where("cache_key = ?", key)
	if projectID != nil {
		db = db.Where("project_id = ?", *projectID)
	}
	return db.Delete(&models.AnalyticsCache{}).Error
}

func (r *analyticsRepository) CleanExpiredCache(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at <= ?", time.Now()).Delete(&models.AnalyticsCache{}).Error
}

// Data retrieval implementations

func (r *analyticsRepository) GetTotalRevenue(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error) {
	var total float64
	query := r.db.WithContext(ctx).Table("payments").
		Select("COALESCE(SUM(paid_amount), 0)").
		Where("payments.status = ?", models.PaymentStatusPaid)

	if projectID != nil {
		query = query.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	if startDate != nil {
		query = query.Where("payments.payment_date >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("payment_date <= ?", *endDate)
	}

	err := query.Scan(&total).Error
	return total, err
}

func (r *analyticsRepository) GetActiveContractsCount(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (int, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Contract{}).
		Where("contracts.status = ?", models.ContractStatusApproved).
		Where("contracts.active = ?", true)

	if projectID != nil {
		query = query.Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	if startDate != nil {
		query = query.Where("contracts.approved_at >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("contracts.approved_at <= ?", *endDate)
	}

	err := query.Count(&count).Error
	return int(count), err
}

func (r *analyticsRepository) GetAveragePayment(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error) {
	var avg float64
	query := r.db.WithContext(ctx).Table("payments").
		Select("COALESCE(AVG(paid_amount), 0)").
		Where("payments.status = ?", models.PaymentStatusPaid)

	if projectID != nil {
		query = query.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	if startDate != nil {
		query = query.Where("payments.payment_date >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("payments.payment_date <= ?", *endDate)
	}

	err := query.Scan(&avg).Error
	return avg, err
}

func (r *analyticsRepository) GetOccupancyRate(ctx context.Context, projectID *uint) (float64, error) {
	var total, occupied int64

	lotQuery := r.db.WithContext(ctx).Model(&models.Lot{})
	if projectID != nil {
		lotQuery = lotQuery.Where("project_id = ?", *projectID)
	}

	err := lotQuery.Count(&total).Error
	if err != nil || total == 0 {
		return 0, err
	}

	err = lotQuery.Where("lots.status IN ?", []string{models.LotStatusReserved, models.LotStatusFinanced, models.LotStatusFullyPaid}).Count(&occupied).Error
	if err != nil {
		return 0, err
	}

	return (float64(occupied) / float64(total)) * 100, nil
}

func (r *analyticsRepository) GetRevenueTrend(ctx context.Context, projectID *uint, timeframe string, year *int) ([]models.RevenueTrendPoint, error) {
	var points []models.RevenueTrendPoint

	var startDate, endDate time.Time
	if year != nil {
		startDate = time.Date(*year, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(*year, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		// Default to last 12 months if not specified
		months := 12
		if timeframe == "6M" {
			months = 6
		}
		// Calculate start date
		startDate = time.Now().AddDate(0, -months, 0)
		endDate = time.Now().AddDate(0, months, 0) // Future projections within same timeframe
	}

	// Real revenue (paid payments)
	var realResults []struct {
		Label string
		Total float64
	}

	realQuery := r.db.WithContext(ctx).Table("payments").
		Select("TO_CHAR(payments.payment_date, 'Mon') as label, SUM(payments.paid_amount) as total, MIN(payments.payment_date) as sort_date").
		Where("payments.status = ?", models.PaymentStatusPaid).
		Where("payments.payment_date >= ?", startDate).
		Where("payments.payment_date <= ?", endDate).
		Group("TO_CHAR(payments.payment_date, 'Mon')").
		Order("sort_date ASC")

	if projectID != nil {
		realQuery = realQuery.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	err := realQuery.Scan(&realResults).Error
	if err != nil {
		return nil, err
	}

	// Projected revenue (pending payments for future months)
	var projectedResults []struct {
		Label string
		Total float64
	}

	projectedQuery := r.db.WithContext(ctx).Table("payments").
		Select("TO_CHAR(payments.due_date, 'Mon') as label, SUM(payments.amount) as total, MIN(payments.due_date) as sort_date").
		Where("payments.status = ?", models.PaymentStatusPending).
		Where("payments.due_date >= ?", startDate).
		Where("payments.due_date <= ?", endDate).
		Group("TO_CHAR(payments.due_date, 'Mon')").
		Order("sort_date ASC")

	if projectID != nil {
		projectedQuery = projectedQuery.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	err = projectedQuery.Scan(&projectedResults).Error
	if err != nil {
		return nil, err
	}

	// Merge results
	labelMap := make(map[string]*models.RevenueTrendPoint)

	for _, res := range realResults {
		labelMap[res.Label] = &models.RevenueTrendPoint{Label: res.Label, Real: res.Total}
	}

	for _, res := range projectedResults {
		if pt, ok := labelMap[res.Label]; ok {
			pt.Projected = res.Total
		} else {
			labelMap[res.Label] = &models.RevenueTrendPoint{Label: res.Label, Projected: res.Total}
		}
	}

	// Combine all results and sort
	// Include both real and projected data points
	// In a real implementation, we would ensure all months in range are present
	for _, res := range realResults {
		if pt, ok := labelMap[res.Label]; ok {
			points = append(points, *pt)
			delete(labelMap, res.Label) // Remove to avoid duplicates
		}
	}

	// Add remaining projected-only months
	for _, pt := range labelMap {
		points = append(points, *pt)
	}

	return points, nil
}

func (r *analyticsRepository) GetLotDistribution(ctx context.Context, projectID *uint) (*models.LotDistribution, error) {
	var dist models.LotDistribution

	query := r.db.WithContext(ctx).Model(&models.Lot{})
	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	var results []struct {
		Status string
		Count  int
	}

	err := query.Select("status, COUNT(*) as count").Group("status").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	for _, res := range results {
		dist.TotalLots += res.Count
		switch res.Status {
		case models.LotStatusAvailable:
			dist.Available = res.Count
		case models.LotStatusReserved:
			dist.Reserved = res.Count
		case models.LotStatusFinanced:
			dist.Financed = res.Count
		case models.LotStatusFullyPaid:
			dist.FullyPaid = res.Count
		}
	}

	// Calculate percentages
	if dist.TotalLots > 0 {
		dist.AvailablePercentage = (float64(dist.Available) / float64(dist.TotalLots)) * 100
		dist.ReservedPercentage = (float64(dist.Reserved) / float64(dist.TotalLots)) * 100
		dist.FinancedPercentage = (float64(dist.Financed) / float64(dist.TotalLots)) * 100
		dist.FullyPaidPercentage = (float64(dist.FullyPaid) / float64(dist.TotalLots)) * 100
	}

	return &dist, nil
}

func (r *analyticsRepository) GetProjectPerformance(ctx context.Context, projectID *uint, startDate, endDate *time.Time, year *int, revenueTimeframe string) ([]models.ProjectPerformance, error) {
	query := r.db.WithContext(ctx).Preload("Lots")
	if projectID != nil {
		query = query.Where("id = ?", *projectID)
	}
	var projects []models.Project
	err := query.Find(&projects).Error
	if err != nil {
		return nil, err
	}

	var performance []models.ProjectPerformance
	for _, p := range projects {
		var totalLots, soldLots int
		totalLots = p.LotCount
		if totalLots == 0 {
			totalLots = len(p.Lots)
		}

		for _, lot := range p.Lots {
			if lot.Status == models.LotStatusFinanced || lot.Status == models.LotStatusFullyPaid {
				soldLots++
			}
		}

		growth := 0.0
		if totalLots > 0 {
			growth = (float64(soldLots) / float64(totalLots)) * 100
		}

		// Mock weekly performance for now or calculate real ones from payments
		performance = append(performance, models.ProjectPerformance{
			ProjectID:         strconv.FormatUint(uint64(p.ID), 10),
			ProjectName:       p.Name,
			GrowthPercentage:  growth,
			WeeklyPerformance: []float64{65, 59, 80, 81}, // Mock weekly data for now
		})
	}

	return performance, nil
}

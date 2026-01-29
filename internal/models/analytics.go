package models

import (
	"encoding/json"
	"time"
)

// AnalyticsCache represents a cached analytics result
type AnalyticsCache struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	CacheKey  string          `gorm:"not null;index:idx_analytics_cache_key_project" json:"cache_key"`
	ProjectID *uint           `gorm:"index:idx_analytics_cache_key_project" json:"project_id"`
	Data      json.RawMessage `gorm:"type:jsonb;not null" json:"data"`
	ExpiresAt time.Time       `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// TableName specifies the table name for AnalyticsCache
func (AnalyticsCache) TableName() string {
	return "analytics_cache"
}

// AnalyticsOverview represents high-level statistics and trend data
type AnalyticsOverview struct {
	TotalRevenue              float64             `json:"total_revenue"`
	RevenueChangePercentage   float64             `json:"revenue_change_percentage"`
	ActiveContracts           int                 `json:"active_contracts"`
	ContractsChangePercentage float64             `json:"contracts_change_percentage"`
	AveragePayment            float64             `json:"average_payment"`
	PaymentChangePercentage   float64             `json:"payment_change_percentage"`
	OccupancyRate             float64             `json:"occupancy_rate"`
	OccupancyChangePercentage float64             `json:"occupancy_change_percentage"`
	CurrencySymbol            string              `json:"currency_symbol"`
	RevenueTrend              []RevenueTrendPoint `json:"revenue_trend"`
}

// RevenueTrendPoint represents a data point in the revenue chart
type RevenueTrendPoint struct {
	Label     string  `json:"label"`
	Real      float64 `json:"real"`
	Projected float64 `json:"projected"`
}

// LotDistribution represents availability statistics for lots
type LotDistribution struct {
	TotalLots           int     `json:"total_lots"`
	Financed            int     `json:"financed"`
	FullyPaid           int     `json:"fully_paid"`
	Reserved            int     `json:"reserved"`
	Available           int     `json:"available"`
	FinancedPercentage  float64 `json:"financed_percentage"`
	FullyPaidPercentage float64 `json:"fully_paid_percentage"`
	ReservedPercentage  float64 `json:"reserved_percentage"`
	AvailablePercentage float64 `json:"available_percentage"`
}

// ProjectPerformance represents performance metrics for a project
type ProjectPerformance struct {
	ProjectID         string    `json:"project_id"`
	ProjectName       string    `json:"project_name"`
	GrowthPercentage  float64   `json:"growth_percentage"`
	WeeklyPerformance []float64 `json:"weekly_performance"`
}

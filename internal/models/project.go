package models

import (
	"time"
)

// Project represents a real estate project
type Project struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Name               string    `gorm:"not null" json:"name"`
	Description        string    `gorm:"type:text;not null" json:"description"`
	ProjectType        string    `gorm:"default:residential" json:"project_type"`
	Address            string    `gorm:"not null" json:"address"`
	LotCount           int       `gorm:"not null" json:"lot_count"`
	PricePerSquareUnit float64   `gorm:"type:decimal(10,2);not null" json:"price_per_square_unit"`
	InterestRate       float64   `gorm:"type:decimal(5,2);not null" json:"interest_rate"`
	GUID               string    `gorm:"column:guid;not null" json:"guid"`
	CommissionRate     float64   `gorm:"type:decimal(5,2);default:0" json:"commission_rate"`
	MeasurementUnit    string    `gorm:"default:m2" json:"measurement_unit"`
	DeliveryDate       *string   `gorm:"type:date" json:"delivery_date"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`

	// Associations
	Lots []Lot `gorm:"foreignKey:ProjectID" json:"lots,omitempty"`
}

// TableName specifies the table name for Project
func (Project) TableName() string {
	return "projects"
}

// ProjectResponse is the JSON response format for projects
type ProjectResponse struct {
	ID                 uint      `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	ProjectType        string    `json:"project_type"`
	Address            string    `json:"address"`
	LotCount           int       `json:"lot_count"`
	PricePerSquareUnit float64   `json:"price_per_square_unit"`
	InterestRate       float64   `json:"interest_rate"`
	CommissionRate     float64   `json:"commission_rate"`
	MeasurementUnit    string    `json:"measurement_unit"`
	DeliveryDate       *string   `json:"delivery_date"`
	AvailableLots      int       `json:"available_lots"`
	ReservedLots       int       `json:"reserved_lots"`
	SoldLots           int       `json:"sold_lots"`
	CreatedAt          time.Time `json:"created_at"`
}

// ToResponse converts Project to ProjectResponse
func (p *Project) ToResponse() ProjectResponse {
	var available, reserved, sold int
	for _, lot := range p.Lots {
		switch lot.Status {
		case LotStatusAvailable:
			available++
		case LotStatusReserved:
			reserved++
		case LotStatusFinanced, LotStatusFullyPaid:
			sold++
		}
	}

	return ProjectResponse{
		ID:                 p.ID,
		Name:               p.Name,
		Description:        p.Description,
		ProjectType:        p.ProjectType,
		Address:            p.Address,
		LotCount:           p.LotCount,
		PricePerSquareUnit: p.PricePerSquareUnit,
		InterestRate:       p.InterestRate,
		CommissionRate:     p.CommissionRate,
		MeasurementUnit:    p.MeasurementUnit,
		DeliveryDate:       p.DeliveryDate,
		AvailableLots:      available,
		ReservedLots:       reserved,
		SoldLots:           sold,
		CreatedAt:          p.CreatedAt,
	}
}

package models

import (
	"time"
)

// Lot represents a lot within a project
type Lot struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	ProjectID          uint      `gorm:"not null;index" json:"project_id"`
	Name               string    `gorm:"not null" json:"name"`
	Status             string    `gorm:"default:available;index" json:"status"`
	Length             float64   `gorm:"type:decimal(10,2);not null" json:"length"`
	Width              float64   `gorm:"type:decimal(10,2);not null" json:"width"`
	Price              float64   `gorm:"type:decimal(15,2);not null" json:"price"`
	Address            *string   `json:"address"`
	MeasurementUnit    *string   `json:"measurement_unit"`
	OverridePrice      *float64  `gorm:"type:decimal(15,2)" json:"override_price"`
	RegistrationNumber *string   `gorm:"index" json:"registration_number"`
	Note               *string   `gorm:"type:text" json:"note"`
	OverrideArea       *float64  `json:"override_area"`
	North              *string   `gorm:"type:text" json:"north"`
	South              *string   `gorm:"type:text" json:"south"`
	East               *string   `gorm:"type:text" json:"east"`
	West               *string   `gorm:"type:text" json:"west"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`

	// Associations
	Project   Project    `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Contracts []Contract `gorm:"foreignKey:LotID" json:"contracts,omitempty"`
}

// TableName specifies the table name for Lot
func (Lot) TableName() string {
	return "lots"
}

// Lot status constants
const (
	LotStatusAvailable = "available"
	LotStatusReserved  = "reserved"
	LotStatusFinanced  = "financed"
	LotStatusFullyPaid = "fully_paid"
	LotStatusActive    = "active"
)

// Area calculates the lot area
func (l *Lot) Area() float64 {
	if l.OverrideArea != nil && *l.OverrideArea > 0 {
		return *l.OverrideArea
	}
	return l.Length * l.Width
}

// EffectivePrice returns the override price if set, otherwise the base price
func (l *Lot) EffectivePrice() float64 {
	if l.OverridePrice != nil && *l.OverridePrice > 0 {
		return *l.OverridePrice
	}
	return l.Price
}

// IsAvailable returns true if the lot can be reserved
func (l *Lot) IsAvailable() bool {
	return l.Status == LotStatusAvailable || l.Status == LotStatusActive
}

// LotResponse is the JSON response format for lots
type LotResponse struct {
	ID                 uint    `json:"id"`
	ProjectID          uint    `json:"project_id"`
	ProjectName        string  `json:"project_name"`
	Name               string  `json:"name"`
	Status             string  `json:"status"`
	Length             float64 `json:"length"`
	Width              float64 `json:"width"`
	Area               float64 `json:"area"`
	Price              float64 `json:"price"`
	EffectivePrice     float64 `json:"effective_price"`
	Address            *string `json:"address"`
	MeasurementUnit    *string `json:"measurement_unit"`
	RegistrationNumber *string `json:"registration_number"`
	Note               *string `json:"note"`
	North              *string `json:"north"`
	South              *string `json:"south"`
	East               *string `json:"east"`
	West               *string `json:"west"`
}

// ToResponse converts Lot to LotResponse
func (l *Lot) ToResponse() LotResponse {
	return LotResponse{
		ID:                 l.ID,
		ProjectID:          l.ProjectID,
		ProjectName:        l.Project.Name,
		Name:               l.Name,
		Status:             l.Status,
		Length:             l.Length,
		Width:              l.Width,
		Area:               l.Area(),
		Price:              l.Price,
		EffectivePrice:     l.EffectivePrice(),
		Address:            l.Address,
		MeasurementUnit:    l.MeasurementUnit,
		RegistrationNumber: l.RegistrationNumber,
		Note:               l.Note,
		North:              l.North,
		South:              l.South,
		East:               l.East,
		West:               l.West,
	}
}

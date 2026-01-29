package models

import (
	"time"
)

// AuditLog represents a system audit entry
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	Action    string    `gorm:"size:50;not null" json:"action"` // CREATE, UPDATE, DELETE, LOGIN, APPROVE
	Entity    string    `gorm:"size:50;not null" json:"entity"` // Contract, User, Payment, etc.
	EntityID  uint      `json:"entity_id"`
	Details   string    `gorm:"type:text" json:"details"` // JSON or text description
	IPAddress string    `gorm:"size:45" json:"ip_address"`
	UserAgent string    `gorm:"size:255" json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name for AuditLog
func (AuditLog) TableName() string {
	return "audit_logs"
}

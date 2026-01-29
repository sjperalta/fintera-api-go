package models

import (
	"time"
)

// Notification represents a user notification
type Notification struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	UserID           uint       `gorm:"not null;index" json:"user_id"`
	Title            string     `gorm:"not null" json:"title"`
	Message          string     `gorm:"not null" json:"message"`
	NotificationType *string    `gorm:"index" json:"notification_type"`
	ReadAt           *time.Time `gorm:"index" json:"read_at"`
	CreatedAt        time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for Notification
func (Notification) TableName() string {
	return "notifications"
}

// Notification type constants
const (
	NotificationTypeContractApproved  = "contract_approved"
	NotificationTypeContractRejected  = "contract_rejected"
	NotificationTypeContractCancelled = "contract_cancelled"
	NotificationTypeContractClosed    = "contract_closed"
	NotificationTypePaymentSubmitted  = "payment_submitted"
	NotificationTypePaymentApproved   = "payment_approved"
	NotificationTypePaymentRejected   = "payment_rejected"
	NotificationTypePaymentOverdue    = "payment_overdue"
	NotificationTypeLotReserved       = "lot_reserved"
	NotificationTypeNewUser           = "create_new_user"
	NotificationTypeSystemError       = "system_error"
)

// IsRead returns true if notification has been read
func (n *Notification) IsRead() bool {
	return n.ReadAt != nil
}

// MarkAsRead marks the notification as read
func (n *Notification) MarkAsRead() {
	now := time.Now()
	n.ReadAt = &now
}

// NotificationResponse is the JSON response format
type NotificationResponse struct {
	ID               uint       `json:"id"`
	Title            string     `json:"title"`
	Message          string     `json:"message"`
	NotificationType *string    `json:"notification_type"`
	Read             bool       `json:"read"`
	ReadAt           *time.Time `json:"read_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ToResponse converts Notification to NotificationResponse
func (n *Notification) ToResponse() NotificationResponse {
	return NotificationResponse{
		ID:               n.ID,
		Title:            n.Title,
		Message:          n.Message,
		NotificationType: n.NotificationType,
		Read:             n.IsRead(),
		ReadAt:           n.ReadAt,
		CreatedAt:        n.CreatedAt,
	}
}

// RefreshToken represents a JWT refresh token
type RefreshToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for RefreshToken
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// IsExpired returns true if the refresh token has expired
func (r *RefreshToken) IsExpired() bool {
	if r.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*r.ExpiresAt)
}

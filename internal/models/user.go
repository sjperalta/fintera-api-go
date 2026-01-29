package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	Email               string     `gorm:"uniqueIndex;not null" json:"email"`
	EncryptedPassword   string     `gorm:"column:encrypted_password;not null" json:"-"`
	ResetPasswordToken  *string    `json:"-"`
	ResetPasswordSentAt *time.Time `json:"-"`
	RememberCreatedAt   *time.Time `json:"-"`
	ConfirmationToken   *string    `json:"-"`
	ConfirmedAt         *time.Time `json:"confirmed_at"`
	ConfirmationSentAt  *time.Time `json:"-"`
	UnconfirmedEmail    *string    `json:"-"`
	Role                string     `gorm:"default:user" json:"role"`
	FullName            string     `json:"full_name"`
	Phone               string     `json:"phone"`
	Status              string     `gorm:"default:active" json:"status"`
	Identity            string     `gorm:"uniqueIndex" json:"identity"`
	RTN                 string     `gorm:"column:rtn;uniqueIndex" json:"rtn"`
	DiscardedAt         *time.Time `gorm:"index" json:"-"`
	RecoveryCode        *string    `json:"-"`
	RecoveryCodeSentAt  *time.Time `json:"-"`
	Address             *string    `json:"address"`
	CreatedBy           *uint      `json:"created_by"`
	Note                *string    `json:"note"`
	CreditScore         int        `gorm:"default:0" json:"credit_score"`
	Locale              string     `gorm:"default:es" json:"locale"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`

	// Associations
	Creator       *User          `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Contracts     []Contract     `gorm:"foreignKey:ApplicantUserID" json:"contracts,omitempty"`
	Notifications []Notification `gorm:"foreignKey:UserID" json:"notifications,omitempty"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook for setting defaults
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Role == "" {
		u.Role = RoleUser
	}
	if u.Status == "" {
		u.Status = StatusActive
	}
	if u.Locale == "" {
		u.Locale = LocaleES
	}
	return nil
}

// IsAdmin returns true if user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsSeller returns true if user has seller role
func (u *User) IsSeller() bool {
	return u.Role == RoleSeller
}

// IsActive returns true if user status is active
func (u *User) IsActive() bool {
	return u.Status == StatusActive && u.DiscardedAt == nil
}

// IsConfirmed returns true if user email is confirmed
func (u *User) IsConfirmed() bool {
	return u.ConfirmedAt != nil
}

// IsDiscarded returns true if user is soft-deleted
func (u *User) IsDiscarded() bool {
	return u.DiscardedAt != nil
}

// Role constants
const (
	RoleAdmin  = "admin"
	RoleSeller = "seller"
	RoleUser   = "user"
)

// Status constants
const (
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusSuspended = "suspended"
)

// Locale constants
const (
	LocaleES = "es"
	LocaleEN = "en"
)

// UserResponse is the JSON response format for users
type UserResponse struct {
	ID          uint       `json:"id"`
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	Phone       string     `json:"phone"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	Identity    string     `json:"identity"`
	RTN         string     `json:"rtn"`
	Address     *string    `json:"address"`
	CreditScore int        `json:"credit_score"`
	Locale      string     `json:"locale"`
	ConfirmedAt *time.Time `json:"confirmed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		FullName:    u.FullName,
		Phone:       u.Phone,
		Role:        u.Role,
		Status:      u.Status,
		Identity:    u.Identity,
		RTN:         u.RTN,
		Address:     u.Address,
		CreditScore: u.CreditScore,
		Locale:      u.Locale,
		ConfirmedAt: u.ConfirmedAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

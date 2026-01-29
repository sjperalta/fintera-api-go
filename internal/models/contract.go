package models

import (
	"time"
)

// Contract represents a sales contract for a lot
type Contract struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	LotID           uint       `gorm:"not null;index" json:"lot_id"`
	CreatorID       *uint      `gorm:"index" json:"creator_id"`
	ApplicantUserID uint       `gorm:"not null;index" json:"applicant_user_id"`
	PaymentTerm     int        `gorm:"not null" json:"payment_term"`
	FinancingType   string     `gorm:"not null" json:"financing_type"`
	Status          string     `gorm:"default:pending;index" json:"status"`
	Amount          *float64   `gorm:"type:decimal" json:"amount"`
	Balance         *float64   `gorm:"type:decimal" json:"balance"`
	DownPayment     *float64   `gorm:"type:decimal" json:"down_payment"`
	ReserveAmount   *float64   `gorm:"type:decimal" json:"reserve_amount"`
	Currency        string     `gorm:"default:HNL;not null" json:"currency"`
	ApprovedAt      *time.Time `gorm:"index" json:"approved_at"`
	Active          bool       `gorm:"default:false;index" json:"active"`
	Note            *string    `gorm:"type:text" json:"note"`
	RejectionReason *string    `gorm:"type:text" json:"rejection_reason"`
	DocumentPaths   *string    `gorm:"type:text" json:"document_paths"` // JSON string of document paths
	ClosedAt        *time.Time `json:"closed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Associations
	Lot           Lot                   `gorm:"foreignKey:LotID" json:"lot,omitempty"`
	Creator       *User                 `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	ApplicantUser User                  `gorm:"foreignKey:ApplicantUserID" json:"applicant_user,omitempty"`
	Payments      []Payment             `gorm:"foreignKey:ContractID" json:"payments,omitempty"`
	LedgerEntries []ContractLedgerEntry `gorm:"foreignKey:ContractID" json:"ledger_entries,omitempty"`
}

// TableName specifies the table name for Contract
func (Contract) TableName() string {
	return "contracts"
}

// Contract status constants
const (
	ContractStatusPending   = "pending"
	ContractStatusSubmitted = "submitted"
	ContractStatusApproved  = "approved"
	ContractStatusRejected  = "rejected"
	ContractStatusCancelled = "cancelled"
	ContractStatusClosed    = "closed"
)

// Financing type constants
const (
	FinancingTypeDirect = "direct"
	FinancingTypeBank   = "bank"
	FinancingTypeCash   = "cash"
)

// MaySubmit returns true if contract can transition to submitted
func (c *Contract) MaySubmit() bool {
	return c.Status == ContractStatusPending || c.Status == ContractStatusRejected
}

// MayApprove returns true if contract can be approved
func (c *Contract) MayApprove() bool {
	return c.Status == ContractStatusPending ||
		c.Status == ContractStatusSubmitted ||
		c.Status == ContractStatusRejected
}

// MayReject returns true if contract can be rejected
func (c *Contract) MayReject() bool {
	return c.Status == ContractStatusPending || c.Status == ContractStatusSubmitted
}

// MayCancel returns true if contract can be cancelled
func (c *Contract) MayCancel() bool {
	return c.Status == ContractStatusPending ||
		c.Status == ContractStatusSubmitted ||
		c.Status == ContractStatusRejected
}

// MayClose returns true if contract can be closed
func (c *Contract) MayClose() bool {
	if c.Status != ContractStatusApproved {
		return false
	}
	if c.Balance == nil {
		return false
	}
	return *c.Balance <= 0
}

// MayReopen returns true if contract can be reopened
func (c *Contract) MayReopen() bool {
	return c.Status == ContractStatusClosed
}

// CalculateBalance calculates the current balance from ledger entries
func (c *Contract) CalculateBalance() float64 {
	var total float64
	for _, entry := range c.LedgerEntries {
		total += entry.Amount
	}
	return total
}

// ContractResponse is the JSON response format for contracts
type ContractResponse struct {
	ID                     uint                          `json:"id"`
	ContractID             uint                          `json:"contract_id"`
	ProjectID              uint                          `json:"project_id"`
	ProjectName            string                        `json:"project_name"`
	ProjectAddress         string                        `json:"project_address"`
	LotID                  uint                          `json:"lot_id"`
	LotName                string                        `json:"lot_name"`
	LotAddress             *string                       `json:"lot_address"`
	LotWidth               float64                       `json:"lot_width"`
	LotLength              float64                       `json:"lot_length"`
	LotArea                float64                       `json:"lot_area"`
	LotPrice               float64                       `json:"lot_price"`
	LotOverridePrice       *float64                      `json:"lot_override_price"`
	ApplicantUserID        uint                          `json:"applicant_user_id"`
	ApplicantName          string                        `json:"applicant_name"`
	ApplicantPhone         string                        `json:"applicant_phone"`
	ApplicantIdentity      string                        `json:"applicant_identity"`
	ApplicantCreditScore   int                           `json:"applicant_credit_score"`
	CreatedBy              string                        `json:"created_by"`
	Amount                 *float64                      `json:"amount"`
	PaymentTerm            int                           `json:"payment_term"`
	FinancingType          string                        `json:"financing_type"`
	ReserveAmount          *float64                      `json:"reserve_amount"`
	DownPayment            *float64                      `json:"down_payment"`
	Status                 string                        `json:"status"`
	Balance                float64                       `json:"balance"`
	RejectionReason        *string                       `json:"rejection_reason"`
	CancellationNotes      *string                       `json:"cancellation_notes"`
	TotalInterestCollected float64                       `json:"total_interest_collected"`
	TotalPaid              float64                       `json:"total_paid"`
	ApprovedAt             *time.Time                    `json:"approved_at"`
	CreatedAt              time.Time                     `json:"created_at"`
	UpdatedAt              time.Time                     `json:"updated_at"`
	Note                   *string                       `json:"note"`
	PaymentSchedule        []PaymentResponse             `json:"payment_schedule"`
	LedgerEntries          []ContractLedgerEntryResponse `json:"ledger_entries"`
}

// ToResponse converts Contract to ContractResponse
func (c *Contract) ToResponse() ContractResponse {
	resp := ContractResponse{
		ID:                c.ID,
		ContractID:        c.ID,
		LotID:             c.LotID,
		ApplicantUserID:   c.ApplicantUserID,
		PaymentTerm:       c.PaymentTerm,
		FinancingType:     c.FinancingType,
		Amount:            c.Amount,
		ReserveAmount:     c.ReserveAmount,
		DownPayment:       c.DownPayment,
		Status:            c.Status,
		RejectionReason:   c.RejectionReason,
		CancellationNotes: c.Note,
		ApprovedAt:        c.ApprovedAt,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
		Note:              c.Note,
	}

	// Add lot info
	resp.LotName = c.Lot.Name
	resp.LotAddress = c.Lot.Address
	resp.LotWidth = c.Lot.Width
	resp.LotLength = c.Lot.Length
	resp.LotArea = c.Lot.Area()
	resp.LotPrice = c.Lot.Price
	resp.LotOverridePrice = c.Lot.OverridePrice

	// Add project info
	resp.ProjectID = c.Lot.ProjectID
	resp.ProjectName = c.Lot.Project.Name
	resp.ProjectAddress = c.Lot.Project.Address

	// Add applicant info
	resp.ApplicantName = c.ApplicantUser.FullName
	resp.ApplicantPhone = c.ApplicantUser.Phone
	resp.ApplicantIdentity = maskIdentity(c.ApplicantUser.Identity)
	resp.ApplicantCreditScore = c.ApplicantUser.CreditScore

	// Add creator info
	if c.Creator != nil {
		resp.CreatedBy = c.Creator.FullName
	}

	// Calculate balance and totals
	// Use cached balance if available, otherwise calculate from ledger
	if c.Balance != nil {
		resp.Balance = *c.Balance
	} else {
		resp.Balance = c.CalculateBalance()
	}

	// Calculate totals from payments
	var totalInterest, totalPaid float64
	for _, p := range c.Payments {
		if p.InterestAmount != nil {
			totalInterest += *p.InterestAmount
		}
		if p.Status == PaymentStatusPaid && p.PaidAmount != nil {
			totalPaid += *p.PaidAmount
		}
	}
	resp.TotalInterestCollected = totalInterest
	resp.TotalPaid = totalPaid

	// Add payment schedule
	for _, payment := range c.Payments {
		resp.PaymentSchedule = append(resp.PaymentSchedule, payment.ToResponse())
	}

	// Add ledger entries
	for _, entry := range c.LedgerEntries {
		resp.LedgerEntries = append(resp.LedgerEntries, entry.ToResponse())
	}

	return resp
}

// maskIdentity masks an identity string for privacy
func maskIdentity(identity string) string {
	if len(identity) <= 4 {
		masked := ""
		for range identity {
			masked += "*"
		}
		return masked
	}
	masked := identity[:4]
	for i := 4; i < len(identity)-3; i++ {
		masked += "*"
	}
	if len(identity) > 4 {
		masked += identity[len(identity)-3:]
	}
	return masked
}

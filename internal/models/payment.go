package models

import (
	"fmt"
	"strings"
	"time"
)

// Payment represents a payment for a contract
type Payment struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	ContractID       uint       `gorm:"not null;index" json:"contract_id"`
	Amount           float64    `gorm:"type:decimal(10,2);not null" json:"amount"`
	PaidAmount       *float64   `gorm:"type:decimal(15,2);default:0" json:"paid_amount"`
	DueDate          time.Time  `gorm:"type:date;not null;index" json:"due_date"`
	PaymentDate      *time.Time `gorm:"type:date" json:"payment_date"`
	Status           string     `gorm:"default:pending;not null;index" json:"status"`
	PaymentType      string     `gorm:"default:installment" json:"payment_type"`
	Description      *string    `json:"description"`
	InterestAmount   *float64   `gorm:"type:decimal(10,2)" json:"interest_amount"`
	ApprovedAt       *time.Time `gorm:"index" json:"approved_at"`
	ApprovedByUserID *uint      `gorm:"index" json:"approved_by_user_id"`
	RejectionReason       *string    `gorm:"type:text" json:"rejection_reason,omitempty"`
	DocumentPath          *string    `json:"-"` // Receipt file path
	OverdueReminderSentAt   *time.Time `gorm:"column:overdue_reminder_sent_at" json:"-"`   // When overdue reminder email was last sent
	UpcomingReminderSentAt  *time.Time `gorm:"column:upcoming_reminder_sent_at" json:"-"`  // When "due tomorrow" reminder was sent
	CreatedAt               time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`

	// Associations
	Contract       Contract `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
	ApprovedByUser User     `gorm:"foreignKey:ApprovedByUserID" json:"approved_by_user,omitempty"`
}

// TableName specifies the table name for Payment
func (Payment) TableName() string {
	return "payments"
}

// Payment status constants
const (
	PaymentStatusPending      = "pending"
	PaymentStatusSubmitted    = "submitted"
	PaymentStatusPaid         = "paid"
	PaymentStatusRejected     = "rejected"
	PaymentStatusReadjustment = "readjustment"
)

// Payment type constants
const (
	PaymentTypeReservation      = "reservation"
	PaymentTypeDownPayment      = "down_payment"
	PaymentTypeInstallment      = "installment"
	PaymentTypeFull             = "full"
	PaymentTypeAdvance          = "advance"
	PaymentTypeCapitalRepayment = "capital_repayment"
)

// MaySubmit returns true if payment can transition to submitted
func (p *Payment) MaySubmit() bool {
	return p.Status == PaymentStatusPending && p.DocumentPath != nil
}

// MayApprove returns true if payment can be approved
func (p *Payment) MayApprove() bool {
	return p.Status == PaymentStatusPending || p.Status == PaymentStatusSubmitted
}

// MayReject returns true if payment can be rejected
func (p *Payment) MayReject() bool {
	return p.Status == PaymentStatusSubmitted
}

// MayUndo returns true if payment approval can be undone
func (p *Payment) MayUndo() bool {
	return p.Status == PaymentStatusPaid
}

// IsOverdue returns true if payment is past due date
func (p *Payment) IsOverdue() bool {
	return p.Status == PaymentStatusPending && time.Now().After(p.DueDate)
}

// OverdueDays returns the number of days overdue
func (p *Payment) OverdueDays() int {
	if !p.IsOverdue() {
		return 0
	}
	return int(time.Since(p.DueDate).Hours() / 24)
}

// PaymentResponse is the JSON response format for payments
type PaymentResponse struct {
	ID             uint       `json:"id"`
	ContractID     uint       `json:"contract_id"`
	DueDate        time.Time  `json:"due_date"`
	Amount         float64    `json:"amount"`
	Status         string     `json:"status"`
	PaymentType    string     `json:"payment_type"`
	PaidAmount     float64    `json:"paid_amount"`
	InterestAmount float64    `json:"interest_amount"`
	OverdueDays    int        `json:"overdue_days"`
	Description    *string    `json:"description"`
	PaymentDate    *time.Time `json:"payment_date"`
	ApprovedAt     *time.Time `json:"approved_at"`
	Approver       string     `json:"approver,omitempty"`
	HasReceipt      bool    `json:"has_receipt"`
	IsPDF           bool    `json:"is_pdf"`
	IsOverpayment   bool    `json:"is_overpayment"` // true when paid_amount > amount
	RejectionReason *string `json:"rejection_reason,omitempty"`

	// Contract details
	ContractStatus    string  `json:"contract_status,omitempty"`
	ApplicantName     string  `json:"applicant_name,omitempty"`
	ApplicantPhone    string  `json:"applicant_phone,omitempty"`
	ApplicantIdentity string  `json:"applicant_identity,omitempty"`
	ProjectName       string  `json:"project_name,omitempty"`
	ProjectAddress    string  `json:"project_address,omitempty"`
	LotName           string  `json:"lot_name,omitempty"`
	LotAddress        *string `json:"lot_address,omitempty"`
	LotDimensions     string  `json:"lot_dimensions,omitempty"`
}

// ToResponse converts Payment to PaymentResponse
func (p *Payment) ToResponse() PaymentResponse {
	resp := PaymentResponse{
		ID:          p.ID,
		ContractID:  p.ContractID,
		DueDate:     p.DueDate,
		Amount:      p.Amount,
		Status:      p.Status,
		PaymentType: p.PaymentType,
		OverdueDays: p.OverdueDays(),
		Description: p.Description,
		PaymentDate: p.PaymentDate,
		ApprovedAt:  p.ApprovedAt,
		HasReceipt:  p.DocumentPath != nil && *p.DocumentPath != "",
		IsPDF:       p.DocumentPath != nil && strings.HasSuffix(strings.ToLower(*p.DocumentPath), ".pdf"),
	}

	if p.ApprovedByUser.ID != 0 {
		resp.Approver = p.ApprovedByUser.FullName
	}

	if p.PaidAmount != nil {
		resp.PaidAmount = *p.PaidAmount
		resp.IsOverpayment = *p.PaidAmount > p.Amount
	}
	if p.InterestAmount != nil {
		resp.InterestAmount = *p.InterestAmount
	}
	if p.RejectionReason != nil {
		resp.RejectionReason = p.RejectionReason
	}

	// Add contract details if available
	if p.Contract.ID != 0 {
		resp.ContractStatus = p.Contract.Status

		// Add applicant user details
		if p.Contract.ApplicantUser.ID != 0 {
			resp.ApplicantName = p.Contract.ApplicantUser.FullName
			resp.ApplicantPhone = p.Contract.ApplicantUser.Phone
			resp.ApplicantIdentity = maskIdentity(p.Contract.ApplicantUser.Identity)
		}

		// Add lot details
		if p.Contract.Lot.ID != 0 {
			resp.LotName = p.Contract.Lot.Name
			resp.LotAddress = p.Contract.Lot.Address

			// Format dimensions as "width x length"
			if p.Contract.Lot.Width > 0 && p.Contract.Lot.Length > 0 {
				resp.LotDimensions = fmt.Sprintf("%.2f x %.2f", p.Contract.Lot.Width, p.Contract.Lot.Length)
			}

			// Add project details
			if p.Contract.Lot.Project.ID != 0 {
				resp.ProjectName = p.Contract.Lot.Project.Name
				resp.ProjectAddress = p.Contract.Lot.Project.Address
			}
		}
	}

	return resp
}

package models

import (
	"time"
)

// ContractLedgerEntry represents a financial transaction for a contract
type ContractLedgerEntry struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ContractID  uint      `json:"contract_id" gorm:"not null;index"`
	PaymentID   *uint     `json:"payment_id,omitempty" gorm:"index"`
	Amount      float64   `json:"amount" gorm:"not null"` // Negative for credits (payments), positive for debits (charges)
	Description string    `json:"description" gorm:"not null"`
	EntryType   string    `json:"entry_type" gorm:"not null;index"` // initial, payment, interest, prepayment, adjustment
	EntryDate   time.Time `json:"entry_date" gorm:"not null;default:current_timestamp"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Contract *Contract `json:"contract,omitempty" gorm:"foreignKey:ContractID"`
	Payment  *Payment  `json:"payment,omitempty" gorm:"foreignKey:PaymentID"`
}

// Entry type constants
const (
	EntryTypeInitial    = "initial"    // Contract amount (debit)
	EntryTypePayment    = "payment"    // Payment received (credit)
	EntryTypeInterest   = "interest"   // Overdue interest (debit)
	EntryTypePrepayment = "prepayment" // Capital repayment (credit)
	EntryTypeAdjustment = "adjustment" // Manual adjustment or reversal
)

// TableName specifies the table name for GORM
func (ContractLedgerEntry) TableName() string {
	return "contract_ledger_entries"
}

// ContractLedgerEntryResponse is the JSON response format for ledger entries
type ContractLedgerEntryResponse struct {
	ID          uint      `json:"id"`
	ContractID  uint      `json:"contract_id"`
	PaymentID   *uint     `json:"payment_id,omitempty"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	EntryType   string    `json:"entry_type"`
	EntryDate   time.Time `json:"entry_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToResponse converts ContractLedgerEntry to ContractLedgerEntryResponse
func (e *ContractLedgerEntry) ToResponse() ContractLedgerEntryResponse {
	return ContractLedgerEntryResponse{
		ID:          e.ID,
		ContractID:  e.ContractID,
		PaymentID:   e.PaymentID,
		Amount:      e.Amount,
		Description: e.Description,
		EntryType:   e.EntryType,
		EntryDate:   e.EntryDate,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

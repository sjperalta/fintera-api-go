package repository

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"

	"gorm.io/gorm"
)

// LedgerRepository handles database operations for contract ledger entries
type LedgerRepository struct {
	db *gorm.DB
}

// NewLedgerRepository creates a new ledger repository
func NewLedgerRepository(db *gorm.DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

// Create creates a new ledger entry
func (r *LedgerRepository) Create(ctx context.Context, entry *models.ContractLedgerEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// FindByContractID retrieves all ledger entries for a contract
func (r *LedgerRepository) FindByContractID(ctx context.Context, contractID uint) ([]models.ContractLedgerEntry, error) {
	var entries []models.ContractLedgerEntry
	err := r.db.WithContext(ctx).
		Where("contract_id = ?", contractID).
		Order("entry_date ASC, created_at ASC").
		Find(&entries).Error
	return entries, err
}

// FindByPaymentID retrieves all ledger entries for a payment
func (r *LedgerRepository) FindByPaymentID(ctx context.Context, paymentID uint) ([]models.ContractLedgerEntry, error) {
	var entries []models.ContractLedgerEntry
	err := r.db.WithContext(ctx).
		Where("payment_id = ?", paymentID).
		Order("entry_date ASC, created_at ASC").
		Find(&entries).Error
	return entries, err
}

// CalculateBalance calculates the current balance for a contract
// Balance = sum of all ledger entries (positive for debits, negative for credits)
func (r *LedgerRepository) CalculateBalance(ctx context.Context, contractID uint) (float64, error) {
	var result struct {
		Balance float64
	}

	err := r.db.WithContext(ctx).
		Model(&models.ContractLedgerEntry{}).
		Select("COALESCE(SUM(amount), 0) as balance").
		Where("contract_id = ?", contractID).
		Scan(&result).Error

	return result.Balance, err
}

// FindOrCreateByPaymentAndType finds or creates a ledger entry for a payment and entry type
// Used for updating interest entries without creating duplicates
func (r *LedgerRepository) FindOrCreateByPaymentAndType(ctx context.Context, entry *models.ContractLedgerEntry) error {
	// If payment_id and entry_type are set, try to find existing
	if entry.PaymentID != nil && entry.EntryType == models.EntryTypeInterest {
		var existing models.ContractLedgerEntry
		err := r.db.WithContext(ctx).
			Where("payment_id = ? AND entry_type = ?", entry.PaymentID, entry.EntryType).
			First(&existing).Error

		if err == nil {
			// Update existing entry
			existing.Amount = entry.Amount
			existing.Description = entry.Description
			existing.EntryDate = entry.EntryDate
			return r.db.WithContext(ctx).Save(&existing).Error
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
	}

	// Create new entry
	return r.Create(ctx, entry)
}

// DeleteByContractID deletes all ledger entries for a contract (used when canceling)
func (r *LedgerRepository) DeleteByContractID(ctx context.Context, contractID uint) error {
	return r.db.WithContext(ctx).
		Where("contract_id = ?", contractID).
		Delete(&models.ContractLedgerEntry{}).Error
}

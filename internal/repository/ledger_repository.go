package repository

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"

	"gorm.io/gorm"
)

// LedgerRepository defines the interface for contract ledger data access
type LedgerRepository interface {
	Create(ctx context.Context, entry *models.ContractLedgerEntry) error
	FindByContractID(ctx context.Context, contractID uint) ([]models.ContractLedgerEntry, error)
	FindByPaymentID(ctx context.Context, paymentID uint) ([]models.ContractLedgerEntry, error)
	CalculateBalance(ctx context.Context, contractID uint) (float64, error)
	FindOrCreateByPaymentAndType(ctx context.Context, entry *models.ContractLedgerEntry) error
	BatchUpsertInterest(ctx context.Context, entries []models.ContractLedgerEntry) error
	DeleteByContractID(ctx context.Context, contractID uint) error
}

// ledgerRepository handles database operations for contract ledger entries
type ledgerRepository struct {
	db *gorm.DB
}

// NewLedgerRepository creates a new ledger repository
func NewLedgerRepository(db *gorm.DB) LedgerRepository {
	return &ledgerRepository{db: db}
}

// Create creates a new ledger entry
// Create creates a new ledger entry
func (r *ledgerRepository) Create(ctx context.Context, entry *models.ContractLedgerEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// FindByContractID retrieves all ledger entries for a contract
// FindByContractID retrieves all ledger entries for a contract
func (r *ledgerRepository) FindByContractID(ctx context.Context, contractID uint) ([]models.ContractLedgerEntry, error) {
	var entries []models.ContractLedgerEntry
	err := r.db.WithContext(ctx).
		Where("contract_id = ?", contractID).
		Order("entry_date ASC, created_at ASC").
		Find(&entries).Error
	return entries, err
}

// FindByPaymentID retrieves all ledger entries for a payment
// FindByPaymentID retrieves all ledger entries for a payment
func (r *ledgerRepository) FindByPaymentID(ctx context.Context, paymentID uint) ([]models.ContractLedgerEntry, error) {
	var entries []models.ContractLedgerEntry
	err := r.db.WithContext(ctx).
		Where("payment_id = ?", paymentID).
		Order("entry_date ASC, created_at ASC").
		Find(&entries).Error
	return entries, err
}

// CalculateBalance calculates the current balance for a contract
// Balance = sum of all ledger entries (positive for debits, negative for credits)
// CalculateBalance calculates the current balance for a contract
// Balance = sum of all ledger entries (positive for debits, negative for credits)
func (r *ledgerRepository) CalculateBalance(ctx context.Context, contractID uint) (float64, error) {
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
// FindOrCreateByPaymentAndType finds or creates a ledger entry for a payment and entry type
// Used for updating interest entries without creating duplicates
func (r *ledgerRepository) FindOrCreateByPaymentAndType(ctx context.Context, entry *models.ContractLedgerEntry) error {
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

// BatchUpsertInterest handles batch creation or update of interest ledger entries
func (r *ledgerRepository) BatchUpsertInterest(ctx context.Context, entries []models.ContractLedgerEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// 1. Collect all PaymentIDs
	paymentIDs := make([]uint, 0, len(entries))
	for _, e := range entries {
		if e.PaymentID != nil {
			paymentIDs = append(paymentIDs, *e.PaymentID)
		}
	}

	// 2. Find existing interest entries for these payments
	var existing []models.ContractLedgerEntry
	if err := r.db.WithContext(ctx).
		Where("payment_id IN ? AND entry_type = ?", paymentIDs, models.EntryTypeInterest).
		Find(&existing).Error; err != nil {
		return err
	}

	// 3. Map existing entries by PaymentID for quick lookup
	existingMap := make(map[uint]models.ContractLedgerEntry)
	for _, e := range existing {
		if e.PaymentID != nil {
			existingMap[*e.PaymentID] = e
		}
	}

	// 4. Separate into Create and Update lists
	var toCreate []models.ContractLedgerEntry
	var toUpdate []models.ContractLedgerEntry

	for _, entry := range entries {
		if e, ok := existingMap[*entry.PaymentID]; ok {
			// Update fields
			e.Amount = entry.Amount
			e.Description = entry.Description
			e.EntryDate = entry.EntryDate
			toUpdate = append(toUpdate, e)
		} else {
			// New entry
			toCreate = append(toCreate, entry)
		}
	}

	// 5. Execute in transaction
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Bulk Insert
		if len(toCreate) > 0 {
			if err := tx.Create(&toCreate).Error; err != nil {
				return err
			}
		}
		// Bulk Update (simulated via loop in transaction for safety/simplicity)
		for _, u := range toUpdate {
			if err := tx.Model(&models.ContractLedgerEntry{}).Where("id = ?", u.ID).
				Updates(map[string]interface{}{
					"amount":      u.Amount,
					"description": u.Description,
					"entry_date":  u.EntryDate,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteByContractID deletes all ledger entries for a contract (used when canceling)
// DeleteByContractID deletes all ledger entries for a contract (used when canceling)
func (r *ledgerRepository) DeleteByContractID(ctx context.Context, contractID uint) error {
	return r.db.WithContext(ctx).
		Where("contract_id = ?", contractID).
		Delete(&models.ContractLedgerEntry{}).Error
}

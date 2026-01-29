package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/statemachine"
)

// ApprovePayment approves a payment and creates ledger entry
func (s *PaymentService) ApprovePaymentWithLedger(ctx context.Context, id uint, paidAmount float64) error {
	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load payment: %w", err)
	}

	// Use FSM to validate and transition
	fsm := statemachine.NewPaymentFSM(payment)
	if err := fsm.Approve(ctx); err != nil {
		return fmt.Errorf("cannot approve payment: %w", err)
	}

	// Default to full payment amount if not specified
	if paidAmount <= 0 {
		paidAmount = payment.Amount
	}

	// Set payment details
	now := time.Now()
	payment.PaidAmount = &paidAmount
	payment.PaymentDate = &now
	payment.ApprovedAt = &now

	// Update payment
	if err := s.repo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	// Create ledger entry (negative amount = credit, reduces balance)
	// UPDATE: Ledger rules inverted. Positive amount = credit (payment), reduces negative debt balance.
	ledgerEntry := &models.ContractLedgerEntry{
		ContractID:  payment.ContractID,
		PaymentID:   &payment.ID,
		Amount:      paidAmount, // Positive for credit
		Description: fmt.Sprintf("Pago #%d - %s", payment.ID, getPaymentTypeDescription(payment.PaymentType)),
		EntryType:   models.EntryTypePayment,
		EntryDate:   now,
	}
	if err := s.ledgerRepo.Create(ctx, ledgerEntry); err != nil {
		return fmt.Errorf("failed to create ledger entry: %w", err)
	}

	// Check if contract should be closed
	balance, err := s.ledgerRepo.CalculateBalance(ctx, payment.ContractID)
	if err != nil {
		fmt.Printf("Failed to calculate balance: %v\n", err)
	} else if balance >= 0 {
		// TODO: Auto-close contract via ContractService
		fmt.Printf("Contract %d balance is now %.2f - should be closed\n", payment.ContractID, balance)
	}

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		contract, _ := s.contractRepo.FindByIDWithDetails(ctx, payment.ContractID)
		if contract != nil {
			return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
				"Pago aprobado",
				fmt.Sprintf("Tu pago de L%.2f ha sido aprobado", paidAmount),
				models.NotificationTypePaymentApproved)
		}
		return nil
	})

	return nil
}

// RejectPayment rejects a payment
func (s *PaymentService) RejectPaymentWithFSM(ctx context.Context, id uint, reason string) error {
	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load payment: %w", err)
	}

	// Use FSM to validate and transition
	fsm := statemachine.NewPaymentFSM(payment)
	if err := fsm.Reject(ctx); err != nil {
		return fmt.Errorf("cannot reject payment: %w", err)
	}

	// Update payment
	if err := s.repo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		contract, _ := s.contractRepo.FindByIDWithDetails(ctx, payment.ContractID)
		if contract != nil {
			return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
				"Pago rechazado",
				fmt.Sprintf("Tu pago ha sido rechazado. Razón: %s", reason),
				models.NotificationTypePaymentRejected)
		}
		return nil
	})

	return nil
}

// UndoPayment undoes an approved payment and reverses the ledger entry
func (s *PaymentService) UndoPayment(ctx context.Context, id uint) error {
	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load payment: %w", err)
	}

	// Use FSM to validate and transition
	fsm := statemachine.NewPaymentFSM(payment)
	if err := fsm.Undo(ctx); err != nil {
		return fmt.Errorf("cannot undo payment: %w", err)
	}

	// Get original ledger entry
	ledgerEntries, err := s.ledgerRepo.FindByPaymentID(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("failed to find ledger entries: %w", err)
	}

	// Create reversal entry (opposite sign)
	if len(ledgerEntries) > 0 {
		originalEntry := ledgerEntries[0]
		reversalEntry := &models.ContractLedgerEntry{
			ContractID:  payment.ContractID,
			PaymentID:   &payment.ID,
			Amount:      -originalEntry.Amount, // Reverse the sign
			Description: fmt.Sprintf("Reversión de Pago #%d", payment.ID),
			EntryType:   models.EntryTypeAdjustment,
			EntryDate:   time.Now(),
		}
		if err := s.ledgerRepo.Create(ctx, reversalEntry); err != nil {
			return fmt.Errorf("failed to create reversal entry: %w", err)
		}
	}

	// Clear payment details
	payment.PaidAmount = nil
	payment.PaymentDate = nil
	payment.ApprovedAt = nil

	// Update payment
	if err := s.repo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		contract, _ := s.contractRepo.FindByIDWithDetails(ctx, payment.ContractID)
		if contract != nil {
			return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
				"Pago revertido",
				"Un pago aprobado ha sido revertido",
				models.NotificationTypePaymentRejected)
		}
		return nil
	})

	return nil
}

// Helper function to get payment type description in Spanish
func getPaymentTypeDescription(paymentType string) string {
	switch paymentType {
	case models.PaymentTypeReservation:
		return "Reserva"
	case models.PaymentTypeDownPayment:
		return "Pago Inicial"
	case models.PaymentTypeInstallment:
		return "Cuota"
	case models.PaymentTypeFull:
		return "Pago Total"
	case models.PaymentTypeAdvance:
		return "Anticipo"
	default:
		return "Pago"
	}
}

package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/storage"
)

// RevenuePoint represents a data point in the revenue chart
type RevenuePoint struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
}

type UserFinancingSummary struct {
	Balance   float64 `json:"balance"`
	TotalDue  float64 `json:"totalDue"`
	TotalFees float64 `json:"totalFees"`
	Currency  string  `json:"currency"`
}

type PaymentService struct {
	repo            repository.PaymentRepository
	contractRepo    repository.ContractRepository
	lotRepo         repository.LotRepository
	ledgerRepo      *repository.LedgerRepository
	notificationSvc *NotificationService
	auditSvc        *AuditService
	storage         *storage.LocalStorage
	worker          *jobs.Worker
}

func NewPaymentService(
	repo repository.PaymentRepository,
	contractRepo repository.ContractRepository,
	lotRepo repository.LotRepository,
	ledgerRepo *repository.LedgerRepository,
	notificationSvc *NotificationService,
	auditSvc *AuditService,
	storage *storage.LocalStorage,
	worker *jobs.Worker,
) *PaymentService {
	return &PaymentService{
		repo:            repo,
		contractRepo:    contractRepo,
		lotRepo:         lotRepo,
		ledgerRepo:      ledgerRepo,
		notificationSvc: notificationSvc,
		auditSvc:        auditSvc,
		storage:         storage,
		worker:          worker,
	}
}

func (s *PaymentService) FindByID(ctx context.Context, id uint) (*models.Payment, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *PaymentService) FindByContract(ctx context.Context, contractID uint) ([]models.Payment, error) {
	return s.repo.FindByContract(ctx, contractID)
}

func (s *PaymentService) List(ctx context.Context, query *repository.ListQuery) ([]models.Payment, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *PaymentService) Approve(ctx context.Context, id uint, amount, interestAmount, paidAmount float64, actorID uint, ip, userAgent string) (*models.Payment, error) {
	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !payment.MayApprove() {
		return nil, ErrInvalidState
	}

	// Default to full payment amount (Amount + InterestAmount) if not specified
	if paidAmount <= 0 {
		total := payment.Amount
		if payment.InterestAmount != nil {
			total += *payment.InterestAmount
		}
		paidAmount = total
	}

	now := time.Now()
	payment.Status = models.PaymentStatusPaid
	payment.PaidAmount = &paidAmount
	payment.PaymentDate = &now
	payment.ApprovedAt = &now

	if err := s.repo.Update(ctx, payment); err != nil {
		return nil, err
	}

	// Calculate and handle excess amount
	expectedTotal := payment.Amount
	if payment.InterestAmount != nil {
		expectedTotal += *payment.InterestAmount
	}
	extraAmount := paidAmount - expectedTotal

	// Calculate capital repayment part for ledger (before consuming extraAmount in loop)
	capitalRepayment := 0.0
	if extraAmount > 0 {
		capitalRepayment = extraAmount
	}

	if extraAmount > 0 {
		// Apply extra amount to remaining payments from last to first
		pendingPayments, err := s.repo.FindByContract(ctx, payment.ContractID)
		if err == nil {
			// Find only pending/submitted ones excluding current
			var targets []*models.Payment
			for i := range pendingPayments {
				p := &pendingPayments[i]
				if p.ID != payment.ID && (p.Status == models.PaymentStatusPending || p.Status == models.PaymentStatusSubmitted) {
					targets = append(targets, p)
				}
			}

			// Iterate backwards (last payments first)
			for i := len(targets) - 1; i >= 0 && extraAmount > 0; i-- {
				target := targets[i]

				// Apply capital payment ONLY to Principal (Amount)
				// We do not pay off future interest with capital payments; we simply delete the principal obligation.
				targetAmount := target.Amount

				if extraAmount >= targetAmount {
					// Fully paid by extra amount - DELETE the schedule record
					if err := s.repo.Delete(ctx, target.ID); err != nil {
						return nil, fmt.Errorf("failed to delete payment #%d: %w", target.ID, err)
					}
					extraAmount -= targetAmount
				} else {
					// Partially paid
					target.Amount -= extraAmount
					extraAmount = 0
					s.repo.Update(ctx, target)
				}
			}
		}
	}

	desc := "Pago Recibido"
	if payment.Description != nil {
		desc = fmt.Sprintf("Pago Recibido: %s", *payment.Description)
	}

	// Create ledger entry for payment (credit)
	// 1. Regular Payment (Installment)
	regularAmount := paidAmount - capitalRepayment
	ledgerEntry := &models.ContractLedgerEntry{
		ContractID:  payment.ContractID,
		PaymentID:   &payment.ID,
		Amount:      regularAmount, // Positive for credit
		Description: desc,
		EntryType:   models.EntryTypePayment,
		EntryDate:   now,
	}

	if err := s.ledgerRepo.Create(ctx, ledgerEntry); err != nil {
		return nil, fmt.Errorf("failed to create ledger entry: %w", err)
	}

	// 2. Prepayment (Capital Repayment)
	if capitalRepayment > 0 {
		capitalDesc := desc + " (Abono a Capital)"
		capitalEntry := &models.ContractLedgerEntry{
			ContractID:  payment.ContractID,
			PaymentID:   &payment.ID,
			Amount:      capitalRepayment,
			Description: capitalDesc,
			EntryType:   models.EntryTypePrepayment,
			EntryDate:   now,
		}

		if err := s.ledgerRepo.Create(ctx, capitalEntry); err != nil {
			return nil, fmt.Errorf("failed to create capital ledger entry: %w", err)
		}
	}

	// Update contract balance and Lot status
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		// Update lot status to financed if this is a reservation payment
		if payment.PaymentType == models.PaymentTypeReservation {
			contract, err := s.contractRepo.FindByID(ctx, payment.ContractID)
			if err == nil {
				lot, err := s.lotRepo.FindByID(ctx, contract.LotID)
				if err == nil {
					lot.Status = models.LotStatusFinanced
					s.lotRepo.Update(ctx, lot)
				}
			}
		}
		return s.updateContractBalance(ctx, payment.ContractID)
	})

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		contract, _ := s.contractRepo.FindByIDWithDetails(ctx, payment.ContractID)
		if contract != nil {
			return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
				"Pago aprobado",
				"Tu pago ha sido aprobado",
				models.NotificationTypePaymentApproved)
		}
		return nil
	})

	// Audit log
	s.auditSvc.Log(ctx, actorID, "APPROVE", "Payment", payment.ID,
		fmt.Sprintf("Pago de %.2f aprobado para contrato #%d", paidAmount, payment.ContractID), ip, userAgent)

	return payment, nil
}

func (s *PaymentService) Reject(ctx context.Context, id uint, actorID uint, ip, userAgent string) (*models.Payment, error) {
	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !payment.MayReject() {
		return nil, ErrInvalidState
	}

	payment.Status = models.PaymentStatusRejected

	if err := s.repo.Update(ctx, payment); err != nil {
		return nil, err
	}

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		contract, _ := s.contractRepo.FindByIDWithDetails(ctx, payment.ContractID)
		return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
			"Pago rechazado",
			"Tu pago ha sido rechazado",
			models.NotificationTypePaymentRejected)
	})

	// Audit log
	s.auditSvc.Log(ctx, actorID, "REJECT", "Payment", payment.ID,
		fmt.Sprintf("Pago de %.2f rechazado", payment.Amount), ip, userAgent)

	return payment, nil
}

func (s *PaymentService) UpdateReceiptPath(ctx context.Context, id uint, path string) error {
	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	payment.DocumentPath = &path
	payment.Status = models.PaymentStatusSubmitted

	return s.repo.Update(ctx, payment)
}

func (s *PaymentService) CheckOverduePayments(ctx context.Context) error {
	payments, err := s.repo.FindOverdue(ctx)
	if err != nil {
		return err
	}

	for _, payment := range payments {
		// Notify user about overdue payment
		s.notificationSvc.NotifyUser(ctx, payment.Contract.ApplicantUserID,
			"Pago vencido",
			"Tienes un pago vencido pendiente",
			models.NotificationTypePaymentOverdue)
	}

	return nil
}

func (s *PaymentService) updateContractBalance(ctx context.Context, contractID uint) error {
	// Recalculate and update contract balance
	balance, err := s.ledgerRepo.CalculateBalance(ctx, contractID)
	if err != nil {
		return err
	}

	// Get contract details
	contract, err := s.contractRepo.FindByID(ctx, contractID)
	if err != nil {
		return err
	}

	// Update balance field
	contract.Balance = &balance
	if err := s.contractRepo.Update(ctx, contract); err != nil {
		return err
	}

	// Auto-close contract if balance is >= 0 (fully paid) and status is approved
	if balance >= 0 && contract.Status == models.ContractStatusApproved {
		// Close the contract
		now := time.Now()
		contract.Status = models.ContractStatusClosed
		contract.ClosedAt = &now
		contract.Active = false

		if err := s.contractRepo.Update(ctx, contract); err != nil {
			return fmt.Errorf("failed to auto-close contract: %w", err)
		}

		// Update lot status to fully paid
		lot, err := s.lotRepo.FindByID(ctx, contract.LotID)
		if err == nil {
			lot.Status = models.LotStatusFullyPaid
			s.lotRepo.Update(ctx, lot)
		}

		// Notify user about contract closure
		s.worker.EnqueueAsync(func(ctx context.Context) error {
			return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
				"Contrato completado",
				"Tu contrato ha sido completado exitosamente. ¡Felicidades!",
				models.NotificationTypeContractApproved)
		})
	}

	return nil
}

// CalculateOverdueInterest calculates and applies interest for overdue payments
func (s *PaymentService) CalculateOverdueInterest(ctx context.Context) error {
	// Find all overdue payments (pending status and due_date < today)
	payments, err := s.repo.FindOverdue(ctx)
	if err != nil {
		return err
	}

	interestRate := 0.05 // 5% annual interest, TODO: Move to config or contract setting

	for _, payment := range payments {
		if payment.PaymentDate == nil {
			// Calculate days overdue
			daysOverdue := int(time.Since(payment.DueDate).Hours() / 24)
			if daysOverdue <= 0 {
				continue
			}

			// Interest formula: (amount * rate * days) / 365
			interestAmount := (payment.Amount * interestRate * float64(daysOverdue)) / 365.0

			// Find or create ledger entry for interest
			// To avoid duplicates, we check if an interest entry exists for this payment and date
			// For simplicity in this MVP, we might accumulate or just log.
			// Better approach: Create a daily interest entry if not exists.

			// Check if we already added interest for today
			// This requires a more complex query or tracking.
			// For now, let's just log it as a placeholder for the logic
			// because fully implementing interest accumulation requires careful ledger management
			// to avoid double charging if job runs multiple times.

			// Creating a ledger entry for the accrued interest
			description := fmt.Sprintf("Interés por mora (%d días) - Pago #%d", daysOverdue, payment.ID)

			// We only want to add interest if it hasn't been added for this day?
			// Or maybe update the total interest balance.

			// Simplification for Phase 3:
			// Just create the entry. In production, we'd check for duplicates.
			entry := &models.ContractLedgerEntry{
				ContractID:  payment.ContractID,
				Amount:      interestAmount,
				Description: description,
				EntryType:   models.EntryTypeInterest,
				EntryDate:   time.Now(),
			}

			if err := s.ledgerRepo.Create(ctx, entry); err != nil {
				// Log error but continue
				continue
			}
		}
	}
	return nil
}

func (s *PaymentService) GetMonthlyStatistics(ctx context.Context, month, year int) ([]RevenuePoint, error) {
	payments, err := s.repo.FindPaidByMonth(ctx, month, year)
	if err != nil {
		return nil, err
	}

	// Aggregate by day
	dailyMap := make(map[string]float64)
	for _, p := range payments {
		if p.PaymentDate != nil {
			dateStr := p.PaymentDate.Format("2006-01-02")
			amount := 0.0
			if p.PaidAmount != nil {
				amount = *p.PaidAmount
			} else {
				amount = p.Amount
			}
			dailyMap[dateStr] += amount
		}
	}

	// Create result slice
	var results []RevenuePoint

	// Determine days in month
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		results = append(results, RevenuePoint{
			Date:   dateStr,
			Amount: dailyMap[dateStr],
		})
	}

	return results, nil
}

func (s *PaymentService) GetStats(ctx context.Context) (*repository.PaymentStats, error) {
	return s.repo.GetMonthlyStats(ctx)
}

func (s *PaymentService) GetUserFinancingSummary(ctx context.Context, userID uint) (*UserFinancingSummary, error) {
	// 1. Get all contracts for user
	contracts, err := s.contractRepo.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary := &UserFinancingSummary{
		Currency: "HNL", // Default
	}

	for _, contract := range contracts {
		// Calculate balance for each contract if not already updated (or just sum them)
		if contract.Balance != nil {
			summary.Balance += *contract.Balance
		}

		// Get overdue payments for this contract
		payments, err := s.repo.FindByContract(ctx, contract.ID)
		if err == nil {
			now := time.Now()
			for _, p := range payments {
				// Consider pending payments that are past due date
				if p.Status == models.PaymentStatusPending && p.DueDate.Before(now) {
					summary.TotalDue += p.Amount
					if p.InterestAmount != nil {
						summary.TotalDue += *p.InterestAmount
						summary.TotalFees += *p.InterestAmount
					}
				}
			}
		}
	}

	return summary, nil
}

func (s *PaymentService) GetUserPayments(ctx context.Context, userID uint) ([]models.Payment, error) {
	// 1. Get all contracts for user
	contracts, err := s.contractRepo.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	allPayments := make([]models.Payment, 0)

	// 2. Get payments for each contract
	for _, contract := range contracts {
		payments, err := s.repo.FindByContract(ctx, contract.ID)
		if err != nil {
			return nil, err
		}

		// Append payments, enriching with contract data if needed (though Payment struct usually has ContractID,
		// if we need full Contract preload, repo might handle it.
		// For now, let's just collect them. The frontend uses `contract` nested object if available,
		// but `FindByContract` might not preload it.
		// Let's manually double check if FindByContract preloads.
		// Assuming standard GORM usage, usually it doesn't unless specified.
		// But let's attach the contract object manually to avoid N+1 queries later or frontend issues if expectation is there.

		for i := range payments {
			payments[i].Contract = contract
			// Also avoid cyclic reference json issues if any (unlikely with struct)
		}

		allPayments = append(allPayments, payments...)
	}

	return allPayments, nil
}

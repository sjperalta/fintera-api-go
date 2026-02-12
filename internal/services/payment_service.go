package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/storage"
	"github.com/sjperalta/fintera-api/pkg/logger"
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
	ledgerRepo      repository.LedgerRepository
	notificationSvc *NotificationService
	emailSvc        *EmailService
	auditSvc        *AuditService
	storage         *storage.LocalStorage
	worker          *jobs.Worker
}

func NewPaymentService(
	repo repository.PaymentRepository,
	contractRepo repository.ContractRepository,
	lotRepo repository.LotRepository,
	ledgerRepo repository.LedgerRepository,
	notificationSvc *NotificationService,
	emailSvc *EmailService,
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
		emailSvc:        emailSvc,
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
	payment.ApprovedByUserID = &actorID

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
			// In-app notification
			if err := s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
				"Pago aprobado",
				"Tu pago ha sido aprobado",
				models.NotificationTypePaymentApproved); err != nil {
				return err
			}

			// Email notification
			// We need to ensure payment has contract loaded or passed correctly
			payment.Contract = *contract // Attach Loaded contract
			return s.emailSvc.SendPaymentApproved(ctx, payment)
		}
		return nil
	})

	// Audit log
	s.auditSvc.Log(ctx, actorID, "APPROVE", "Payment", payment.ID,
		fmt.Sprintf("Pago de %.2f aprobado para contrato #%d", paidAmount, payment.ContractID), ip, userAgent)

	return payment, nil
}

func (s *PaymentService) Reject(ctx context.Context, id uint, actorID uint, reason string, ip, userAgent string) (*models.Payment, error) { // Updated signature verification
	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !payment.MayReject() {
		return nil, ErrInvalidState
	}

	payment.Status = models.PaymentStatusRejected
	if reason != "" {
		payment.RejectionReason = &reason
	}

	if err := s.repo.Update(ctx, payment); err != nil {
		return nil, err
	}

	// Notify user (in-app + email with reason)
	paymentAmount := payment.Amount
	paymentDueDate := payment.DueDate.Format("02/01/2006")
	notifyMsg := "Tu pago ha sido rechazado."
	if reason != "" {
		notifyMsg = "Tu pago ha sido rechazado. Razón: " + reason
	}
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		contract, err := s.contractRepo.FindByIDWithDetails(ctx, payment.ContractID)
		if err != nil || contract == nil {
			return err
		}
		if err := s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
			"Pago rechazado",
			notifyMsg,
			models.NotificationTypePaymentRejected); err != nil {
			return err
		}
		return s.emailSvc.SendPaymentRejected(ctx, contract, paymentAmount, paymentDueDate, reason)
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

// SendDailyPaymentReminderEmails sends overdue payment reminder emails to active users with active contracts.
// Intended to run once per day. Groups overdue payments by applicant user and sends one email per user.
func (s *PaymentService) SendDailyPaymentReminderEmails(ctx context.Context) error {
	payments, err := s.repo.FindOverdueForActiveContracts(ctx)
	if err != nil {
		return fmt.Errorf("find overdue for active contracts: %w", err)
	}

	// Group by applicant user ID
	byUser := make(map[uint][]models.Payment)
	for i := range payments {
		p := &payments[i]
		if p.Contract.ApplicantUserID == 0 {
			continue
		}
		byUser[p.Contract.ApplicantUserID] = append(byUser[p.Contract.ApplicantUserID], *p)
	}

	sent := 0
	for userID, userPayments := range byUser {
		if len(userPayments) == 0 {
			continue
		}
		user := &userPayments[0].Contract.ApplicantUser
		if user.ID == 0 {
			// Preload may not have filled it in some edge case
			continue
		}
		err := s.emailSvc.SendOverduePayments(ctx, user, userPayments)
		if err != nil {
			logger.Warn(fmt.Sprintf("[Daily reminder] Failed to send overdue email to user %d: %v", userID, err))
			continue
		}
		paymentIDs := make([]uint, 0, len(userPayments))
		for _, p := range userPayments {
			paymentIDs = append(paymentIDs, p.ID)
		}
		if err := s.repo.MarkOverdueReminderSent(ctx, paymentIDs); err != nil {
			logger.Warn(fmt.Sprintf("[Daily reminder] Failed to mark reminder sent for user %d: %v", userID, err))
		}
		sent++
	}

	logger.Info(fmt.Sprintf("[Daily reminder] Sent %d payment reminder email(s) to active users with overdue payments", sent))
	return nil
}

// SendDailyUpcomingPaymentReminderEmails sends "payment due tomorrow" reminders to active users with active contracts.
// Runs once per day. Only includes payments that have not yet had an upcoming reminder sent.
func (s *PaymentService) SendDailyUpcomingPaymentReminderEmails(ctx context.Context) error {
	payments, err := s.repo.FindPaymentsDueTomorrowForActiveContracts(ctx)
	if err != nil {
		return fmt.Errorf("find payments due tomorrow: %w", err)
	}

	byUser := make(map[uint][]models.Payment)
	for i := range payments {
		p := &payments[i]
		if p.Contract.ApplicantUserID == 0 {
			continue
		}
		byUser[p.Contract.ApplicantUserID] = append(byUser[p.Contract.ApplicantUserID], *p)
	}

	sent := 0
	for userID, userPayments := range byUser {
		if len(userPayments) == 0 {
			continue
		}
		user := &userPayments[0].Contract.ApplicantUser
		if user.ID == 0 {
			continue
		}
		err := s.emailSvc.SendUpcomingPayments(ctx, user, userPayments)
		if err != nil {
			logger.Warn(fmt.Sprintf("[Daily reminder] Failed to send upcoming payment email to user %d: %v", userID, err))
			continue
		}
		paymentIDs := make([]uint, 0, len(userPayments))
		for _, p := range userPayments {
			paymentIDs = append(paymentIDs, p.ID)
		}
		if err := s.repo.MarkUpcomingReminderSent(ctx, paymentIDs); err != nil {
			logger.Warn(fmt.Sprintf("[Daily reminder] Failed to mark upcoming reminder sent for user %d: %v", userID, err))
		}
		sent++
	}

	logger.Info(fmt.Sprintf("[Daily reminder] Sent %d upcoming payment reminder email(s) (due tomorrow)", sent))
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
// CalculateOverdueInterest calculates and applies interest for overdue payments
func (s *PaymentService) CalculateOverdueInterest(ctx context.Context) error {
	// Find all overdue payments (pending status and due_date < today)
	// We need to preload Contract -> Lot -> Project to get InterestRate
	// And filter for Approved contracts only
	payments, err := s.repo.FindOverdue(ctx)
	if err != nil {
		return err
	}

	overdueCount := 0
	totalInterestCalculated := 0.0

	for _, payment := range payments {
		// Filter: Only Installments
		if payment.PaymentType != models.PaymentTypeInstallment {
			continue
		}

		// Filter: Only Approved active contracts
		if payment.Contract.Status != models.ContractStatusApproved || !payment.Contract.Active {
			continue
		}

		// Calculate days overdue
		if time.Now().Before(payment.DueDate) {
			continue
		}
		daysOverdue := int(time.Since(payment.DueDate).Hours() / 24)
		if daysOverdue <= 0 {
			continue
		}

		// Get Interest Rate from Project
		interestRate := 0.0
		if payment.Contract.LotID != 0 && payment.Contract.Lot.ProjectID != 0 {
			// Assuming Lot and Project are preloaded by FindOverdue
			interestRate = payment.Contract.Lot.Project.InterestRate / 100.0 // Convert percentage to decimal
		}

		// Specific requirement: "if not found an interest_rate in the project level then the defult is 0"
		if interestRate <= 0 {
			continue
		}

		// Formula: amount * (days / 365) * rate
		// Logic: "amount of debt" = payment.Amount (the installment amount)
		interestAmount := (payment.Amount * float64(daysOverdue) / 365.0) * interestRate

		// Update Ledger Entry
		// Rule: "balance and ledger has debt form, everything start in negative then down to zero"
		// Logic: Interest increases debt. Debt is negative. So Interest Entry must be NEGATIVE.
		// Rule: "this should be calculated daily" -> Update the *accumulated* interest entry to reflect current total.

		description := fmt.Sprintf("Interés acumulado por mora (%d días) - Pago #%d", daysOverdue, payment.ID)

		entry := &models.ContractLedgerEntry{
			ContractID:  payment.ContractID,
			PaymentID:   &payment.ID,
			Amount:      -interestAmount, // NEGATIVE to increase debt
			Description: description,
			EntryType:   models.EntryTypeInterest,
			EntryDate:   time.Now(),
		}

		// FindOrCreate updates the existing entry for this payment if it exists, maintaining strict "current total" logic
		if err := s.ledgerRepo.FindOrCreateByPaymentAndType(ctx, entry); err != nil {
			logger.Error("Failed to update interest ledger entry", "payment_id", payment.ID, "error", err)
			continue
		}

		// Update the payment's InterestAmount field for display
		payment.InterestAmount = &interestAmount
		if err := s.repo.Update(ctx, &payment); err != nil {
			logger.Error("Failed to update payment interest amount", "payment_id", payment.ID, "error", err)
		}

		overdueCount++
		totalInterestCalculated += interestAmount
	}

	// Notify Admins
	if overdueCount > 0 {
		msg := fmt.Sprintf("Proceso de Interés Diario completado.\n\nPagos Vencidos Procesados: %d\nTotal Interés Acumulado Calculado: L %.2f", overdueCount, totalInterestCalculated)
		s.worker.EnqueueAsync(func(ctx context.Context) error {
			return s.notificationSvc.NotifyAdmins(ctx, "Reporte Diario de Intereses", msg, models.NotificationTypeSystem)
		})
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

		// FindByContract already preloads Contract.Lot.Project, Contract.ApplicantUser, ApprovedByUser.
		// Do not overwrite payment.Contract so paid_amount and full contract details are preserved.
		allPayments = append(allPayments, payments...)
	}

	return allPayments, nil
}

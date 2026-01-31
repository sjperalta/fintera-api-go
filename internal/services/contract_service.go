package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/statemachine"
)

type ContractService struct {
	repo            repository.ContractRepository
	lotRepo         repository.LotRepository
	userRepo        repository.UserRepository
	paymentRepo     repository.PaymentRepository
	ledgerRepo      *repository.LedgerRepository
	notificationSvc *NotificationService
	emailSvc        *EmailService
	auditSvc        *AuditService
	worker          *jobs.Worker
	paymentSchedule *PaymentScheduleService
}

func NewContractService(
	repo repository.ContractRepository,
	lotRepo repository.LotRepository,
	userRepo repository.UserRepository,
	paymentRepo repository.PaymentRepository,
	ledgerRepo *repository.LedgerRepository,
	notificationSvc *NotificationService,
	emailSvc *EmailService,
	auditSvc *AuditService,
	worker *jobs.Worker,
) *ContractService {
	return &ContractService{
		repo:            repo,
		lotRepo:         lotRepo,
		userRepo:        userRepo,
		paymentRepo:     paymentRepo,
		ledgerRepo:      ledgerRepo,
		notificationSvc: notificationSvc,
		emailSvc:        emailSvc,
		auditSvc:        auditSvc,
		worker:          worker,
		paymentSchedule: NewPaymentScheduleService(),
	}
}

// FindByID gets a contract by ID
func (s *ContractService) FindByID(ctx context.Context, id uint) (*models.Contract, error) {
	return s.repo.FindByID(ctx, id)
}

// FindByIDWithDetails gets a contract by ID with all nested associations preloaded
func (s *ContractService) FindByIDWithDetails(ctx context.Context, id uint) (*models.Contract, error) {
	return s.repo.FindByIDWithDetails(ctx, id)
}

func (s *ContractService) List(ctx context.Context, query *repository.ContractQuery) ([]models.Contract, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *ContractService) Create(ctx context.Context, contract *models.Contract) error {
	// Verify lot is available
	lot, err := s.lotRepo.FindByID(ctx, contract.LotID)
	if err != nil {
		return err
	}
	if !lot.IsAvailable() {
		return errors.New("el lote no está disponible")
	}

	// Create contract
	// Calculate balance: Amount - Reserve - DownPayment
	// Assuming contract.Amount is already set or derived from lot price
	// If not set, use Lot EffectivePrice
	if contract.Amount == nil || *contract.Amount == 0 {
		price := lot.EffectivePrice()
		contract.Amount = &price
	}

	// Initial balance is the full amount (as debt)
	// It will be reduced as payments (including reserve and down payment) are approved
	balance := -(*contract.Amount)
	contract.Balance = &balance

	if err := s.repo.Create(ctx, contract); err != nil {
		return err
	}

	// Update lot status to reserved
	lot.Status = models.LotStatusReserved
	s.lotRepo.Update(ctx, lot)

	// Notify admins asynchronously
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.notificationSvc.NotifyAdmins(ctx,
			"Nueva solicitud de contrato",
			"Se ha recibido una nueva solicitud de contrato",
			models.NotificationTypeContractApproved)
	})

	// Audit log
	s.auditSvc.Log(ctx, contract.ApplicantUserID, "CREATE", "Contract", contract.ID,
		fmt.Sprintf("Solicitud de contrato creada para el lote %s del proyecto %s. Precio: %.2f", lot.Name, lot.Project.Name, *contract.Amount), "", "")

	return nil
}

func (s *ContractService) Update(ctx context.Context, contract *models.Contract) error {
	return s.repo.Update(ctx, contract)
}

func (s *ContractService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepo.FindByEmail(ctx, email)
}

func (s *ContractService) GetUserByID(ctx context.Context, id uint) (*models.User, error) {
	return s.userRepo.FindByID(ctx, id)
}

func (s *ContractService) GetUserByIdentity(ctx context.Context, identity string) (*models.User, error) {
	return s.userRepo.FindByIdentity(ctx, identity)
}

func (s *ContractService) CreateUser(ctx context.Context, user *models.User, password string) error {
	// TODO: Handle password hashing before creating user or update repository to handle password
	return s.userRepo.Create(ctx, user)
}

// Approve approves a contract
func (s *ContractService) Approve(ctx context.Context, id uint) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use FSM to validate and transition state
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Approve(ctx); err != nil {
		return nil, fmt.Errorf("cannot approve contract: %w", err)
	}

	// Set approval timestamp
	now := time.Now()
	contract.ApprovedAt = &now
	contract.Active = true

	// Update contract
	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Create initial ledger entry (contract amount as debit)
	if contract.Amount != nil {
		initialEntry := &models.ContractLedgerEntry{
			ContractID:  contract.ID,
			Amount:      -(*contract.Amount), // Negative representing debt
			Description: "Monto Inicial del Contrato",
			EntryType:   models.EntryTypeInitial,
			EntryDate:   now,
		}
		if err := s.ledgerRepo.Create(ctx, initialEntry); err != nil {
			return nil, fmt.Errorf("failed to create initial ledger entry: %w", err)
		}
	}

	// Generate payment schedule
	payments, err := s.paymentSchedule.GenerateSchedule(ctx, contract)
	if err != nil {
		return nil, fmt.Errorf("failed to generate payment schedule: %w", err)
	}

	// Create payments
	for _, payment := range payments {
		if err := s.paymentRepo.Create(ctx, &payment); err != nil {
			return nil, fmt.Errorf("failed to create payment: %w", err)
		}
	}

	// Update lot status to reserved (it moves to financed when first payment is made)
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusReserved
		s.lotRepo.Update(ctx, lot)
	}

	// Notify user asynchronously
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
			"Contrato aprobado",
			"Tu solicitud de contrato ha sido aprobada",
			models.NotificationTypeContractApproved)
	})

	// Audit log
	// Assuming creator/approver ID is available in context or contract.CreatorID.
	// For now, using ApplicantUserID as a fallback if CreatorID is nil, but ideally it should be the admin performing the action.
	// Since Approve takes context, we might extract user from context if available in middleware, but for now we log the action on the contract.
	userID := contract.ApplicantUserID
	if contract.CreatorID != nil {
		userID = *contract.CreatorID
	}
	s.auditSvc.Log(ctx, userID, "APPROVE", "Contract", contract.ID,
		fmt.Sprintf("Contrato aprobado. Lote: %s, Precio: %.2f", contract.Lot.Name, *contract.Amount), "", "")

	return contract, nil
}

func (s *ContractService) Reject(ctx context.Context, id uint, reason string) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use FSM to validate and transition state
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Reject(ctx); err != nil {
		return nil, fmt.Errorf("cannot reject contract: %w", err)
	}

	contract.RejectionReason = &reason

	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Release lot
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusAvailable
		s.lotRepo.Update(ctx, lot)
	}

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
			"Contrato rechazado",
			"Tu solicitud de contrato ha sido rechazada: "+reason,
			models.NotificationTypeContractRejected)
	})

	// Audit log
	s.auditSvc.Log(ctx, contract.ApplicantUserID, "REJECT", "Contract", contract.ID,
		fmt.Sprintf("Contrato rechazado. Razón: %s", reason), "", "")

	return contract, nil
}

func (s *ContractService) Cancel(ctx context.Context, id uint, note string) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use FSM to validate and transition state
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Cancel(ctx); err != nil {
		return nil, fmt.Errorf("cannot cancel contract: %w", err)
	}

	contract.Note = &note
	contract.Active = false

	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Delete pending/submitted payments
	for _, payment := range contract.Payments {
		if payment.Status == models.PaymentStatusPending || payment.Status == models.PaymentStatusSubmitted {
			s.paymentRepo.Delete(ctx, payment.ID)
		}
	}

	// Delete ledger entries
	s.ledgerRepo.DeleteByContractID(ctx, contract.ID)

	// Release lot
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusAvailable
		s.lotRepo.Update(ctx, lot)
	}

	// Audit log
	s.auditSvc.Log(ctx, contract.ApplicantUserID, "CANCEL", "Contract", contract.ID,
		fmt.Sprintf("Contrato cancelado. Nota: %s", note), "", "")

	return contract, nil
}

// Close closes a contract when balance is paid off
func (s *ContractService) Close(ctx context.Context, id uint) (*models.Contract, error) {
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}

	// Calculate current balance
	balance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate balance: %w", err)
	}

	if balance < 0 {
		return nil, fmt.Errorf("cannot close contract with outstanding balance: %.2f", balance)
	}

	// Use FSM to validate and transition
	fsm := statemachine.NewContractFSM(contract)
	if err := fsm.Close(ctx); err != nil {
		return nil, fmt.Errorf("cannot close contract: %w", err)
	}

	// Set closed timestamp
	now := time.Now()
	contract.ClosedAt = &now

	if err := s.repo.Update(ctx, contract); err != nil {
		return nil, err
	}

	// Update lot status to fully paid
	lot, _ := s.lotRepo.FindByID(ctx, contract.LotID)
	if lot != nil {
		lot.Status = models.LotStatusFullyPaid
		s.lotRepo.Update(ctx, lot)
	}

	// Notify user
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.notificationSvc.NotifyUser(ctx, contract.ApplicantUserID,
			"Contrato completado",
			"¡Felicidades! Tu contrato ha sido completado",
			models.NotificationTypeContractApproved)
	})

	// Audit log
	s.auditSvc.Log(ctx, contract.ApplicantUserID, "CLOSE", "Contract", contract.ID,
		"Contrato completado y cerrado exitosamente", "", "")

	return contract, nil
}

// CapitalRepayment applies a capital repayment to the contract, marking pending installments as paid
// and adjusting the last partially covered installment.
func (s *ContractService) CapitalRepayment(ctx context.Context, id uint, amount float64, actorID uint, ip, userAgent string) error {
	if amount <= 0 {
		return fmt.Errorf("repayment amount must be greater than 0")
	}

	// 1. Fetch contract with details (including payments)
	contract, err := s.repo.FindByIDWithDetails(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load contract: %w", err)
	}

	// Check contract is approved
	if contract.Status != models.ContractStatusApproved {
		return fmt.Errorf("can only apply capital repayment to approved contracts")
	}

	// 2. Create Payment record for the repayment (to show in schedule/history)
	now := time.Now()
	repaymentDescription := fmt.Sprintf("Abono a Capital: L%.2f", amount)
	repaymentPayment := &models.Payment{
		ContractID:  contract.ID,
		Amount:      amount,
		PaidAmount:  &amount,
		PaymentDate: &now,
		Status:      models.PaymentStatusPaid,
		PaymentType: models.PaymentTypeCapitalRepayment,
		Description: &repaymentDescription,
		ApprovedAt:  &now,
		DueDate:     now, // Using now as due date for recording purposes
	}
	if err := s.paymentRepo.Create(ctx, repaymentPayment); err != nil {
		return fmt.Errorf("failed to create repayment payment record: %w", err)
	}

	// 3. Create prepayment ledger entry (credit) and link to payment
	prepaymentEntry := &models.ContractLedgerEntry{
		ContractID:  contract.ID,
		PaymentID:   &repaymentPayment.ID,
		Amount:      amount, // Positive (payment/credit)
		Description: repaymentDescription,
		EntryType:   models.EntryTypePrepayment,
		EntryDate:   now,
	}
	if err := s.ledgerRepo.Create(ctx, prepaymentEntry); err != nil {
		return fmt.Errorf("failed to create ledger entry: %w", err)
	}

	// 4. Process pending installment payments (Strategy: Reduce Term - Delete from end)
	remainingAmount := amount
	payments := contract.Payments

	// Filter for actionable installments
	var targets []*models.Payment
	for i := range payments {
		p := &payments[i]
		if p.PaymentType == models.PaymentTypeInstallment && (p.Status == models.PaymentStatusPending || p.Status == models.PaymentStatusSubmitted) {
			targets = append(targets, p)
		}
	}

	// Iterate backwards (last payments first)
	for i := len(targets) - 1; i >= 0 && remainingAmount > 0; i-- {
		target := targets[i]

		// Apply capital payment ONLY to Principal (Amount)
		// We do not pay off future interest with capital payments; we simply delete the principal obligation.
		targetAmount := target.Amount

		if remainingAmount >= targetAmount {
			// Fully covered - DELETE the payment to shorten term
			if err := s.paymentRepo.Delete(ctx, target.ID); err != nil {
				return fmt.Errorf("failed to delete payment #%d: %w", target.ID, err)
			}
			remainingAmount -= targetAmount
		} else {
			// Partially covered - Reduce amount
			target.Amount -= remainingAmount
			desc := fmt.Sprintf("Ajustado por abono a capital de L%.2f", remainingAmount)
			if target.Description != nil {
				desc = fmt.Sprintf("%s (%s)", *target.Description, desc)
			}
			target.Description = &desc

			if err := s.paymentRepo.Update(ctx, target); err != nil {
				return fmt.Errorf("failed to update payment #%d: %w", target.ID, err)
			}
			remainingAmount = 0
		}
	}

	// 4. Update contract balance and lot status if needed
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		// Calculate current balance
		balance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
		if err == nil {
			contract.Balance = &balance

			// Clear associations to prevent GORM from re-saving/resurrecting deleted payments
			// The contract object still has the stale 'Payments' slice from the original fetch,
			// which includes the payments we just deleted. Saving the contract with these
			// associations would re-insert them.
			contract.Payments = nil
			contract.LedgerEntries = nil

			s.repo.Update(ctx, contract)

			// Auto-close if balance >= 0
			if balance >= 0 {
				s.Close(ctx, contract.ID)
			}
		}
		return nil
	})

	// 5. Audit log
	s.auditSvc.Log(ctx, actorID, "CAPITAL_REPAYMENT", "Contract", contract.ID,
		fmt.Sprintf("Abono a capital de L%.2f aplicado al contrato #%d", amount, contract.ID), ip, userAgent)

	return nil
}

// ApplyPrepayment applies a capital repayment to reduce contract balance
func (s *ContractService) ApplyPrepayment(ctx context.Context, id uint, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("prepayment amount must be greater than 0")
	}

	contract, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load contract: %w", err)
	}

	// Check contract is approved
	if contract.Status != models.ContractStatusApproved {
		return fmt.Errorf("can only apply prepayment to approved contracts")
	}

	// Calculate current balance
	balance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
	if err != nil {
		return fmt.Errorf("failed to calculate balance: %w", err)
	}

	// Check if prepayment exceeds debt (balance is negative, so we check if amount > -balance)
	if amount > -balance {
		return fmt.Errorf("prepayment amount %.2f exceeds balance %.2f", amount, balance)
	}

	// Create prepayment ledger entry (negative amount = credit)
	prepaymentEntry := &models.ContractLedgerEntry{
		ContractID:  contract.ID,
		Amount:      amount, // Positive (payment)
		Description: fmt.Sprintf("Abono a Capital: L%.2f", amount),
		EntryType:   models.EntryTypePrepayment,
		EntryDate:   time.Now(),
	}
	if err := s.ledgerRepo.Create(ctx, prepaymentEntry); err != nil {
		return fmt.Errorf("failed to create prepayment entry: %w", err)
	}

	// Recalculate balance
	newBalance, err := s.ledgerRepo.CalculateBalance(ctx, contract.ID)
	if err != nil {
		return fmt.Errorf("failed to recalculate balance: %w", err)
	}

	// Close contract if balance is now <= 0
	// Close contract if balance is now >= 0 (fully paid)
	if newBalance >= 0 {
		if _, err := s.Close(ctx, contract.ID); err != nil {
			return fmt.Errorf("failed to close contract: %w", err)
		}
	}

	return nil
}

func (s *ContractService) ReleaseUnpaidReservations(ctx context.Context) error {
	// Find reservations older than 48 hours without payment
	contracts, err := s.repo.FindPendingReservations(ctx, 48)
	if err != nil {
		return err
	}

	for _, contract := range contracts {
		s.Cancel(ctx, contract.ID, "Reservación no pagada a tiempo")
	}

	return nil
}

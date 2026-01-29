package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
)

// PaymentScheduleService handles payment schedule generation
type PaymentScheduleService struct{}

// NewPaymentScheduleService creates a new payment schedule service
func NewPaymentScheduleService() *PaymentScheduleService {
	return &PaymentScheduleService{}
}

// GenerateSchedule creates payment schedule for an approved contract
func (s *PaymentScheduleService) GenerateSchedule(ctx context.Context, contract *models.Contract) ([]models.Payment, error) {
	if contract.Amount == nil || *contract.Amount <= 0 {
		return nil, fmt.Errorf("contract amount is required")
	}
	if contract.DownPayment == nil || *contract.DownPayment < 0 {
		return nil, fmt.Errorf("down payment is required")
	}
	if contract.ReserveAmount == nil || *contract.ReserveAmount < 0 {
		return nil, fmt.Errorf("reserve amount is required")
	}

	var payments []models.Payment
	now := time.Now()

	// 1. Reservation payment (if reserve amount > 0)
	if *contract.ReserveAmount > 0 {
		reservationPayment := models.Payment{
			ContractID:  contract.ID,
			Amount:      *contract.ReserveAmount,
			DueDate:     now.AddDate(0, 0, 7), // Due in 7 days
			Status:      models.PaymentStatusPending,
			PaymentType: models.PaymentTypeReservation,
			Description: stringPtr("Pago de Reserva"),
		}
		payments = append(payments, reservationPayment)
	}

	// 2. Down payment (if down payment > 0)
	if *contract.DownPayment > 0 {
		downPaymentDue := now.AddDate(0, 0, 14) // Due in 14 days
		if *contract.ReserveAmount > 0 {
			downPaymentDue = now.AddDate(0, 0, 21) // Due in 21 days if there's a reservation
		}

		downPayment := models.Payment{
			ContractID:  contract.ID,
			Amount:      *contract.DownPayment,
			DueDate:     downPaymentDue,
			Status:      models.PaymentStatusPending,
			PaymentType: models.PaymentTypeDownPayment,
			Description: stringPtr("Pago Inicial"),
		}
		payments = append(payments, downPayment)
	}

	// 3. Calculate installments
	totalPaid := *contract.ReserveAmount + *contract.DownPayment
	remainingAmount := *contract.Amount - totalPaid

	if remainingAmount > 0 && contract.PaymentTerm > 0 {
		// Avoid cents in installments: Use Floor to get a round number for base installments
		baseInstallment := math.Floor(remainingAmount / float64(contract.PaymentTerm))

		// The first payment picks up the remainder/difference
		firstInstallmentAmount := remainingAmount - (baseInstallment * float64(contract.PaymentTerm-1))

		// Start installments 1 month after approval
		firstInstallmentDate := now.AddDate(0, 1, 0)

		for i := 0; i < contract.PaymentTerm; i++ {
			dueDate := firstInstallmentDate.AddDate(0, i, 0)

			// First payment gets the remainder, others get the base rounded amount
			amount := baseInstallment
			if i == 0 {
				amount = firstInstallmentAmount
			}

			installment := models.Payment{
				ContractID:  contract.ID,
				Amount:      amount,
				DueDate:     dueDate,
				Status:      models.PaymentStatusPending,
				PaymentType: models.PaymentTypeInstallment,
				Description: stringPtr(fmt.Sprintf("Cuota %d de %d", i+1, contract.PaymentTerm)),
			}
			payments = append(payments, installment)
		}
	} else if remainingAmount > 0 {
		// Full payment if no payment term specified
		fullPaymentDue := now.AddDate(0, 1, 0)
		fullPayment := models.Payment{
			ContractID:  contract.ID,
			Amount:      remainingAmount,
			DueDate:     fullPaymentDue,
			Status:      models.PaymentStatusPending,
			PaymentType: models.PaymentTypeFull,
			Description: stringPtr("Pago Total"),
		}
		payments = append(payments, fullPayment)
	}

	return payments, nil
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

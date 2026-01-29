package statemachine

import (
	"context"
	"fmt"

	"github.com/looplab/fsm"
	"github.com/sjperalta/fintera-api/internal/models"
)

// PaymentFSM wraps a payment with its state machine
type PaymentFSM struct {
	payment *models.Payment
	fsm     *fsm.FSM
}

// NewPaymentFSM creates a new payment state machine
func NewPaymentFSM(payment *models.Payment) *PaymentFSM {
	pfsm := &PaymentFSM{
		payment: payment,
	}

	pfsm.fsm = fsm.NewFSM(
		payment.Status,
		fsm.Events{
			// pending → submitted (requires receipt)
			{Name: "submit", Src: []string{models.PaymentStatusPending}, Dst: models.PaymentStatusSubmitted},

			// pending/submitted → paid
			{Name: "approve", Src: []string{models.PaymentStatusPending, models.PaymentStatusSubmitted}, Dst: models.PaymentStatusPaid},

			// submitted → rejected
			{Name: "reject", Src: []string{models.PaymentStatusSubmitted}, Dst: models.PaymentStatusRejected},

			// paid → submitted (undo)
			{Name: "undo", Src: []string{models.PaymentStatusPaid}, Dst: models.PaymentStatusSubmitted},

			// pending → readjustment
			{Name: "readjustment", Src: []string{models.PaymentStatusPending}, Dst: models.PaymentStatusReadjustment},
		},
		fsm.Callbacks{},
	)

	return pfsm
}

// Submit transitions payment to submitted state
func (p *PaymentFSM) Submit(ctx context.Context) error {
	if !p.payment.MaySubmit() {
		return fmt.Errorf("payment cannot be submitted in current state: %s", p.payment.Status)
	}

	if err := p.fsm.Event(ctx, "submit"); err != nil {
		return fmt.Errorf("failed to submit payment: %w", err)
	}

	p.payment.Status = p.fsm.Current()
	return nil
}

// Approve transitions payment to paid state
func (p *PaymentFSM) Approve(ctx context.Context) error {
	if !p.payment.MayApprove() {
		return fmt.Errorf("payment cannot be approved in current state: %s", p.payment.Status)
	}

	if err := p.fsm.Event(ctx, "approve"); err != nil {
		return fmt.Errorf("failed to approve payment: %w", err)
	}

	p.payment.Status = p.fsm.Current()
	return nil
}

// Reject transitions payment to rejected state
func (p *PaymentFSM) Reject(ctx context.Context) error {
	if !p.payment.MayReject() {
		return fmt.Errorf("payment cannot be rejected in current state: %s", p.payment.Status)
	}

	if err := p.fsm.Event(ctx, "reject"); err != nil {
		return fmt.Errorf("failed to reject payment: %w", err)
	}

	p.payment.Status = p.fsm.Current()
	return nil
}

// Undo transitions payment from paid back to submitted
func (p *PaymentFSM) Undo(ctx context.Context) error {
	if !p.payment.MayUndo() {
		return fmt.Errorf("payment cannot be undone in current state: %s", p.payment.Status)
	}

	if err := p.fsm.Event(ctx, "undo"); err != nil {
		return fmt.Errorf("failed to undo payment: %w", err)
	}

	p.payment.Status = p.fsm.Current()
	return nil
}

// Readjustment transitions payment to readjustment state
func (p *PaymentFSM) Readjustment(ctx context.Context) error {
	if err := p.fsm.Event(ctx, "readjustment"); err != nil {
		return fmt.Errorf("failed to readjust payment: %w", err)
	}

	p.payment.Status = p.fsm.Current()
	return nil
}

// Current returns the current state
func (p *PaymentFSM) Current() string {
	return p.fsm.Current()
}

// Can checks if a transition is possible
func (p *PaymentFSM) Can(event string) bool {
	return p.fsm.Can(event)
}

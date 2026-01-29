package statemachine

import (
	"context"
	"fmt"

	"github.com/looplab/fsm"
	"github.com/sjperalta/fintera-api/internal/models"
)

// ContractFSM wraps a contract with its state machine
type ContractFSM struct {
	contract *models.Contract
	fsm      *fsm.FSM
}

// NewContractFSM creates a new contract state machine
func NewContractFSM(contract *models.Contract) *ContractFSM {
	cffsm := &ContractFSM{
		contract: contract,
	}

	cffsm.fsm = fsm.NewFSM(
		contract.Status,
		fsm.Events{
			// pending/rejected → submitted
			{Name: "submit", Src: []string{models.ContractStatusPending, models.ContractStatusRejected}, Dst: models.ContractStatusSubmitted},

			// pending/submitted/rejected → approved
			{Name: "approve", Src: []string{models.ContractStatusPending, models.ContractStatusSubmitted, models.ContractStatusRejected}, Dst: models.ContractStatusApproved},

			// pending/submitted → rejected
			{Name: "reject", Src: []string{models.ContractStatusPending, models.ContractStatusSubmitted}, Dst: models.ContractStatusRejected},

			// pending/submitted/rejected → cancelled
			{Name: "cancel", Src: []string{models.ContractStatusPending, models.ContractStatusSubmitted, models.ContractStatusRejected}, Dst: models.ContractStatusCancelled},

			// approved → closed
			{Name: "close", Src: []string{models.ContractStatusApproved}, Dst: models.ContractStatusClosed},

			// closed → approved (reopen)
			{Name: "reopen", Src: []string{models.ContractStatusClosed}, Dst: models.ContractStatusApproved},
		},
		fsm.Callbacks{},
	)

	return cffsm
}

// Submit transitions contract to submitted state
func (c *ContractFSM) Submit(ctx context.Context) error {
	if !c.contract.MaySubmit() {
		return fmt.Errorf("contract cannot be submitted in current state: %s", c.contract.Status)
	}

	if err := c.fsm.Event(ctx, "submit"); err != nil {
		return fmt.Errorf("failed to submit contract: %w", err)
	}

	c.contract.Status = c.fsm.Current()
	return nil
}

// Approve transitions contract to approved state
func (c *ContractFSM) Approve(ctx context.Context) error {
	if !c.contract.MayApprove() {
		return fmt.Errorf("contract cannot be approved in current state: %s", c.contract.Status)
	}

	if err := c.fsm.Event(ctx, "approve"); err != nil {
		return fmt.Errorf("failed to approve contract: %w", err)
	}

	c.contract.Status = c.fsm.Current()
	return nil
}

// Reject transitions contract to rejected state
func (c *ContractFSM) Reject(ctx context.Context) error {
	if !c.contract.MayReject() {
		return fmt.Errorf("contract cannot be rejected in current state: %s", c.contract.Status)
	}

	if err := c.fsm.Event(ctx, "reject"); err != nil {
		return fmt.Errorf("failed to reject contract: %w", err)
	}

	c.contract.Status = c.fsm.Current()
	return nil
}

// Cancel transitions contract to cancelled state
func (c *ContractFSM) Cancel(ctx context.Context) error {
	if !c.contract.MayCancel() {
		return fmt.Errorf("contract cannot be cancelled in current state: %s", c.contract.Status)
	}

	if err := c.fsm.Event(ctx, "cancel"); err != nil {
		return fmt.Errorf("failed to cancel contract: %w", err)
	}

	c.contract.Status = c.fsm.Current()
	return nil
}

// Close transitions contract to closed state
func (c *ContractFSM) Close(ctx context.Context) error {
	if !c.contract.MayClose() {
		return fmt.Errorf("contract cannot be closed: balance must be <= 0")
	}

	if err := c.fsm.Event(ctx, "close"); err != nil {
		return fmt.Errorf("failed to close contract: %w", err)
	}

	c.contract.Status = c.fsm.Current()
	return nil
}

// Reopen transitions contract from closed back to approved
func (c *ContractFSM) Reopen(ctx context.Context) error {
	if !c.contract.MayReopen() {
		return fmt.Errorf("contract cannot be reopened in current state: %s", c.contract.Status)
	}

	if err := c.fsm.Event(ctx, "reopen"); err != nil {
		return fmt.Errorf("failed to reopen contract: %w", err)
	}

	c.contract.Status = c.fsm.Current()
	return nil
}

// Current returns the current state
func (c *ContractFSM) Current() string {
	return c.fsm.Current()
}

// Can checks if a transition is possible
func (c *ContractFSM) Can(event string) bool {
	return c.fsm.Can(event)
}

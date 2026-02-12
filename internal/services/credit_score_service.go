package services

import (
	"context"
	"fmt"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

// CreditScoreService handles credit score calculations
type CreditScoreService struct {
	userRepo     repository.UserRepository
	contractRepo repository.ContractRepository
	paymentRepo  repository.PaymentRepository
}

func NewCreditScoreService(userRepo repository.UserRepository, contractRepo repository.ContractRepository, paymentRepo repository.PaymentRepository) *CreditScoreService {
	return &CreditScoreService{
		userRepo:     userRepo,
		contractRepo: contractRepo,
		paymentRepo:  paymentRepo,
	}
}

// UpdateScore calculates and updates credit score for a single user
func (s *CreditScoreService) UpdateScore(ctx context.Context, userID uint) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Calculate credit score based on payment history
	score := s.calculateCreditScore(ctx, userID)

	// Update user's credit score
	user.CreditScore = score
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update credit score: %w", err)
	}

	logger.Info(fmt.Sprintf("[CreditScoreService] Updated credit score for user %d: %d", userID, score))
	return nil
}

// UpdateAllScores updates credit scores for all users
func (s *CreditScoreService) UpdateAllScores(ctx context.Context) error {
	logger.Info("[CreditScoreService] Updating all user credit scores...")

	// Process users in batches
	page := 1
	pageSize := 100
	totalProcessed := 0

	for {
		// Use List query for pagination
		query := repository.NewListQuery()
		query.Page = page
		query.PerPage = pageSize

		users, total, err := s.userRepo.List(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to fetch users page %d: %w", page, err)
		}

		if len(users) == 0 {
			break
		}

		for _, user := range users {
			if err := s.UpdateScore(ctx, user.ID); err != nil {
				logger.Error(fmt.Sprintf("[CreditScoreService] Error updating score for user %d: %v", user.ID, err))
				continue
			}
			totalProcessed++
		}

		// Check if we've processed all users
		if int64(totalProcessed) >= total || len(users) < pageSize {
			break
		}

		page++
	}

	logger.Info(fmt.Sprintf("[CreditScoreService] Updated credit scores for %d users", totalProcessed))
	return nil
}

// calculateCreditScore calculates credit score based on payment history
func (s *CreditScoreService) calculateCreditScore(ctx context.Context, userID uint) int {
	baseScore := 500 // Starting score

	// Get user's contracts
	contracts, err := s.contractRepo.FindByUser(ctx, userID)
	if err != nil {
		return baseScore
	}

	for _, contract := range contracts {
		// Get payments for this contract
		payments, err := s.paymentRepo.FindByContract(ctx, contract.ID)
		if err != nil {
			continue
		}

		for _, payment := range payments {
			if payment.Status == models.PaymentStatusPaid && payment.PaymentDate != nil {
				// Calculate days late
				daysLate := int(payment.PaymentDate.Sub(payment.DueDate).Hours() / 24)

				if daysLate <= 0 {
					// On-time payment: +5 points
					baseScore += 5
				} else if daysLate <= 7 {
					// 1-7 days late: -2 points
					baseScore -= 2
				} else if daysLate <= 30 {
					// 8-30 days late: -5 points
					baseScore -= 5
				} else {
					// 30+ days late: -10 points
					baseScore -= 10
				}
			}
		}

		// Penalize cancelled contracts
		if contract.Status == models.ContractStatusCancelled {
			baseScore -= 20
		}

		// Bonus for closed contracts (fully paid)
		if contract.Status == models.ContractStatusClosed {
			baseScore += 50
		}
	}

	// Ensure score stays within reasonable bounds
	if baseScore < 300 {
		baseScore = 300
	}
	if baseScore > 850 {
		baseScore = 850
	}

	return baseScore
}

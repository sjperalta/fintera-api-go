package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// GenerateRecoveryCode generates a 6-digit random code
func GenerateRecoveryCode() (string, error) {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[num.Int64()]
	}
	return string(code), nil
}

// SendRecoveryCode generates and sends a recovery code to the user's email
func (s *UserService) SendRecoveryCode(ctx context.Context, email string) error {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether email exists or not
		return nil
	}

	// Generate 6-digit code
	code, err := GenerateRecoveryCode()
	if err != nil {
		return fmt.Errorf("failed to generate recovery code: %w", err)
	}

	// Save code and timestamp
	user.RecoveryCode = &code
	now := time.Now()
	user.RecoveryCodeSentAt = &now

	if err := s.repo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to save recovery code: %w", err)
	}

	// Send email asynchronously
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.emailService.SendRecoveryCode(ctx, user, code)
	})

	return nil
}

// VerifyRecoveryCode checks if the recovery code is valid
func (s *UserService) VerifyRecoveryCode(ctx context.Context, email, code string) (bool, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return false, nil // Don't reveal user existence
	}

	if user.RecoveryCode == nil || user.RecoveryCodeSentAt == nil {
		return false, nil
	}

	// Check if code matches
	if *user.RecoveryCode != code {
		return false, nil
	}

	// Check if code hasn't expired (15 minutes)
	if time.Since(*user.RecoveryCodeSentAt) > 15*time.Minute {
		return false, nil
	}

	return true, nil
}

// UpdatePasswordWithCode updates password using recovery code
func (s *UserService) UpdatePasswordWithCode(ctx context.Context, email, code, newPassword string) error {
	// Verify code first
	valid, err := s.VerifyRecoveryCode(ctx, email, code)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidRecoveryCode
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Hash new password
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password and clear recovery code
	user.EncryptedPassword = hashedPassword
	user.RecoveryCode = nil
	user.RecoveryCodeSentAt = nil

	return s.repo.Update(ctx, user)
}

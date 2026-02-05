package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/pkg/logger"
)

// GenerateTempPassword generates a 5-character password with: at least 1 number, 1 uppercase letter, and 1 symbol.
func GenerateTempPassword() (string, error) {
	const (
		digits   = "0123456789"
		uppers   = "ABCDEFGHJKLMNPQRSTUVWXYZ" // exclude I,O for clarity
		symbols  = "!@#$%&*"
	)
	charsets := []string{digits, uppers, symbols}
	result := make([]byte, 5)

	// Ensure at least one of each required type
	for i, charset := range charsets {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}

	// Fill remaining 2 slots from combined charset
	all := digits + uppers + symbols
	for i := 3; i < 5; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(all))))
		if err != nil {
			return "", err
		}
		result[i] = all[n.Int64()]
	}

	// Shuffle
	for i := len(result) - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		result[i], result[j.Int64()] = result[j.Int64()], result[i]
	}
	return string(result), nil
}

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
	email = strings.TrimSpace(email)
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

	now := time.Now()
	if err := s.repo.SetRecoveryCode(ctx, user.ID, code, now); err != nil {
		return fmt.Errorf("failed to save recovery code: %w", err)
	}
	logger.Info("[Recovery] Code saved for user", "user_id", user.ID)

	// Send email asynchronously
	s.worker.EnqueueAsync(func(ctx context.Context) error {
		return s.emailService.SendRecoveryCode(ctx, user, code)
	})

	return nil
}

// VerifyRecoveryCode checks if the recovery code is valid
func (s *UserService) VerifyRecoveryCode(ctx context.Context, email, code string) (bool, error) {
	email = strings.TrimSpace(email)
	code = strings.TrimSpace(code)
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return false, nil // Don't reveal user existence
	}

	if user.RecoveryCode == nil || user.RecoveryCodeSentAt == nil {
		logger.Info("[Recovery] Verify failed: recovery_code or recovery_code_sent_at is nil", "user_id", user.ID)
		return false, nil
	}

	// Check if code matches
	if *user.RecoveryCode != code {
		logger.Info("[Recovery] Verify failed: code mismatch", "user_id", user.ID, "code_len", len(code), "stored_len", len(*user.RecoveryCode))
		return false, nil
	}

	// Check if code hasn't expired (15 minutes)
	if time.Since(*user.RecoveryCodeSentAt) > 15*time.Minute {
		logger.Info("[Recovery] Verify failed: code expired", "user_id", user.ID)
		return false, nil
	}

	return true, nil
}

// UpdatePasswordWithCode updates password using recovery code
func (s *UserService) UpdatePasswordWithCode(ctx context.Context, email, code, newPassword string) error {
	email = strings.TrimSpace(email)
	code = strings.TrimSpace(code)
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
	user.MustChangePassword = false

	return s.repo.Update(ctx, user)
}

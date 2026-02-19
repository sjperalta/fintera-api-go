package services

import (
	"testing"

	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestEmailService_checkEmailPreconditions(t *testing.T) {
	logger.Setup("test")

	// Test case 1: Email notifications disabled
	cfg := &config.Config{
		EnableEmailNotifications: false,
	}
	service := NewEmailService(cfg)
	user := &models.User{Email: "test@example.com", FullName: "Test User", ID: 1}

	ok, err := service.checkEmailPreconditions(user, "test operation")
	assert.False(t, ok, "Should return false when notifications are disabled")
	assert.Nil(t, err, "Should not return error when notifications are disabled")

	// Test case 2: Email configured and valid
	cfg = &config.Config{
		EnableEmailNotifications: true,
		ResendAPIKey:             "test_key",
		FromEmail:                "from@example.com",
	}
	service = NewEmailService(cfg)

	ok, err = service.checkEmailPreconditions(user, "test operation")
	assert.True(t, ok, "Should return true when properly configured")
	assert.Nil(t, err, "Should not return error when properly configured")

	// Test case 3: Email not configured (missing key)
	cfg = &config.Config{
		EnableEmailNotifications: true,
		ResendAPIKey:             "", // Missing key
		FromEmail:                "from@example.com",
	}
	service = NewEmailService(cfg)

	ok, err = service.checkEmailPreconditions(user, "test operation")
	assert.False(t, ok, "Should return false when config is missing")
	assert.Error(t, err, "Should return error when config is missing")
	assert.Contains(t, err.Error(), "RESEND_API_KEY is not set")

	// Test case 4: Invalid email
	cfg = &config.Config{
		EnableEmailNotifications: true,
		ResendAPIKey:             "test_key",
		FromEmail:                "from@example.com",
	}
	service = NewEmailService(cfg)
	userInvalid := &models.User{Email: "", FullName: "Invalid User", ID: 2}

	ok, err = service.checkEmailPreconditions(userInvalid, "test operation")
	assert.False(t, ok, "Should return false when email is invalid")
	assert.Error(t, err, "Should return error when email is invalid")
	assert.Equal(t, "email address is empty", err.Error())
}

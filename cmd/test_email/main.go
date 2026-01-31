package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger.Setup("development")

	if cfg.ResendAPIKey == "" {
		log.Fatal("RESEND_API_KEY is not set")
	}

	// Initialize email service
	emailService := services.NewEmailService(cfg)

	// Create a mock user
	// Check if EMAIL_TO is set, otherwise use a default or ask user
	toEmail := os.Getenv("TEST_EMAIL_TO")
	if toEmail == "" {
		toEmail = "test@example.com"
		log.Println("TEST_EMAIL_TO not set, using test@example.com. Emails might mock or fail if domain not verified.")
	}

	user := &models.User{
		FullName: "Test User",
		Email:    toEmail,
	}

	// Send Account Created email
	log.Printf("Sending Account Created email to %s...", toEmail)
	err = emailService.SendAccountCreated(context.Background(), user)
	if err != nil {
		log.Fatalf("Failed to send Account Created email: %v", err)
	}
	log.Println("Account Created email sent successfully!")

	// Send Verification Code
	log.Printf("Sending Recovery Code email to %s...", toEmail)
	err = emailService.SendRecoveryCode(context.Background(), user, "123456")
	if err != nil {
		log.Fatalf("Failed to send Recovery Code email: %v", err)
	}
	log.Println("Recovery Code email sent successfully!")
}

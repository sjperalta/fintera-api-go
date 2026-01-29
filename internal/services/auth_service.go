package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication operations
type AuthService struct {
	userRepo         repository.UserRepository
	refreshTokenRepo repository.RefreshTokenRepository
	cfg              *config.Config
}

// NewAuthService creates a new auth service
func NewAuthService(userRepo repository.UserRepository, rtRepo repository.RefreshTokenRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		refreshTokenRepo: rtRepo,
		cfg:              cfg,
	}
}

// LoginResult represents the result of a login attempt
type LoginResult struct {
	Token        string              `json:"token"`
	RefreshToken string              `json:"refresh_token"`
	User         models.UserResponse `json:"user"`
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("credenciales inválidas")
	}

	// Check if user is active
	if !user.IsActive() {
		return nil, errors.New("cuenta inactiva o suspendida")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.EncryptedPassword), []byte(password)); err != nil {
		return nil, errors.New("credenciales inválidas")
	}

	// Generate JWT token
	token, err := s.generateJWT(user)
	if err != nil {
		return nil, errors.New("error al generar token")
	}

	// Generate refresh token
	refreshToken, err := s.generateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, errors.New("error al generar refresh token")
	}

	return &LoginResult{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user.ToResponse(),
	}, nil
}

// RefreshToken validates a refresh token and returns new tokens
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*LoginResult, error) {
	// Find refresh token
	rt, err := s.refreshTokenRepo.FindByToken(ctx, refreshToken)
	if err != nil {
		return nil, errors.New("token inválido")
	}

	// Check if expired
	if rt.IsExpired() {
		s.refreshTokenRepo.Delete(ctx, refreshToken)
		return nil, errors.New("token expirado")
	}

	// Find user
	user, err := s.userRepo.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, errors.New("usuario no encontrado")
	}

	// Check if user is active
	if !user.IsActive() {
		return nil, errors.New("cuenta inactiva o suspendida")
	}

	// Delete old refresh token
	s.refreshTokenRepo.Delete(ctx, refreshToken)

	// Generate new JWT token
	token, err := s.generateJWT(user)
	if err != nil {
		return nil, errors.New("error al generar token")
	}

	// Generate new refresh token
	newRefreshToken, err := s.generateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, errors.New("error al generar refresh token")
	}

	return &LoginResult{
		Token:        token,
		RefreshToken: newRefreshToken,
		User:         user.ToResponse(),
	}, nil
}

// Logout invalidates a refresh token
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.refreshTokenRepo.Delete(ctx, refreshToken)
}

// generateJWT creates a new JWT token for a user
func (s *AuthService) generateJWT(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Duration(s.cfg.JWTExpirationHours) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// generateRefreshToken creates a new refresh token
func (s *AuthService) generateRefreshToken(ctx context.Context, userID uint) (string, error) {
	// Generate random token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	// Set expiration (30 days)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	// Save to database
	rt := &models.RefreshToken{
		UserID:    userID,
		Token:     token,
		ExpiresAt: &expiresAt,
	}

	if err := s.refreshTokenRepo.Create(ctx, rt); err != nil {
		return "", err
	}

	return token, nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// VerifyPassword compares a password with a hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

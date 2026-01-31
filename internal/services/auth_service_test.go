package services

import (
	"context"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
)

type mockUserRepo struct {
	repository.UserRepository
	mockFindByEmail func(ctx context.Context, email string) (*models.User, error)
	mockFindByID    func(ctx context.Context, id uint) (*models.User, error)
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return m.mockFindByEmail(ctx, email)
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	return m.mockFindByID(ctx, id)
}

func TestAuthService_Login_InactiveUser(t *testing.T) {
	mockRepo := &mockUserRepo{}
	service := NewAuthService(mockRepo, nil, nil)

	mockRepo.mockFindByEmail = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{
			Email:  email,
			Status: models.StatusInactive,
		}, nil
	}

	result, err := service.Login(context.Background(), "inactive@example.com", "password")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, "cuenta inactiva o suspendida", err.Error())
}

func TestAuthService_RefreshToken_InactiveUser(t *testing.T) {
	mockRepo := &mockUserRepo{}
	mockRTRepo := &mockmockRTRepo{}
	service := NewAuthService(mockRepo, mockRTRepo, nil)

	mockRTRepo.mockFindByToken = func(ctx context.Context, token string) (*models.RefreshToken, error) {
		return &models.RefreshToken{UserID: 1}, nil
	}
	mockRepo.mockFindByID = func(ctx context.Context, id uint) (*models.User, error) {
		return &models.User{
			ID:     id,
			Status: models.StatusInactive,
		}, nil
	}

	result, err := service.RefreshToken(context.Background(), "token")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, "cuenta inactiva o suspendida", err.Error())
}

type mockmockRTRepo struct {
	repository.RefreshTokenRepository
	mockFindByToken func(ctx context.Context, token string) (*models.RefreshToken, error)
	mockDelete      func(ctx context.Context, token string) error
}

func (m *mockmockRTRepo) FindByToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	return m.mockFindByToken(ctx, token)
}

func (m *mockmockRTRepo) Delete(ctx context.Context, token string) error {
	if m.mockDelete != nil {
		return m.mockDelete(ctx, token)
	}
	return nil
}

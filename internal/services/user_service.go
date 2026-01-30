package services

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// UserService handles user-related business logic
type UserService struct {
	repo         repository.UserRepository
	worker       *jobs.Worker
	emailService *EmailService
}

func NewUserService(repo repository.UserRepository, worker *jobs.Worker, emailService *EmailService) *UserService {
	return &UserService{
		repo:         repo,
		worker:       worker,
		emailService: emailService,
	}
}

func (s *UserService) FindByID(ctx context.Context, id uint) (*models.User, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.repo.FindByEmail(ctx, email)
}

func (s *UserService) List(ctx context.Context, query *repository.ListQuery) ([]models.User, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *UserService) Create(ctx context.Context, user *models.User, password string) error {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}
	user.EncryptedPassword = hashedPassword
	return s.repo.Create(ctx, user)
}

func (s *UserService) Update(ctx context.Context, user *models.User) error {
	return s.repo.Update(ctx, user)
}

func (s *UserService) Delete(ctx context.Context, id uint) error {
	return s.repo.SoftDelete(ctx, id)
}

func (s *UserService) Restore(ctx context.Context, id uint) error {
	return s.repo.Restore(ctx, id)
}

func (s *UserService) ToggleStatus(ctx context.Context, id uint) (*models.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user.Status == models.StatusActive {
		user.Status = models.StatusInactive
	} else {
		user.Status = models.StatusActive
	}
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID uint, currentPassword, newPassword string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !VerifyPassword(currentPassword, user.EncryptedPassword) {
		return ErrInvalidPassword
	}
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.EncryptedPassword = hashedPassword
	return s.repo.Update(ctx, user)
}

func (s *UserService) ForceChangePassword(ctx context.Context, userID uint, newPassword string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.EncryptedPassword = hashedPassword
	return s.repo.Update(ctx, user)
}

func (s *UserService) UpdateLocale(ctx context.Context, userID uint, locale string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	user.Locale = locale
	return s.repo.Update(ctx, user)
}

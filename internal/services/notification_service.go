package services

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

type NotificationService struct {
	repo     repository.NotificationRepository
	userRepo repository.UserRepository
}

func NewNotificationService(repo repository.NotificationRepository, userRepo repository.UserRepository) *NotificationService {
	return &NotificationService{repo: repo, userRepo: userRepo}
}

func (s *NotificationService) FindByID(ctx context.Context, id uint) (*models.Notification, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *NotificationService) FindByUser(ctx context.Context, userID uint, query *repository.ListQuery) ([]models.Notification, int64, error) {
	return s.repo.FindByUser(ctx, userID, query)
}

func (s *NotificationService) Create(ctx context.Context, notification *models.Notification) error {
	return s.repo.Create(ctx, notification)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, id uint) error {
	notification, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	notification.MarkAsRead()
	return s.repo.Update(ctx, notification)
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uint) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

func (s *NotificationService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *NotificationService) NotifyUser(ctx context.Context, userID uint, title, message, notifType string) error {
	notification := &models.Notification{
		UserID:           userID,
		Title:            title,
		Message:          message,
		NotificationType: &notifType,
	}
	return s.repo.Create(ctx, notification)
}

func (s *NotificationService) NotifyAdmins(ctx context.Context, title, message, notifType string) error {
	admins, err := s.userRepo.FindAdmins(ctx)
	if err != nil {
		return err
	}
	for _, admin := range admins {
		notification := &models.Notification{
			UserID:           admin.ID,
			Title:            title,
			Message:          message,
			NotificationType: &notifType,
		}
		s.repo.Create(ctx, notification)
	}
	return nil
}

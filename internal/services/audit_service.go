package services

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// Log records an audit entry
func (s *AuditService) Log(ctx context.Context, userID uint, action, entity string, entityID uint, details, ip, userAgent string) error {
	logEntry := &models.AuditLog{
		UserID:    userID,
		Action:    action,
		Entity:    entity,
		EntityID:  entityID,
		Details:   details,
		IPAddress: ip,
		UserAgent: userAgent,
	}
	return s.db.Create(logEntry).Error
}

// List retrieves audit logs with filters
func (s *AuditService) List(ctx context.Context, limit, offset int, entity string) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	db := s.db.Model(&models.AuditLog{})
	if entity != "" {
		db = db.Where("entity = ?", entity)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	result := db.Preload("User").Order("created_at desc").Limit(limit).Offset(offset).Find(&logs)
	return logs, total, result.Error
}

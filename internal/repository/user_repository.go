package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByIdentity(ctx context.Context, identity string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	SetRecoveryCode(ctx context.Context, userID uint, code string, sentAt time.Time) error
	Delete(ctx context.Context, id uint) error
	SoftDelete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	List(ctx context.Context, query *ListQuery) ([]models.User, int64, error)
	FindAdmins(ctx context.Context) ([]models.User, error)
	FindAll(ctx context.Context) ([]models.User, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("discarded_at IS NULL").
		First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("LOWER(email) = LOWER(?) AND discarded_at IS NULL", email).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByIdentity(ctx context.Context, identity string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("identity = ? AND discarded_at IS NULL", identity).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		if isDuplicateKeyError(err, "users_identity_key") {
			return errors.New("Ya existe un usuario con este documento de identidad")
		}
		if isDuplicateKeyError(err, "users_email_key") {
			return errors.New("Ya existe un usuario con este correo electrónico")
		}
		return err
	}
	return nil
}

func isDuplicateKeyError(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == constraintName
	}
	return false
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) SetRecoveryCode(ctx context.Context, userID uint, code string, sentAt time.Time) error {
	sentAt = sentAt.UTC()
	u := &models.User{
		RecoveryCode:       &code,
		RecoveryCodeSentAt: &sentAt,
	}
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Select("RecoveryCode", "RecoveryCodeSentAt").
		Updates(u).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.User{}, id).Error
}

func (r *userRepository) SoftDelete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("discarded_at", gorm.Expr("NOW()")).Error
}

func (r *userRepository) Restore(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("discarded_at", nil).Error
}

func (r *userRepository) List(ctx context.Context, query *ListQuery) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	db := r.db.WithContext(ctx).Model(&models.User{}).Where("discarded_at IS NULL")

	// Apply search
	if query.Search != "" {
		search := "%" + query.Search + "%"
		db = db.Where("full_name ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR identity ILIKE ?",
			search, search, search, search)
	}

	// Apply role filter
	if query.Filters["role"] != "" {
		db = db.Where("role = ?", query.Filters["role"])
	}

	// Apply status filter
	if query.Filters["status"] != "" {
		db = db.Where("status = ?", query.Filters["status"])
	}

	// Filter by creator (e.g. users created by a specific seller)
	if query.Filters["created_by"] != "" {
		db = db.Where("created_by = ?", query.Filters["created_by"])
	}

	// Count total
	db.Count(&total)

	// Apply sorting
	if query.SortBy != "" {
		order := query.SortBy
		if query.SortDir == "desc" {
			order += " DESC"
		}
		db = db.Order(order)
	} else {
		db = db.Order("created_at DESC")
	}

	// Apply pagination
	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	err := db.Find(&users).Error
	return users, total, err
}

func (r *userRepository) FindAdmins(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).
		Where("role = ? AND status = ? AND discarded_at IS NULL", models.RoleAdmin, models.StatusActive).
		Find(&users).Error
	return users, err
}

func (r *userRepository) FindAll(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).
		Where("discarded_at IS NULL").
		Find(&users).Error
	return users, err
}

// ListQuery represents common query parameters
type ListQuery struct {
	Page    int
	PerPage int
	Search  string
	SortBy  string
	SortDir string
	Filters map[string]string
}

// NewListQuery creates a ListQuery with defaults
func NewListQuery() *ListQuery {
	return &ListQuery{
		Page:    1,
		PerPage: 20,
		Filters: make(map[string]string),
	}
}

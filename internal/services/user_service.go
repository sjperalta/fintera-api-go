package services

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// UserService handles user-related business logic
type UserService struct {
	repo         repository.UserRepository
	contractRepo repository.ContractRepository
	worker       *jobs.Worker
	emailService *EmailService
	auditSvc     *AuditService
	imageService *ImageService
}

func NewUserService(repo repository.UserRepository, contractRepo repository.ContractRepository, worker *jobs.Worker, emailService *EmailService, auditSvc *AuditService, imageService *ImageService) *UserService {
	return &UserService{
		repo:         repo,
		contractRepo: contractRepo,
		worker:       worker,
		emailService: emailService,
		auditSvc:     auditSvc,
		imageService: imageService,
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

func (s *UserService) Create(ctx context.Context, user *models.User, password string, actorID uint) error {
	user.Email = strings.ToLower(user.Email)

	// If password is empty, generate a 5-char temp password (number, uppercase, symbol)
	tempPassword := ""
	if password == "" {
		pw, err := GenerateTempPassword()
		if err != nil {
			return fmt.Errorf("failed to generate temp password: %w", err)
		}
		password = pw
		tempPassword = pw
		user.MustChangePassword = true
	}

	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}
	user.EncryptedPassword = hashedPassword
	if err := s.repo.Create(ctx, user); err != nil {
		return err
	}
	if err := s.emailService.SendAccountCreated(ctx, user, tempPassword); err != nil {
		// Log but don't fail user creation; welcome email is best-effort
		_ = err
	}
	return s.auditSvc.Log(ctx, actorID, "CREATE", "User", user.ID, fmt.Sprintf("Usuario creado: %s (%s) - Rol: %s", user.FullName, user.Email, user.Role), "", "")
}

func (s *UserService) Update(ctx context.Context, user *models.User, actorID uint) error {
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "UPDATE", "User", user.ID, fmt.Sprintf("Usuario actualizado: %s", user.Email), "", "")
}

func (s *UserService) Delete(ctx context.Context, id uint, actorID uint) error {
	// Check for active contracts
	hasActive, err := s.contractRepo.HasActiveContracts(ctx, id)
	if err != nil {
		return fmt.Errorf("error checking user contracts: %w", err)
	}
	if hasActive {
		return fmt.Errorf("cannot delete user with active contracts")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "DELETE", "User", id, "Usuario eliminado permanentemente (hard delete)", "", "")
}

func (s *UserService) Restore(ctx context.Context, id uint, actorID uint) error {
	if err := s.repo.Restore(ctx, id); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "RESTORE", "User", id, "Usuario restaurado", "", "")
}

func (s *UserService) ToggleStatus(ctx context.Context, id uint, actorID uint) (*models.User, error) {
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
	s.auditSvc.Log(ctx, actorID, "TOGGLE_STATUS", "User", id, fmt.Sprintf("Estado cambiado a %s", user.Status), "", "")
	return user, nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID uint, currentPassword, newPassword string, actorID uint) error {
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
	user.MustChangePassword = false
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "CHANGE_PASSWORD", "User", userID, "Contraseña actualizada por el usuario", "", "")
}

func (s *UserService) ForceChangePassword(ctx context.Context, userID uint, newPassword string, actorID uint) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.EncryptedPassword = hashedPassword
	user.MustChangePassword = false
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}
	return s.auditSvc.Log(ctx, actorID, "FORCE_CHANGE_PASSWORD", "User", userID, "Contraseña restablecida por administrador", "", "")
}

func (s *UserService) UpdateLocale(ctx context.Context, userID uint, locale string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	user.Locale = locale
	return s.repo.Update(ctx, user)
}

// ResendConfirmation sends the account-created (welcome/confirmation) email to the user.
func (s *UserService) ResendConfirmation(ctx context.Context, userID uint) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	return s.emailService.SendAccountCreated(ctx, user, "")
}

func (s *UserService) UpdateProfilePicture(ctx context.Context, userID uint, file multipart.File, header *multipart.FileHeader, actorID uint) (*models.User, error) {
	// Process image
	originalPath, thumbPath, err := s.imageService.ProcessAndSaveProfilePicture(file, header)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.ProfilePicture = &originalPath
	user.ProfilePictureThumb = &thumbPath

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		if err := s.auditSvc.Log(ctx, actorID, "UPDATE_PICTURE", "User", userID, "Foto de perfil actualizada", "", ""); err != nil {
			// Log error but don't fail the request
			_ = err
		}
	}

	return user, nil
}

package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
)

type UserHandler struct {
	userService    *services.UserService
	paymentService *services.PaymentService
}

func NewUserHandler(userService *services.UserService, paymentService *services.PaymentService) *UserHandler {
	return &UserHandler{
		userService:    userService,
		paymentService: paymentService,
	}
}

// @Summary List Users
// @Description Get a paginated list of users
// @Tags Users
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param search_term query string false "Search by name or email"
// @Param role query string false "Filter by role"
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /users [get]
func (h *UserHandler) Index(c *gin.Context) {
	query := repository.NewListQuery()
	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	query.Search = c.Query("search_term")

	// Sellers only see users with role "user" (clients/leads)
	currentRole := strings.ToLower(middleware.GetUserRole(c))
	if currentRole == "seller" {
		query.Filters["role"] = "user"
	} else {
		query.Filters["role"] = c.Query("role")
	}

	// When my_clients=1 (e.g. from "Ver todos los clientes"), filter to users created by current user
	if c.Query("my_clients") == "1" {
		currentUserID := middleware.GetUserID(c)
		if currentUserID > 0 {
			query.Filters["created_by"] = strconv.FormatUint(uint64(currentUserID), 10)
		}
	}

	status := c.Query("status")
	if status == "" {
		status = models.StatusActive
	} else if status == "all" {
		status = ""
	}
	query.Filters["status"] = status

	users, total, err := h.userService.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.UserResponse
	for _, u := range users {
		responses = append(responses, u.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"users": responses,
		"pagination": gin.H{
			"page":        query.Page,
			"per_page":    query.PerPage,
			"total":       total,
			"total_pages": (total + int64(query.PerPage) - 1) / int64(query.PerPage),
		},
	})
}

// @Summary Get User
// @Description Get a user by ID
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} models.UserResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id} [get]
func (h *UserHandler) Show(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("user_id"), 10, 32)
	user, err := h.userService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user.ToResponse()})
}

type CreateUserRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	FullName       string `json:"full_name"`
	FullNamePascal string `json:"FullName"` // Support PascalCase from some frontends/tools
	Phone          string `json:"phone"`
	Role           string `json:"role"`
	Identity       string `json:"identity"`
	RTN            string `json:"rtn"`
	Address        string `json:"address"`
}

// @Summary Create User
// @Description Create a new user
// @Tags Users
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "User Data"
// @Success 201 {object} models.UserResponse
// @Failure 400 {object} map[string]string
// @Security BearerAuth
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Support both full_name and FullName
	if req.FullName == "" && req.FullNamePascal != "" {
		req.FullName = req.FullNamePascal
	}

	// Manual validation for FullName since we removed binding:"required" to support the alias
	if req.FullName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Key: 'CreateUserRequest.FullName' Error:Field validation for 'FullName' failed on the 'required' tag"})
		return
	}

	creatorRole := middleware.GetUserRole(c)
	if creatorRole == "seller" && req.Role != "user" && req.Role != "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Los vendedores solo pueden crear usuarios con rol 'user'"})
		return
	}

	creatorID := middleware.GetUserID(c)
	user := &models.User{
		Email:     req.Email,
		FullName:  req.FullName,
		Phone:     req.Phone,
		Role:      req.Role,
		Identity:  req.Identity,
		RTN:       req.RTN,
		CreatedBy: &creatorID,
	}
	if req.Address != "" {
		user.Address = &req.Address
	}

	if err := h.userService.Create(c.Request.Context(), user, req.Password, creatorID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": user.ToResponse(), "message": "Usuario creado exitosamente"})
}

// @Summary Update User
// @Description Update user details (admin: any field; owner: full_name, phone, address, locale, identity, rtn only)
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Param request body map[string]string true "User Fields"
// @Success 200 {object} models.UserResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("user_id"), 10, 32)
	user, err := h.userService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isAdmin := middleware.IsAdmin(c)

	if v, ok := req["full_name"].(string); ok {
		user.FullName = v
	}
	if v, ok := req["phone"].(string); ok {
		user.Phone = v
	}
	if v, ok := req["address"].(string); ok {
		user.Address = &v
	}
	if v, ok := req["locale"].(string); ok {
		user.Locale = v
	}
	if v, ok := req["identity"].(string); ok {
		user.Identity = v
	}
	if v, ok := req["rtn"].(string); ok {
		user.RTN = v
	}

	// Only admin may change role, status, or email
	if isAdmin {
		if v, ok := req["role"].(string); ok {
			user.Role = v
		}
		if v, ok := req["status"].(string); ok {
			user.Status = v
		}
		if v, ok := req["email"].(string); ok {
			user.Email = v
		}
	}

	actorID := middleware.GetUserID(c)
	if err := h.userService.Update(c.Request.Context(), user, actorID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user.ToResponse(), "message": "Usuario actualizado exitosamente"})
}

// @Summary Delete User
// @Description Soft delete a user
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("user_id"), 10, 32)
	actorID := middleware.GetUserID(c)
	if err := h.userService.Delete(c.Request.Context(), uint(id), actorID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Usuario eliminado exitosamente"})
}

// @Summary Toggle User Status
// @Description Enable or disable a user
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} models.UserResponse
// @Security BearerAuth
// @Router /users/{user_id}/toggle_status [put]
func (h *UserHandler) ToggleStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("user_id"), 10, 32)
	actorID := middleware.GetUserID(c)
	user, err := h.userService.ToggleStatus(c.Request.Context(), uint(id), actorID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user.ToResponse(), "message": "Estado actualizado"})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

// @Summary Change Password
// @Description Change current user's password
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Param request body ChangePasswordRequest true "Password Data"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id}/change_password [patch]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("user_id"), 10, 32)
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current user ID and Role from context (assuming middleware sets these)
	currentUserID := middleware.GetUserID(c)
	currentUserRole := middleware.GetUserRole(c)

	// If admin is changing another user's password, force change without old password
	if currentUserRole == "admin" && uint(id) != currentUserID {
		if err := h.userService.ForceChangePassword(c.Request.Context(), uint(id), req.NewPassword, currentUserID); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Standard password change requires current password
		if req.CurrentPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is required"})
			return
		}
		if err := h.userService.ChangePassword(c.Request.Context(), uint(id), req.CurrentPassword, req.NewPassword, currentUserID); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contraseña actualizada exitosamente"})
}

// @Summary Resend Confirmation
// @Description Resend email confirmation
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id}/resend_confirmation [post]
func (h *UserHandler) ResendConfirmation(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de usuario inválido"})
		return
	}

	currentUserID := middleware.GetUserID(c)
	currentUserRole := middleware.GetUserRole(c)
	// Only the user themselves or an admin can resend confirmation
	if uint(userID) != currentUserID && currentUserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para reenviar la confirmación de este usuario"})
		return
	}

	if err := h.userService.ResendConfirmation(c.Request.Context(), uint(userID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al reenviar el email de confirmación"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email de confirmación reenviado"})
}

// @Summary Get User Contracts
// @Description List contracts for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /users/{user_id}/contracts [get]
func (h *UserHandler) Contracts(c *gin.Context) {
	// TODO: Implement
	c.JSON(http.StatusOK, gin.H{"contracts": []interface{}{}})
}

// @Summary Get User Payments
// @Description List payments for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /users/{user_id}/payments [get]
func (h *UserHandler) Payments(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de usuario inválido"})
		return
	}

	payments, err := h.paymentService.GetUserPayments(c.Request.Context(), uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener pagos"})
		return
	}

	// Use same response shape as admin payment list so paid_amount (including overpayments) is always a number
	responses := make([]models.PaymentResponse, 0, len(payments))
	for _, p := range payments {
		responses = append(responses, p.ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{"payments": responses})
}

// @Summary Get Payment History
// @Description Get payment history for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /users/{user_id}/payment_history [get]
func (h *UserHandler) PaymentHistory(c *gin.Context) {
	// TODO: Implement
	c.JSON(http.StatusOK, gin.H{"payment_history": []interface{}{}})
}

// @Summary Get User Summary
// @Description Get summary stats for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /users/{user_id}/summary [get]
func (h *UserHandler) Summary(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de usuario inválido"})
		return
	}

	summary, err := h.paymentService.GetUserFinancingSummary(c.Request.Context(), uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener resumen"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// @Summary Restore User
// @Description Restore a soft-deleted user
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id}/restore [post]
func (h *UserHandler) Restore(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	actorID := middleware.GetUserID(c)
	if err := h.userService.Restore(c.Request.Context(), uint(id), actorID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Usuario restaurado exitosamente"})
}

type UpdateLocaleRequest struct {
	Locale string `json:"locale" binding:"required"`
}

// @Summary Update Locale
// @Description Update user's locale preference
// @Tags Users
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Param request body UpdateLocaleRequest true "Locale Data"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /users/{user_id}/update_locale [patch]
func (h *UserHandler) UpdateLocale(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req UpdateLocaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userService.UpdateLocale(c.Request.Context(), uint(id), req.Locale); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Idioma actualizado"})
}

type SendRecoveryCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// @Summary Send Recovery Code
// @Description Send password recovery code to email
// @Tags Users
// @Accept json
// @Produce json
// @Param request body SendRecoveryCodeRequest true "Email"
// @Success 200 {object} map[string]string
// @Router /users/send_recovery_code [post]
func (h *UserHandler) SendRecoveryCode(c *gin.Context) {
	var req SendRecoveryCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userService.SendRecoveryCode(c.Request.Context(), req.Email); err != nil {
		// Log the error for debugging
		// assuming "github.com/sjperalta/fintera-api/pkg/logger" is imported or available via h.
		// Since logger isn't imported in this file, we might need to add it or just rely on the service logging I added.
		// Wait, I added logging to EmailService, but UserService might wrap it.
		// check UserService.SendRecoveryCode.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al enviar código"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Código de recuperación enviado"})
}

type VerifyRecoveryCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// @Summary Verify Recovery Code
// @Description Verify if recovery code is valid
// @Tags Users
// @Accept json
// @Produce json
// @Param request body VerifyRecoveryCodeRequest true "Verification Data"
// @Success 200 {object} map[string]bool
// @Router /users/verify_recovery_code [post]
func (h *UserHandler) VerifyRecoveryCode(c *gin.Context) {
	var req VerifyRecoveryCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	valid, err := h.userService.VerifyRecoveryCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al verificar código"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": valid})
}

type UpdatePasswordWithCodeRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// @Summary Reset Password
// @Description Reset password using recovery code
// @Tags Users
// @Accept json
// @Produce json
// @Param request body UpdatePasswordWithCodeRequest true "Reset Data"
// @Success 200 {object} map[string]string
// @Router /users/update_password_with_code [post]
func (h *UserHandler) UpdatePasswordWithCode(c *gin.Context) {
	var req UpdatePasswordWithCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userService.UpdatePasswordWithCode(c.Request.Context(), req.Email, req.Code, req.NewPassword); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Código inválido o expirado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contraseña actualizada"})
}

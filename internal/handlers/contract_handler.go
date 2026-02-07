package handlers

import (
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
)

type ContractHandler struct {
	contractService *services.ContractService
	storage         *storage.LocalStorage
}

func NewContractHandler(contractService *services.ContractService, storage *storage.LocalStorage) *ContractHandler {
	return &ContractHandler{contractService: contractService, storage: storage}
}

// @Summary List Contracts
// @Description Get a paginated list of contracts for the current user (or all for admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param search_term query string false "Search term"
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /contracts [get]
func (h *ContractHandler) Index(c *gin.Context) {
	query := &repository.ContractQuery{ListQuery: repository.NewListQuery()}
	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	query.Search = c.Query("search_term")
	query.Status = c.Query("status")
	if startDate := c.Query("start_date"); startDate != "" {
		query.Filters["start_date"] = startDate
	}
	if endDate := c.Query("end_date"); endDate != "" {
		query.Filters["end_date"] = endDate
	}
	query.UserID = middleware.GetUserID(c)
	query.IsAdmin = middleware.IsAdmin(c)

	contracts, total, err := h.contractService.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, contract := range contracts {
		responses = append(responses, contract.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"contracts": responses,
		"pagination": gin.H{
			"page":        query.Page,
			"per_page":    query.PerPage,
			"total":       total,
			"total_pages": (total + int64(query.PerPage) - 1) / int64(query.PerPage),
		},
	})
}

// @Summary Get Contract Stats
// @Description Get contract count statistics (Total, Pending, Approved, Rejected)
// @Tags Contracts
// @Accept json
// @Produce json
// @Success 200 {object} repository.ContractStats
// @Security BearerAuth
// @Router /contracts/stats [get]
func (h *ContractHandler) GetStats(c *gin.Context) {
	stats, err := h.contractService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// @Summary Get Contract
// @Description Get a contract by ID
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} models.ContractResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id} [get]
func (h *ContractHandler) Show(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	contract, err := h.contractService.FindByIDWithDetails(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse()})
}

// @Summary Create Contract
// @Description Create a new contract with optional new user and documents
// @Tags Contracts
// @Accept multipart/form-data
// @Produce json
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param contract formData string true "Contract Data (JSON)"
// @Param user formData string false "User Data (JSON)"
// @Param documents formData file false "Documents"
// @Success 201 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts [post]
func (h *ContractHandler) Create(c *gin.Context) {
	// 1. Extract IDs from path
	lotID, _ := strconv.ParseUint(c.Param("lot_id"), 10, 32)
	creatorID := middleware.GetUserID(c)

	// 2. Parse Form Data
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing form data: " + err.Error()})
		return
	}

	// 3. Extract Contract Fields
	paymentTerm, _ := strconv.Atoi(c.Request.FormValue("contract[payment_term]"))
	financingType := strings.TrimSpace(strings.ToLower(c.Request.FormValue("contract[financing_type]")))
	reserveAmount, _ := strconv.ParseFloat(c.Request.FormValue("contract[reserve_amount]"), 64)
	downPayment, _ := strconv.ParseFloat(c.Request.FormValue("contract[down_payment]"), 64)
	maxPaymentDateStr := strings.TrimSpace(c.Request.FormValue("contract[max_payment_date]"))
	note := c.Request.FormValue("contract[note]")
	applicantUserIDStr := c.Request.FormValue("contract[applicant_user_id]")

	// Validate and parse max_payment_date when financing type is bank or cash
	var maxPaymentDate *time.Time
	if financingType == models.FinancingTypeBank || financingType == models.FinancingTypeCash {
		if maxPaymentDateStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha máxima de pago es requerida para financiamiento bancario o contado"})
			return
		}
		parsed, err := time.Parse("2006-01-02", maxPaymentDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha máxima de pago debe tener formato YYYY-MM-DD"})
			return
		}
		today := time.Now().Truncate(24 * time.Hour)
		maxDateOnly := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		todayOnly := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		if maxDateOnly.Before(todayOnly) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha máxima de pago no puede ser anterior a hoy"})
			return
		}
		maxPaymentDate = &parsed
	}

	// 4. Handle User (Find or Create)
	var applicantID uint
	if applicantUserIDStr != "" {
		// Existing User
		id, _ := strconv.ParseUint(applicantUserIDStr, 10, 32)
		user, err := h.contractService.GetUserByID(c.Request.Context(), uint(id))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario no encontrado"})
			return
		}

		if !user.IsActive() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El usuario no está activo y no puede crear contratos"})
			return
		}
		applicantID = user.ID
	} else {
		// Create New User
		email := strings.TrimSpace(c.Request.FormValue("user[email]"))
		identity := strings.TrimSpace(c.Request.FormValue("user[identity]"))

		// Required
		if email == "" || identity == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email e Identidad son requeridos para nuevos usuarios"})
			return
		}

		// Validate email format
		if _, err := mail.ParseAddress(email); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El correo electrónico no tiene un formato válido"})
			return
		}

		// Validate identity (non-empty, reasonable length to avoid garbage)
		if len(identity) < 5 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La identidad debe tener al menos 5 caracteres"})
			return
		}

		// Check duplicate by email
		existingUser, err := h.contractService.GetUserByEmail(c.Request.Context(), email)
		if err == nil && existingUser != nil {
			applicantID = existingUser.ID // Use existing user if found
		} else {
			// Check duplicate by identity
			existingUserByIdentity, err := h.contractService.GetUserByIdentity(c.Request.Context(), identity)
			if err == nil && existingUserByIdentity != nil {
				applicantID = existingUserByIdentity.ID
			} else {
				// Create new user
				newUser := &models.User{
					Email:    email,
					FullName: c.Request.FormValue("user[full_name]"),
					Phone:    c.Request.FormValue("user[phone]"),
					Identity: identity,
					RTN:      c.Request.FormValue("user[rtn]"),
					Role:     models.RoleUser,
					Status:   models.StatusActive,
					Locale:   models.LocaleES,
				}
				// Generate 5-char temp password (number, uppercase, symbol) - user must change on first login
				tempPassword, err := services.GenerateTempPassword()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generando contraseña temporal"})
					return
				}
				if err := h.contractService.CreateUser(c.Request.Context(), newUser, tempPassword); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creando usuario: " + err.Error()})
					return
				}
				applicantID = newUser.ID
			}
		}
	}

	// 5. Handle Document Uploads
	var documentPaths []string
	form, _ := c.MultipartForm()
	files := form.File

	// Iterate looking for documents[0], documents[1], etc.
	// Since we don't know the exact keys or if they are array style "documents[]", generic iteration is harder with the map structure
	// But the frontend sends: formData.append(`documents[${index}]`, doc);

	// We can iterate the map, but order might not be preserved (not critical for paths list)
	// We can iterate the map, but order might not be preserved (not critical for paths list)
	for key, fileHeaders := range files {
		// Check if key starts with "documents"
		if len(key) >= 9 && key[:9] == "documents" {
			for _, fileHeader := range fileHeaders {
				file, err := fileHeader.Open()
				if err != nil {
					continue
				}
				defer file.Close()

				if !storage.IsValidContentType(fileHeader.Header.Get("Content-Type")) {
					continue
				}

				path, err := h.storage.Upload(file, fileHeader, "contracts")
				if err != nil {
					continue
				}
				documentPaths = append(documentPaths, path)
			}
		}
	}

	// 6. Create Contract Object
	contract := &models.Contract{
		LotID:           uint(lotID),
		ApplicantUserID: applicantID,
		CreatorID:       &creatorID,
		PaymentTerm:     paymentTerm,
		FinancingType:   financingType,
		ReserveAmount:   &reserveAmount,
		DownPayment:     &downPayment,
		MaxPaymentDate:  maxPaymentDate,
		Note:            &note,
		Status:          models.ContractStatusPending,
		Currency:        "HNL",
	}

	// 7. Call Service
	if err := h.contractService.Create(c.Request.Context(), contract); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Solicitud de contrato creada exitosamente", "contract_id": contract.ID})
}

// UpdateContractRequest is the body for PATCH contract (schedule-related fields only; allowed when status is pending/rejected/submitted)
type UpdateContractRequest struct {
	PaymentTerm    *int     `json:"payment_term"`
	ReserveAmount  *float64 `json:"reserve_amount"`
	DownPayment    *float64 `json:"down_payment"`
	MaxPaymentDate *string  `json:"max_payment_date"` // YYYY-MM-DD; for bank/cash
}

// @Summary Update Contract
// @Description Update contract schedule fields (payment_term, reserve_amount, down_payment). Allowed only when status is pending, rejected, or submitted. Schedule is recalculated on approval.
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param request body UpdateContractRequest true "Contract Data"
// @Success 200 {object} models.ContractResponse
// @Failure 400,403,404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id} [patch]
func (h *ContractHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	contract, err := h.contractService.FindByIDWithDetails(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}

	status := contract.Status
	if status != models.ContractStatusPending && status != models.ContractStatusRejected && status != models.ContractStatusSubmitted {
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo se pueden editar plazos, reserva y pago inicial cuando el contrato está en estado pendiente, rechazado o enviado"})
		return
	}

	var req UpdateContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.PaymentTerm != nil {
		if *req.PaymentTerm < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El plazo de pago debe ser al menos 1"})
			return
		}
		contract.PaymentTerm = *req.PaymentTerm
	}
	if req.ReserveAmount != nil {
		if *req.ReserveAmount < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El monto de reserva no puede ser negativo"})
			return
		}
		contract.ReserveAmount = req.ReserveAmount
	}
	if req.DownPayment != nil {
		if *req.DownPayment < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El pago inicial no puede ser negativo"})
			return
		}
		contract.DownPayment = req.DownPayment
	}
	if req.MaxPaymentDate != nil {
		ft := strings.ToLower(contract.FinancingType)
		if ft == models.FinancingTypeBank || ft == models.FinancingTypeCash {
			s := strings.TrimSpace(*req.MaxPaymentDate)
			if s == "" {
				contract.MaxPaymentDate = nil
			} else {
				parsed, err := time.Parse("2006-01-02", s)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha máxima de pago debe tener formato YYYY-MM-DD"})
					return
				}
				contract.MaxPaymentDate = &parsed
			}
		}
	}

	if err := h.contractService.Update(c.Request.Context(), contract); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato actualizado"})
}

// @Summary Approve Contract
// @Description Approve a pending contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} models.ContractResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/approve [post]
func (h *ContractHandler) Approve(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	contract, err := h.contractService.Approve(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato aprobado"})
}

type RejectContractRequest struct {
	Reason string `json:"reason"`
}

// @Summary Reject Contract
// @Description Reject a pending contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param request body RejectContractRequest true "Reason"
// @Success 200 {object} models.ContractResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/reject [post]
func (h *ContractHandler) Reject(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	var req RejectContractRequest
	c.ShouldBindJSON(&req)

	contract, err := h.contractService.Reject(c.Request.Context(), uint(id), req.Reason)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato rechazado"})
}

type CancelContractRequest struct {
	Note string `json:"note"`
}

// @Summary Cancel Contract
// @Description Cancel an active contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param request body CancelContractRequest true "Note"
// @Success 200 {object} models.ContractResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/cancel [post]
func (h *ContractHandler) Cancel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	var req CancelContractRequest
	c.ShouldBindJSON(&req)

	contract, err := h.contractService.Cancel(c.Request.Context(), uint(id), req.Note)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": contract.ToResponse(), "message": "Contrato cancelado"})
}

// @Summary Reopen Contract
// @Description Reopen a cancelled contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/reopen [post]
func (h *ContractHandler) Reopen(c *gin.Context) {
	// TODO: Implement
	c.JSON(http.StatusOK, gin.H{"message": "Contrato reabierto"})
}

type CapitalRepaymentRequest struct {
	Amount float64 `json:"capital_repayment_amount" binding:"required,gt=0"`
}

// @Summary Capital Repayment
// @Description Apply capital repayment to contract (Admin)
// @Tags Contracts
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param contract_id path int true "Contract ID"
// @Param request body CapitalRepaymentRequest true "Amount"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/capital_repayment [post]
func (h *ContractHandler) CapitalRepayment(c *gin.Context) {
	contractID, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	var req CapitalRepaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Monto requerido y debe ser mayor a 0"})
		return
	}

	err := h.contractService.CapitalRepayment(c.Request.Context(), uint(contractID), req.Amount,
		middleware.GetUserID(c),
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Fetch updated contract with details to return to frontend
	contract, err := h.contractService.FindByIDWithDetails(c.Request.Context(), uint(contractID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Abono a capital aplicado exitosamente, pero hubo un error al recargar los detalles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Abono a capital aplicado exitosamente",
		"contract": contract.ToResponse(),
	})
}

// @Summary Delete Rejected Contract
// @Description Delete a rejected contract and release the lot so it can be reserved again. Only allowed when contract status is rejected.
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]string
// @Failure 403,404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id} [delete]
func (h *ContractHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	if err := h.contractService.DeleteRejected(c.Request.Context(), uint(id)); err != nil {
		if err.Error() == "record not found" || strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
			return
		}
		if strings.Contains(err.Error(), "rechazado") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Contrato eliminado. El lote está disponible para nueva reserva"})
}

// @Summary Get Contract Ledger
// @Description Get ledger entries for a contract
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/ledger [get]
func (h *ContractHandler) Ledger(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	contract, err := h.contractService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contrato no encontrado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ledger_entries": contract.LedgerEntries})
}

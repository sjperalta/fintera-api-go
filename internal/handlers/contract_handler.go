package handlers

import (
	"net/http"
	"strconv"

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
	financingType := c.Request.FormValue("contract[financing_type]")
	reserveAmount, _ := strconv.ParseFloat(c.Request.FormValue("contract[reserve_amount]"), 64)
	downPayment, _ := strconv.ParseFloat(c.Request.FormValue("contract[down_payment]"), 64)
	note := c.Request.FormValue("contract[note]")
	applicantUserIDStr := c.Request.FormValue("contract[applicant_user_id]")

	// 4. Handle User (Find or Create)
	var applicantID uint
	if applicantUserIDStr != "" {
		// Existing User
		id, _ := strconv.ParseUint(applicantUserIDStr, 10, 32)
		applicantID = uint(id)
	} else {
		// Create New User
		// Check if email already exists
		email := c.Request.FormValue("user[email]")
		identity := c.Request.FormValue("user[identity]")

		// Basic validation
		if email == "" || identity == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email e Identidad son requeridos para nuevos usuarios"})
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
				// Set default password (should be changed by user later)
				tempPassword := "Fintera" + identity

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

// @Summary Update Contract
// @Description Update a contract
// @Tags Contracts
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param request body map[string]interface{} true "Contract Data"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id} [patch]
func (h *ContractHandler) Update(c *gin.Context) {
	// TODO: Implement
	c.JSON(http.StatusOK, gin.H{"message": "Contrato actualizado"})
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

	c.JSON(http.StatusOK, gin.H{"message": "Abono a capital aplicado exitosamente"})
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

package handlers

import (
	"net/http"
	"strconv"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
)

type PaymentHandler struct {
	paymentService *services.PaymentService
	storage        *storage.LocalStorage
}

func NewPaymentHandler(paymentService *services.PaymentService, storage *storage.LocalStorage) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService, storage: storage}
}

// @Summary List Payments
// @Description Get a paginated list of payments
// @Tags Payments
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /payments [get]
func (h *PaymentHandler) Index(c *gin.Context) {
	query := repository.NewListQuery()
	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	query.Filters["status"] = c.Query("status")
	query.Filters["start_date"] = c.Query("start_date")
	query.Filters["end_date"] = c.Query("end_date")

	if search := c.Query("search"); search != "" {
		query.Filters["search_term"] = search
	}
	if search := c.Query("search_term"); search != "" {
		query.Filters["search_term"] = search
	}
	if applicant := c.Query("applicant"); applicant != "" {
		query.Filters["search_term"] = applicant
	}

	// Parse sort parameter (format: field-direction)
	if sort := c.Query("sort"); sort != "" {
		parts := strings.Split(sort, "-")
		query.SortBy = parts[0]
		if len(parts) > 1 {
			query.SortDir = parts[1]
		}
	}

	payments, total, err := h.paymentService.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, p := range payments {
		responses = append(responses, p.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"payments": responses,
		"pagination": gin.H{
			"page":        query.Page,
			"per_page":    query.PerPage,
			"total":       total,
			"total_pages": (total + int64(query.PerPage) - 1) / int64(query.PerPage),
		},
	})
}

// @Summary Payment Statistics
// @Description Get payment statistics for revenue flow
// @Tags Payments
// @Accept json
// @Produce json
// @Param month query int true "Month (1-12)"
// @Param year query int true "Year (YYYY)"
// @Success 200 {object} []services.RevenuePoint
// @Security BearerAuth
// @Router /payments/statistics [get]
func (h *PaymentHandler) Statistics(c *gin.Context) {
	month, _ := strconv.Atoi(c.Query("month"))
	year, _ := strconv.Atoi(c.Query("year"))

	if month < 1 || month > 12 || year < 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month or year"})
		return
	}

	stats, err := h.paymentService.GetMonthlyStatistics(c.Request.Context(), month, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// @Summary Payment Stats
// @Description Get monthly payment statistics (pending, collected, overdue)
// @Tags Payments
// @Accept json
// @Produce json
// @Success 200 {object} repository.PaymentStats
// @Security BearerAuth
// @Router /payments/stats [get]
func (h *PaymentHandler) Stats(c *gin.Context) {
	stats, err := h.paymentService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// @Summary Get Payment
// @Description Get a payment by ID
// @Tags Payments
// @Accept json
// @Produce json
// @Param payment_id path int true "Payment ID"
// @Success 200 {object} models.PaymentResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /payments/{payment_id} [get]
func (h *PaymentHandler) Show(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("payment_id"), 10, 32)
	payment, err := h.paymentService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pago no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment": payment.ToResponse()})
}

type ApprovePaymentRequest struct {
	Amount         float64 `json:"amount"`
	InterestAmount float64 `json:"interest_amount"`
	PaidAmount     float64 `json:"paid_amount"`
	Payment        *struct {
		Amount         float64 `json:"amount"`
		InterestAmount float64 `json:"interest_amount"`
		PaidAmount     float64 `json:"paid_amount"`
	} `json:"payment"`
}

// @Summary Approve Payment
// @Description Approve a payment (Admin)
// @Tags Payments
// @Accept json
// @Produce json
// @Param payment_id path int true "Payment ID"
// @Param request body ApprovePaymentRequest true "Amount"
// @Success 200 {object} models.PaymentResponse
// @Security BearerAuth
// @Router /payments/{payment_id}/approve [post]
func (h *PaymentHandler) Approve(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("payment_id"), 10, 32)
	var req ApprovePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Ignore error if it's just empty body
	}

	amount := req.Amount
	interestAmount := req.InterestAmount
	paidAmount := req.PaidAmount

	// Fallback to nested payment if provided
	if req.Payment != nil {
		if amount == 0 {
			amount = req.Payment.Amount
		}
		if interestAmount == 0 {
			interestAmount = req.Payment.InterestAmount
		}
		if paidAmount == 0 {
			paidAmount = req.Payment.PaidAmount
		}
	}

	payment, err := h.paymentService.Approve(c.Request.Context(), uint(id), amount, interestAmount, paidAmount,
		h.getUserID(c),
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment": payment.ToResponse(), "message": "Pago aprobado"})
}

// RejectPaymentRequest is the request body for rejecting a payment
type RejectPaymentRequest struct {
	Reason string `json:"reason"`
}

// @Summary Reject Payment
// @Description Reject a payment (Admin). Optionally include a reason in the request body; the applicant receives it in the notification and email.
// @Tags Payments
// @Accept json
// @Produce json
// @Param payment_id path int true "Payment ID"
// @Param body body RejectPaymentRequest false "Rejection reason (optional)"
// @Success 200 {object} models.PaymentResponse
// @Security BearerAuth
// @Router /payments/{payment_id}/reject [post]
func (h *PaymentHandler) Reject(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("payment_id"), 10, 32)
	var req RejectPaymentRequest
	c.ShouldBindJSON(&req)

	payment, err := h.paymentService.Reject(c.Request.Context(), uint(id),
		h.getUserID(c),
		req.Reason,
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment": payment.ToResponse(), "message": "Pago rechazado"})
}

// @Summary Upload Receipt
// @Description Upload payment receipt image/pdf
// @Tags Payments
// @Accept multipart/form-data
// @Produce json
// @Param payment_id path int true "Payment ID"
// @Param receipt formData file true "Receipt File"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /payments/{payment_id}/receipt [post]
func (h *PaymentHandler) UploadReceipt(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("payment_id"), 10, 32)

	payment, err := h.paymentService.FindByID(c.Request.Context(), uint(id))
	if err != nil || payment.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pago no encontrado"})
		return
	}

	currentUserID := h.getUserID(c)
	currentUserRole := ""
	if role, exists := c.Get("userRole"); exists {
		currentUserRole = role.(string)
	}
	canUpload := currentUserRole == "admin" || currentUserRole == "seller" ||
		(payment.Contract.ID != 0 && payment.Contract.ApplicantUserID == currentUserID)
	if !canUpload {
		c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para subir comprobante a este pago"})
		return
	}

	file, header, err := c.Request.FormFile("receipt")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Archivo requerido"})
		return
	}
	defer file.Close()

	if c.Request.ContentLength > 0 && c.Request.ContentLength > storage.MaxFileSize() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Archivo demasiado grande"})
		return
	}

	if !storage.IsValidContentType(header.Header.Get("Content-Type")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipo de archivo inválido"})
		return
	}

	path, err := h.storage.Upload(file, header, "receipts")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar archivo"})
		return
	}

	if err := h.paymentService.UpdateReceiptPath(c.Request.Context(), uint(id), path); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comprobante subido exitosamente"})
}

// @Summary Download Receipt
// @Description Download payment receipt
// @Tags Payments
// @Produce application/octet-stream
// @Param payment_id path int true "Payment ID"
// @Success 200 {file} file "receipt"
// @Security BearerAuth
// @Router /payments/{payment_id}/receipt [get]
func (h *PaymentHandler) DownloadReceipt(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("payment_id"), 10, 32)
	payment, err := h.paymentService.FindByID(c.Request.Context(), uint(id))
	if err != nil || payment.DocumentPath == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comprobante no encontrado"})
		return
	}

	// Authorization check
	currentUserID := h.getUserID(c)
	currentUserRole := ""
	if role, exists := c.Get("userRole"); exists {
		currentUserRole = role.(string)
	}

	fullPath, err := h.storage.SafeFullPath(*payment.DocumentPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comprobante no encontrado"})
		return
	}

	// Allow if admin or seller
	if currentUserRole == "admin" || currentUserRole == "seller" {
		c.File(fullPath)
		return
	}

	// Allow if applicant
	if currentUserID == payment.Contract.ApplicantUserID {
		c.File(fullPath)
		return
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "No tienes permiso para ver este comprobante"})
}

// @Summary Undo Payment
// @Description Revert payment state (Admin)
// @Tags Payments
// @Accept json
// @Produce json
// @Param payment_id path int true "Payment ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /payments/{payment_id}/undo [post]
func (h *PaymentHandler) Undo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("payment_id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de pago inválido"})
		return
	}
	if err := h.paymentService.UndoPayment(c.Request.Context(), uint(id)); err != nil {
		if strings.Contains(err.Error(), "no encontrado") || strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Pago no encontrado"})
			return
		}
		if strings.Contains(err.Error(), "cannot undo") || strings.Contains(err.Error(), "cannot be undone") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pago deshecho"})
}

// Nested routes handlers

// @Summary List Contract Payments
// @Description Get payments for a contract
// @Tags Payments
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/payments [get]
func (h *PaymentHandler) IndexByContract(c *gin.Context) {
	contractID, _ := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	payments, err := h.paymentService.FindByContract(c.Request.Context(), uint(contractID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, p := range payments {
		responses = append(responses, p.ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{"payments": responses})
}

// @Summary Get Contract Payment
// @Description Get a payment for a specific contract
// @Tags Payments
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param payment_id path int true "Payment ID"
// @Success 200 {object} models.PaymentResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/payments/{payment_id} [get]
func (h *PaymentHandler) ShowByContract(c *gin.Context) {
	h.Show(c)
}

// @Summary Approve Contract Payment
// @Description Approve a payment for a specific contract
// @Tags Payments
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param payment_id path int true "Payment ID"
// @Param request body ApprovePaymentRequest true "Amount"
// @Success 200 {object} models.PaymentResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/payments/{payment_id}/approve [post]
func (h *PaymentHandler) ApproveByContract(c *gin.Context) {
	h.Approve(c)
}

// @Summary Reject Contract Payment
// @Description Reject a payment for a specific contract
// @Tags Payments
// @Accept json
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param payment_id path int true "Payment ID"
// @Success 200 {object} models.PaymentResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/payments/{payment_id}/reject [post]
func (h *PaymentHandler) RejectByContract(c *gin.Context) {
	h.Reject(c)
}

// @Summary Upload Contract Payment Receipt
// @Description Upload receipt for a contract payment
// @Tags Payments
// @Accept multipart/form-data
// @Produce json
// @Param contract_id path int true "Contract ID"
// @Param project_id path int true "Project ID"
// @Param lot_id path int true "Lot ID"
// @Param payment_id path int true "Payment ID"
// @Param receipt formData file true "Receipt File"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id}/contracts/{contract_id}/payments/{payment_id}/receipt [post]
func (h *PaymentHandler) UploadReceiptByContract(c *gin.Context) {
	h.UploadReceipt(c)
}

// Helper to get user ID from context
func (h *PaymentHandler) getUserID(c *gin.Context) uint {
	id, exists := c.Get("userID")
	if !exists {
		return 0
	}
	switch v := id.(type) {
	case uint:
		return v
	case float64:
		return uint(v)
	default:
		return 0
	}
}

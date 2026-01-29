package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
)

type ProjectHandler struct {
	projectService *services.ProjectService
}

func NewProjectHandler(projectService *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

func (h *ProjectHandler) Index(c *gin.Context) {
	// @Summary List Projects
	// @Description Get a paginated list of projects
	// @Tags Projects
	// @Accept json
	// @Produce json
	// @Param page query int false "Page number" default(1)
	// @Param per_page query int false "Items per page" default(20)
	// @Param search_term query string false "Search term"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /projects [get]
	query := repository.NewListQuery()
	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	query.Search = c.Query("search_term")

	projects, total, err := h.projectService.List(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, p := range projects {
		responses = append(responses, p.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{"projects": responses, "pagination": gin.H{"total": total}})
}

// @Summary Get Project
// @Description Get a project by ID
// @Tags Projects
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Success 200 {object} models.ProjectResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id} [get]
func (h *ProjectHandler) Show(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	project, err := h.projectService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proyecto no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": project.ToResponse()})
}

// @Summary Create Project
// @Description Create a new project
// @Tags Projects
// @Accept json
// @Produce json
// @Param request body models.Project true "Project Data"
// @Success 201 {object} models.ProjectResponse
// @Security BearerAuth
// @Router /projects [post]
func (h *ProjectHandler) Create(c *gin.Context) {
	var project models.Project
	if err := c.ShouldBindJSON(&project); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.projectService.Create(c.Request.Context(), &project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"project": project.ToResponse()})
}

// @Summary Update Project
// @Description Update an existing project
// @Tags Projects
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Param request body models.Project true "Project Data"
// @Success 200 {object} models.ProjectResponse
// @Security BearerAuth
// @Router /projects/{project_id} [put]
func (h *ProjectHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	var project models.Project
	if err := c.ShouldBindJSON(&project); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	project.ID = uint(id)

	if err := h.projectService.Update(c.Request.Context(), &project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"project": project.ToResponse()})
}

// @Summary Delete Project
// @Description Soft delete a project
// @Tags Projects
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id} [delete]
func (h *ProjectHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	if err := h.projectService.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Proyecto eliminado"})
}

// @Summary Approve Project
// @Description Approve a project (Admin)
// @Tags Projects
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/approve [post]
func (h *ProjectHandler) Approve(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	// For now, just a placeholder update status
	// Ideally service has Approve method
	_, err := h.projectService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}
	// project.Status = "approved"
	// h.projectService.Update(c, project)

	c.JSON(http.StatusOK, gin.H{"message": "Proyecto aprobado (Placeholder)"})
}

type ImportProjectRequest struct {
	Projects []models.Project `json:"projects"`
}

// @Summary Import Projects
// @Description Bulk import projects
// @Tags Projects
// @Accept json
// @Produce json
// @Param request body ImportProjectRequest true "Projects Data"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/import [post]
func (h *ProjectHandler) Import(c *gin.Context) {
	var req ImportProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, p := range req.Projects {
		if err := h.projectService.Create(c.Request.Context(), &p); err != nil {
			// Continue on error for now or break?
			// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			// return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Importados %d proyectos", len(req.Projects))})
}

type LotHandler struct {
	lotService *services.LotService
}

func NewLotHandler(lotService *services.LotService) *LotHandler {
	return &LotHandler{lotService: lotService}
}

// @Summary List Lots
// @Description Get a paginated list of lots for a project
// @Tags Lots
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param search_term query string false "Search term"
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /projects/{project_id}/lots [get]
func (h *LotHandler) Index(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	query := repository.NewListQuery()
	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	query.Search = c.Query("search_term")
	query.Filters["status"] = c.Query("status")

	lots, total, err := h.lotService.List(c.Request.Context(), uint(projectID), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, l := range lots {
		responses = append(responses, l.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{"lots": responses, "pagination": gin.H{"total": total}})
}

// @Summary Get Lot
// @Description Get a lot by ID
// @Tags Lots
// @Accept json
// @Produce json
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} models.LotResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id} [get]
func (h *LotHandler) Show(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("lot_id"), 10, 32)
	lot, err := h.lotService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lote no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"lot": lot.ToResponse()})
}

// @Summary Create Lot
// @Description Create a new lot in a project
// @Tags Lots
// @Accept json
// @Produce json
// @Param project_id path int true "Project ID"
// @Param request body models.Lot true "Lot Data"
// @Success 201 {object} models.LotResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots [post]
func (h *LotHandler) Create(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("project_id"), 10, 32)
	var lot models.Lot
	if err := c.ShouldBindJSON(&lot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lot.ProjectID = uint(projectID)

	if err := h.lotService.Create(c.Request.Context(), &lot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"lot": lot.ToResponse()})
}

// @Summary Update Lot
// @Description Update an existing lot
// @Tags Lots
// @Accept json
// @Produce json
// @Param lot_id path int true "Lot ID"
// @Param request body models.Lot true "Lot Data"
// @Success 200 {object} models.LotResponse
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id} [put]
func (h *LotHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("lot_id"), 10, 32)
	var lot models.Lot
	if err := c.ShouldBindJSON(&lot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lot.ID = uint(id)

	if err := h.lotService.Update(c.Request.Context(), &lot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"lot": lot.ToResponse()})
}

// @Summary Delete Lot
// @Description Soft delete a lot
// @Tags Lots
// @Accept json
// @Produce json
// @Param lot_id path int true "Lot ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /projects/{project_id}/lots/{lot_id} [delete]
func (h *LotHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("lot_id"), 10, 32)
	if err := h.lotService.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lote eliminado"})
}

type NotificationHandler struct {
	notificationService *services.NotificationService
}

func NewNotificationHandler(notificationService *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

// @Summary List Notifications
// @Description Get a paginated list of notifications for the current user
// @Tags Notifications
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /notifications [get]
func (h *NotificationHandler) Index(c *gin.Context) {
	userID := middleware.GetUserID(c)
	query := repository.NewListQuery()
	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))

	notifications, total, err := h.notificationService.FindByUser(c.Request.Context(), userID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []interface{}
	for _, n := range notifications {
		responses = append(responses, n.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{"notifications": responses, "pagination": gin.H{"total": total}})
}

// @Summary Get Notification
// @Description Get a notification by ID
// @Tags Notifications
// @Accept json
// @Produce json
// @Param notification_id path int true "Notification ID"
// @Success 200 {object} models.NotificationResponse
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /notifications/{notification_id} [get]
func (h *NotificationHandler) Show(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("notification_id"), 10, 32)
	notification, err := h.notificationService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notificación no encontrada"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"notification": notification.ToResponse()})
}

// @Summary Mark Notification Read
// @Description Mark a notification as read
// @Tags Notifications
// @Accept json
// @Produce json
// @Param notification_id path int true "Notification ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /notifications/{notification_id} [put]
func (h *NotificationHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("notification_id"), 10, 32)
	if err := h.notificationService.MarkAsRead(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Notificación marcada como leída"})
}

// @Summary Delete Notification
// @Description Delete a notification
// @Tags Notifications
// @Accept json
// @Produce json
// @Param notification_id path int true "Notification ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /notifications/{notification_id} [delete]
func (h *NotificationHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("notification_id"), 10, 32)
	if err := h.notificationService.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Notificación eliminada"})
}

// @Summary Mark All Notifications Read
// @Description Mark all notifications as read for current user
// @Tags Notifications
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /notifications/mark_all_as_read [post]
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if err := h.notificationService.MarkAllAsRead(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Todas las notificaciones marcadas como leídas"})
}

type ReportHandler struct {
	reportService *services.ReportService
}

func NewReportHandler(reportService *services.ReportService) *ReportHandler {
	return &ReportHandler{reportService: reportService}
}

// @Summary Commissions Report
// @Description Download commissions report as CSV
// @Tags Reports
// @Produce text/csv
// @Success 200 {file} file "commissions.csv"
// @Security BearerAuth
// @Router /reports/commissions_csv [get]
// @Router /reports/commissions_csv [get]
func (h *ReportHandler) CommissionsCSV(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	buf, err := h.reportService.GenerateCommissionsCSV(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=commissions.csv")
	c.String(http.StatusOK, buf.String())
}

// @Summary Revenue Report
// @Description Download total revenue report as CSV
// @Tags Reports
// @Produce text/csv
// @Success 200 {file} file "revenue.csv"
// @Security BearerAuth
// @Router /reports/total_revenue_csv [get]
func (h *ReportHandler) TotalRevenueCSV(c *gin.Context) {
	buf, err := h.reportService.GenerateRevenueCSV(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=revenue.csv")
	c.String(http.StatusOK, buf.String())
}

// @Summary Overdue Payments Report
// @Description Download overdue payments report as CSV
// @Tags Reports
// @Produce text/csv
// @Success 200 {file} file "overdue.csv"
// @Security BearerAuth
// @Router /reports/overdue_payments_csv [get]
func (h *ReportHandler) OverduePaymentsCSV(c *gin.Context) {
	buf, err := h.reportService.GenerateOverduePaymentsCSV(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=overdue.csv")
	c.String(http.StatusOK, buf.String())
}

// @Summary User Balance PDF
// @Description Download user balance statement as PDF
// @Tags Reports
// @Produce application/pdf
// @Param user_id query int true "User ID"
// @Success 200 {file} file "balance.pdf"
// @Security BearerAuth
// @Router /reports/user_balance_pdf [get]
func (h *ReportHandler) UserBalancePDF(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	buf, err := h.reportService.GenerateUserBalancePDF(c.Request.Context(), uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=balance_%d.pdf", userID))
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

// @Summary Contract Promise PDF
// @Description Download contract promise PDF
// @Tags Reports
// @Produce application/pdf
// @Param contract_id query int true "Contract ID"
// @Success 200 {file} file "contract.pdf"
// @Security BearerAuth
// @Router /reports/user_promise_contract_pdf [get]
func (h *ReportHandler) UserPromiseContractPDF(c *gin.Context) {
	contractID, _ := strconv.ParseUint(c.Query("contract_id"), 10, 32)
	if contractID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id is required"})
		return
	}

	buf, err := h.reportService.GenerateContractPDF(c.Request.Context(), uint(contractID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=contract_%d.pdf", contractID))
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

// @Summary Contract Rescission PDF
// @Description Download contract rescission PDF
// @Tags Reports
// @Produce application/pdf
// @Param contract_id query int true "Contract ID"
// @Success 200 {file} file "rescission.pdf"
// @Security BearerAuth
// @Router /reports/user_rescission_contract_pdf [get]
func (h *ReportHandler) UserRescissionContractPDF(c *gin.Context) {
	contractID, _ := strconv.ParseUint(c.Query("contract_id"), 10, 32)
	if contractID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id is required"})
		return
	}

	refundAmount, _ := strconv.ParseFloat(c.Query("refund_amount"), 64)
	penaltyAmount, _ := strconv.ParseFloat(c.Query("penalty_amount"), 64)

	buf, err := h.reportService.GenerateRescissionContractPDF(c.Request.Context(), uint(contractID), refundAmount, penaltyAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=rescission_%d.pdf", contractID))
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

// @Summary User Information PDF
// @Description Download user information sheet as PDF
// @Tags Reports
// @Produce application/pdf
// @Param user_id query int true "User ID"
// @Success 200 {file} file "info.pdf"
// @Security BearerAuth
// @Router /reports/user_information_pdf [get]
func (h *ReportHandler) UserInformationPDF(c *gin.Context) {
	// Reusing User Balance PDF for MVP
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	buf, err := h.reportService.GenerateUserBalancePDF(c.Request.Context(), uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=info_%d.pdf", userID))
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

// @Summary Customer Record PDF
// @Description Download customer record PDF (Hoja de Cliente)
// @Tags Reports
// @Produce application/pdf
// @Param contract_id query int true "Contract ID"
// @Success 200 {file} file "customer_record.pdf"
// @Security BearerAuth
// @Router /reports/customer_record_pdf [get]
func (h *ReportHandler) CustomerRecordPDF(c *gin.Context) {
	contractID, _ := strconv.ParseUint(c.Query("contract_id"), 10, 32)
	if contractID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id is required"})
		return
	}

	buf, err := h.reportService.GenerateCustomerRecordPDF(c.Request.Context(), uint(contractID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=customer_record_%d.pdf", contractID))
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

type AuditHandler struct {
	auditService *services.AuditService
}

func NewAuditHandler(auditService *services.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// @Summary List Audit Logs
// @Description Get a paginated list of system audit logs
// @Tags Audit
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(50)
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /audits [get]
func (h *AuditHandler) Index(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	offset := (page - 1) * perPage

	logs, total, err := h.auditService.List(c.Request.Context(), perPage, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"audits": logs, "pagination": gin.H{"total": total, "page": page, "per_page": perPage}})
}

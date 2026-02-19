package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// ContractRepository defines the interface for contract data access
type ContractRepository interface {
	FindByID(ctx context.Context, id uint) (*models.Contract, error)
	FindByIDWithDetails(ctx context.Context, id uint) (*models.Contract, error)
	FindByLot(ctx context.Context, lotID uint) ([]models.Contract, error)
	FindByUser(ctx context.Context, userID uint) ([]models.Contract, error)
	Create(ctx context.Context, contract *models.Contract) error
	Update(ctx context.Context, contract *models.Contract) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, query *ContractQuery) ([]models.Contract, int64, error)
	FindActiveByLot(ctx context.Context, lotID uint) (*models.Contract, error)
	FindPendingReservations(ctx context.Context, olderThan int) ([]models.Contract, error)
	GetStats(ctx context.Context) (*ContractStats, error)
	HasActiveContracts(ctx context.Context, userID uint) (bool, error)
}

// ContractQuery extends ListQuery with contract-specific filters
type ContractQuery struct {
	*ListQuery
	UserID    uint
	IsAdmin   bool
	Status    string
	LotID     uint
	ProjectID uint
}

type contractRepository struct {
	db *gorm.DB
}

// NewContractRepository creates a new contract repository
func NewContractRepository(db *gorm.DB) ContractRepository {
	return &contractRepository{db: db}
}

func (r *contractRepository) FindByID(ctx context.Context, id uint) (*models.Contract, error) {
	var contract models.Contract
	err := r.db.WithContext(ctx).First(&contract, id).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (r *contractRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.Contract, error) {
	var contract models.Contract
	// Load contract + Lot, Project, ApplicantUser, Creator in one query via Joins (avoids 4 separate Preload round-trips).
	// Payments and LedgerEntries are one-to-many so we keep 2 Preloads (3 queries total instead of 6).
	err := r.db.WithContext(ctx).
		Joins("Lot").
		Joins("Lot.Project").
		Joins("ApplicantUser").
		Joins("Creator").
		Preload("Payments", func(db *gorm.DB) *gorm.DB {
			return db.Order("due_date ASC")
		}).
		Preload("LedgerEntries", func(db *gorm.DB) *gorm.DB {
			return db.Order("entry_date ASC")
		}).
		First(&contract, id).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (r *contractRepository) FindByLot(ctx context.Context, lotID uint) ([]models.Contract, error) {
	var contracts []models.Contract
	err := r.db.WithContext(ctx).
		Where("lot_id = ?", lotID).
		Preload("ApplicantUser").
		Find(&contracts).Error
	return contracts, err
}

func (r *contractRepository) FindByUser(ctx context.Context, userID uint) ([]models.Contract, error) {
	var contracts []models.Contract
	err := r.db.WithContext(ctx).
		Where("applicant_user_id = ?", userID).
		Preload("Lot.Project").
		Preload("Payments").
		Find(&contracts).Error
	return contracts, err
}

func (r *contractRepository) Create(ctx context.Context, contract *models.Contract) error {
	return r.db.WithContext(ctx).Create(contract).Error
}

func (r *contractRepository) Update(ctx context.Context, contract *models.Contract) error {
	return r.db.WithContext(ctx).Save(contract).Error
}

func (r *contractRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Contract{}, id).Error
}

func (r *contractRepository) List(ctx context.Context, query *ContractQuery) ([]models.Contract, int64, error) {
	var contracts []models.Contract
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Contract{})

	// Filter by creator if not admin
	if !query.IsAdmin && query.UserID > 0 {
		db = db.Where("creator_id = ?", query.UserID)
	}

	// Apply status filter (single or multiple via status_in)
	if query.Filters != nil {
		if val, ok := query.Filters["status_in"]; ok && val != "" {
			statuses := strings.Split(val, ",")
			for i, s := range statuses {
				statuses[i] = strings.TrimSpace(s)
			}
			if len(statuses) > 0 {
				db = db.Where("contracts.status IN ?", statuses)
			}
		}
	}
	if query.Filters == nil || query.Filters["status_in"] == "" {
		if query.Status != "" {
			db = db.Where("contracts.status = ?", query.Status)
		}
	}

	// Apply lot filter
	if query.LotID > 0 {
		db = db.Where("contracts.lot_id = ?", query.LotID)
	}

	// Apply approved_at date filters
	if query.Filters != nil {
		if val, ok := query.Filters["approved_from"]; ok && val != "" {
			db = db.Where("contracts.approved_at >= ?", val)
		}
		if val, ok := query.Filters["approved_to"]; ok && val != "" {
			// Ensure we include the full day if only date is provided
			if len(val) == 10 { // YYYY-MM-DD
				val += " 23:59:59"
			}
			db = db.Where("contracts.approved_at <= ?", val)
		}
		// Apply created_at date filters (General date filter)
		if val, ok := query.Filters["start_date"]; ok && val != "" {
			db = db.Where("contracts.created_at >= ?", val)
		}
		if val, ok := query.Filters["end_date"]; ok && val != "" {
			// Ensure we include the full day if only date is provided
			if len(val) == 10 { // YYYY-MM-DD
				val += " 23:59:59"
			}
			db = db.Where("contracts.created_at <= ?", val)
		}
	}

	if val, ok := query.Filters["guid"]; ok && val != "" {
		db = db.Where("contracts.guid = ?", val)
	}

	// Apply search (JOINs only for filtering; associations loaded via Preload below)
	if query.Search != "" {
		search := "%" + query.Search + "%"
		db = db.Joins("LEFT JOIN users ON users.id = contracts.applicant_user_id").
			Joins("LEFT JOIN lots ON lots.id = contracts.lot_id").
			Joins("LEFT JOIN projects ON projects.id = lots.project_id").
			Where("users.full_name ILIKE ? OR users.email ILIKE ? OR lots.name ILIKE ? OR projects.name ILIKE ? OR contracts.guid ILIKE ?",
				search, search, search, search, search)
	}

	// Count total using a separate session so the main query is not altered by Count()
	countDB := db.Session(&gorm.Session{})
	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting
	if query.SortBy != "" {
		order := query.SortBy
		if query.SortDir == "desc" {
			order += " DESC"
		}
		db = db.Order(order)
	} else {
		db = db.Order("contracts.created_at DESC")
	}

	// Apply pagination
	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	// Load associations (Lot.Project, ApplicantUser, Creator)
	err := db.
		Preload("Lot.Project").
		Preload("ApplicantUser").
		Preload("Creator").
		Find(&contracts).Error

	if err != nil {
		return nil, 0, err
	}

	// Calculate TotalPaid for each contract using aggregation
	if len(contracts) > 0 {
		var contractIDs []uint
		for _, c := range contracts {
			contractIDs = append(contractIDs, c.ID)
		}

		type Result struct {
			ContractID uint
			Total      float64
		}
		var results []Result

		// Sum paid_amount for payments with status 'paid'
		// Note: We use paid_amount as it's the actual transaction amount
		if err := r.db.Model(&models.Payment{}).
			Select("contract_id, COALESCE(SUM(paid_amount), 0) as total").
			Where("contract_id IN ? AND status = ?", contractIDs, models.PaymentStatusPaid).
			Group("contract_id").
			Scan(&results).Error; err == nil {

			// Map results to contracts
			resultMap := make(map[uint]float64)
			for _, res := range results {
				resultMap[res.ContractID] = res.Total
			}

			for i := range contracts {
				if val, ok := resultMap[contracts[i].ID]; ok {
					contracts[i].TotalPaid = val
				}
			}
		}
	}

	return contracts, total, err
}

// ContractStats holds the count of contracts by status
type ContractStats struct {
	Total    int64 `json:"total"`
	Pending  int64 `json:"pending"`
	Approved int64 `json:"approved"`
	Rejected int64 `json:"rejected"`
}

func (r *contractRepository) GetStats(ctx context.Context) (*ContractStats, error) {
	stats := &ContractStats{}

	// Execute a single query to get counts by status
	rows, err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Select("status, count(*) as count").
		Group("status").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var total int64
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		total += count
		switch status {
		case models.ContractStatusPending:
			stats.Pending = count
		case models.ContractStatusApproved:
			stats.Approved = count
		case models.ContractStatusRejected:
			stats.Rejected = count
		}
	}
	stats.Total = total

	return stats, nil
}

func (r *contractRepository) FindActiveByLot(ctx context.Context, lotID uint) (*models.Contract, error) {
	var contract models.Contract
	err := r.db.WithContext(ctx).
		Where("lot_id = ? AND active = ?", lotID, true).
		First(&contract).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (r *contractRepository) FindPendingReservations(ctx context.Context, olderThanHours int) ([]models.Contract, error) {
	var contracts []models.Contract
	interval := fmt.Sprintf("%d hours", olderThanHours)
	err := r.db.WithContext(ctx).
		Where("contracts.status = ? AND contracts.created_at < NOW() - INTERVAL '"+interval+"'", models.ContractStatusSubmitted).
		Preload("Lot").
		Preload("ApplicantUser").
		Find(&contracts).Error
	return contracts, err
}

func (r *contractRepository) HasActiveContracts(ctx context.Context, userID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Contract{}).
		Where("applicant_user_id = ?", userID).
		Where("active = ? OR status IN ?", true, []string{models.ContractStatusApproved, models.ContractStatusSigned}).
		Count(&count).Error
	return count > 0, err
}

// PaymentStats holds monthly payment statistics
type PaymentStats struct {
	PendingThisMonth   float64 `json:"pending_this_month"`
	CollectedThisMonth float64 `json:"collected_this_month"`
	TotalOverdue       float64 `json:"total_overdue"`
}

// PaymentRepository defines the interface for payment data access
type PaymentRepository interface {
	FindByID(ctx context.Context, id uint) (*models.Payment, error)
	FindByContract(ctx context.Context, contractID uint) ([]models.Payment, error)
	Create(ctx context.Context, payment *models.Payment) error
	Update(ctx context.Context, payment *models.Payment) error
	Delete(ctx context.Context, id uint) error
	DeleteByContract(ctx context.Context, contractID uint) error
	List(ctx context.Context, query *ListQuery) ([]models.Payment, int64, error)
	FindOverdue(ctx context.Context) ([]models.Payment, error)
	FindOverdueForActiveContracts(ctx context.Context) ([]models.Payment, error)
	FindPaymentsDueTomorrowForActiveContracts(ctx context.Context) ([]models.Payment, error)
	MarkOverdueReminderSent(ctx context.Context, paymentIDs []uint) error
	MarkUpcomingReminderSent(ctx context.Context, paymentIDs []uint) error
	FindPendingByUser(ctx context.Context, userID uint) ([]models.Payment, error)
	FindPaidByMonth(ctx context.Context, month, year int) ([]models.Payment, error)
	GetMonthlyStats(ctx context.Context) (*PaymentStats, error)
	FindByUserID(ctx context.Context, userID uint) ([]models.Payment, error)
	BatchUpdateInterest(ctx context.Context, updates map[uint]float64) error
}

type paymentRepository struct {
	db *gorm.DB
}

// NewPaymentRepository creates a new payment repository
func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) FindByID(ctx context.Context, id uint) (*models.Payment, error) {
	var payment models.Payment
	err := r.db.WithContext(ctx).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("ApprovedByUser").
		First(&payment, id).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *paymentRepository) FindByContract(ctx context.Context, contractID uint) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("ApprovedByUser").
		Where("contract_id = ?", contractID).
		Order("due_date ASC").
		Find(&payments).Error
	return payments, err
}

func (r *paymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	return r.db.WithContext(ctx).Create(payment).Error
}

func (r *paymentRepository) Update(ctx context.Context, payment *models.Payment) error {
	return r.db.WithContext(ctx).Save(payment).Error
}

func (r *paymentRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Payment{}, id).Error
}

func (r *paymentRepository) DeleteByContract(ctx context.Context, contractID uint) error {
	return r.db.WithContext(ctx).Where("contract_id = ?", contractID).Delete(&models.Payment{}).Error
}

func (r *paymentRepository) List(ctx context.Context, query *ListQuery) ([]models.Payment, int64, error) {
	var payments []models.Payment
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Payment{})

	// Apply status filter
	// Apply status filter
	statusFilter := query.Filters["status"]
	if statusFilter != "" {
		if strings.HasPrefix(statusFilter, "[") && strings.HasSuffix(statusFilter, "]") {
			// Handle format [paid|submitted]
			inner := statusFilter[1 : len(statusFilter)-1]
			statuses := strings.Split(inner, "|")
			db = db.Where("payments.status IN ?", statuses)
		} else if strings.Contains(statusFilter, ",") {
			// Handle comma-separated list
			statuses := strings.Split(statusFilter, ",")
			db = db.Where("payments.status IN ?", statuses)
		} else if statusFilter == "overdue" {
			// Handle virtual "overdue" status
			db = db.Where("payments.status = ? AND payments.due_date < CURRENT_DATE", models.PaymentStatusPending)
		} else {
			db = db.Where("payments.status = ?", statusFilter)
		}
	}

	// Apply date filters
	if val, ok := query.Filters["start_date"]; ok && val != "" {
		db = db.Where("payments.payment_date >= ? OR (payments.payment_date IS NULL AND payments.due_date >= ?)", val, val)
	}
	if val, ok := query.Filters["end_date"]; ok && val != "" {
		endDate := val
		if len(endDate) == 10 {
			endDate += " 23:59:59"
		}
		db = db.Where("payments.payment_date <= ? OR (payments.payment_date IS NULL AND payments.due_date <= ?)", endDate, endDate)
	}

	// Apply search filter if provided (case-insensitive across multiple fields)
	if search := query.Filters["search_term"]; search != "" {
		term := "%" + search + "%"
		db = db.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN users ON users.id = contracts.applicant_user_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Joins("JOIN projects ON projects.id = lots.project_id").
			Where("(users.full_name ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ? OR users.identity ILIKE ? OR "+
				"COALESCE(payments.description, '') ILIKE ? OR lots.name ILIKE ? OR COALESCE(lots.address, '') ILIKE ? OR "+
				"projects.name ILIKE ? OR projects.address ILIKE ?)", term, term, term, term, term, term, term, term, term)
	}

	// Clone the database session for count to avoid affecting the main query
	countDb := db.Session(&gorm.Session{})
	if err := countDb.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting: always show "submitted" (enviados) first, then the rest
	submittedFirst := "(CASE WHEN payments.status = '" + models.PaymentStatusSubmitted + "' THEN 0 ELSE 1 END) ASC"
	db = db.Order(submittedFirst)
	if query.SortBy != "" {
		field := query.SortBy
		// Map frontend fields to database columns if necessary
		switch field {
		case "updated_at", "created_at", "due_date", "payment_date":
			field = "payments." + field
		case "applicant":
			// Sort by the applicant's full name via contract → user join
			db = db.Joins("LEFT JOIN contracts AS sort_c ON sort_c.id = payments.contract_id").
				Joins("LEFT JOIN users AS sort_u ON sort_u.id = sort_c.applicant_user_id")
			field = "sort_u.full_name"
		}

		order := field
		if strings.ToLower(query.SortDir) == "desc" {
			order += " DESC"
		} else {
			order += " ASC"
		}
		db = db.Order(order)
	} else {
		db = db.Order("payments.due_date ASC")
	}

	// Apply pagination
	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	err := db.
		Select("payments.*"). // Ensure we select only payment fields, especially when joining
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("ApprovedByUser").
		Find(&payments).Error

	return payments, total, err
}

func (r *paymentRepository) FindOverdue(ctx context.Context) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).
		Where("payments.status = ? AND payments.due_date < CURRENT_DATE", models.PaymentStatusPending).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Order("due_date ASC").
		Find(&payments).Error
	return payments, err
}

// FindOverdueForActiveContracts returns overdue payments only for active contracts (approved, active=true)
// and active users (status=active, not discarded). Excludes payments that had a reminder sent in the last 7 days
// to avoid spamming. Preloads Contract.Lot and Contract.ApplicantUser for email templates.
func (r *paymentRepository) FindOverdueForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).
		Joins("JOIN contracts ON contracts.id = payments.contract_id AND contracts.status = ? AND contracts.active = ?",
			models.ContractStatusApproved, true).
		Joins("JOIN users ON users.id = contracts.applicant_user_id AND users.status = ? AND users.discarded_at IS NULL",
			models.StatusActive).
		Where("payments.status = ? AND payments.due_date < CURRENT_DATE", models.PaymentStatusPending).
		Where("(payments.overdue_reminder_sent_at IS NULL OR payments.overdue_reminder_sent_at < CURRENT_TIMESTAMP - INTERVAL '7 days')").
		Preload("Contract.Lot").
		Preload("Contract.ApplicantUser").
		Order("payments.due_date ASC").
		Find(&payments).Error
	return payments, err
}

// FindPaymentsDueTomorrowForActiveContracts returns pending payments with due_date = tomorrow,
// for active contracts and active users, that have not yet had an upcoming reminder sent.
func (r *paymentRepository) FindPaymentsDueTomorrowForActiveContracts(ctx context.Context) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).
		Joins("JOIN contracts ON contracts.id = payments.contract_id AND contracts.status = ? AND contracts.active = ?",
			models.ContractStatusApproved, true).
		Joins("JOIN users ON users.id = contracts.applicant_user_id AND users.status = ? AND users.discarded_at IS NULL",
			models.StatusActive).
		Where("payments.status = ? AND payments.due_date = CURRENT_DATE + INTERVAL '1 day'", models.PaymentStatusPending).
		Where("payments.upcoming_reminder_sent_at IS NULL").
		Preload("Contract.Lot").
		Preload("Contract.ApplicantUser").
		Order("payments.due_date ASC").
		Find(&payments).Error
	return payments, err
}

// MarkOverdueReminderSent sets overdue_reminder_sent_at to now for the given payment IDs.
func (r *paymentRepository) MarkOverdueReminderSent(ctx context.Context, paymentIDs []uint) error {
	if len(paymentIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.Payment{}).
		Where("id IN ?", paymentIDs).
		Update("overdue_reminder_sent_at", gorm.Expr("CURRENT_TIMESTAMP")).Error
}

// MarkUpcomingReminderSent sets upcoming_reminder_sent_at to now for the given payment IDs.
func (r *paymentRepository) MarkUpcomingReminderSent(ctx context.Context, paymentIDs []uint) error {
	if len(paymentIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.Payment{}).
		Where("id IN ?", paymentIDs).
		Update("upcoming_reminder_sent_at", gorm.Expr("CURRENT_TIMESTAMP")).Error
}

func (r *paymentRepository) FindPendingByUser(ctx context.Context, userID uint) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).
		Joins("JOIN contracts ON contracts.id = payments.contract_id").
		Where("contracts.applicant_user_id = ? AND payments.status IN ?", userID,
			[]string{models.PaymentStatusPending, models.PaymentStatusSubmitted}).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("ApprovedByUser").
		Order("payments.due_date ASC").
		Find(&payments).Error
	return payments, err
}

func (r *paymentRepository) FindPaidByMonth(ctx context.Context, month, year int) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("ApprovedByUser").
		Where("payments.status = ? AND EXTRACT(MONTH FROM payments.payment_date) = ? AND EXTRACT(YEAR FROM payments.payment_date) = ?",
			models.PaymentStatusPaid, month, year).
		Order("payment_date ASC").
		Find(&payments).Error
	return payments, err
}

func (r *paymentRepository) GetMonthlyStats(ctx context.Context) (*PaymentStats, error) {
	stats := &PaymentStats{}

	// Get current month start and end
	var pendingThisMonth, collectedThisMonth, totalOverdue float64

	// 1. Pending payments for current month
	err := r.db.WithContext(ctx).
		Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("payments.status IN ? AND EXTRACT(MONTH FROM due_date) = EXTRACT(MONTH FROM CURRENT_DATE) AND EXTRACT(YEAR FROM due_date) = EXTRACT(YEAR FROM CURRENT_DATE)",
			[]string{models.PaymentStatusPending, models.PaymentStatusSubmitted}).
		Scan(&pendingThisMonth).Error
	if err != nil {
		return nil, err
	}

	// 2. Collected payments for current month
	err = r.db.WithContext(ctx).
		Model(&models.Payment{}).
		Select("COALESCE(SUM(paid_amount), 0)").
		Where("payments.status = ? AND EXTRACT(MONTH FROM payment_date) = EXTRACT(MONTH FROM CURRENT_DATE) AND EXTRACT(YEAR FROM payment_date) = EXTRACT(YEAR FROM CURRENT_DATE)",
			models.PaymentStatusPaid).
		Scan(&collectedThisMonth).Error
	if err != nil {
		return nil, err
	}

	// 3. Total overdue payments
	err = r.db.WithContext(ctx).
		Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("payments.status = ? AND due_date < CURRENT_DATE", models.PaymentStatusPending).
		Scan(&totalOverdue).Error
	if err != nil {
		return nil, err
	}

	stats.PendingThisMonth = pendingThisMonth
	stats.CollectedThisMonth = collectedThisMonth
	stats.TotalOverdue = totalOverdue

	return stats, nil
}

func (r *paymentRepository) FindByUserID(ctx context.Context, userID uint) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).
		Joins("JOIN contracts ON contracts.id = payments.contract_id").
		Where("contracts.applicant_user_id = ?", userID).
		Preload("Contract.Lot.Project").
		Preload("Contract.ApplicantUser").
		Preload("ApprovedByUser").
		Order("payments.due_date ASC").
		Find(&payments).Error
	return payments, err
}

func (r *paymentRepository) BatchUpdateInterest(ctx context.Context, updates map[uint]float64) error {
	if len(updates) == 0 {
		return nil
	}

	// Use a transaction to perform updates
	// While not a single SQL statement, this is much better than separate transactions
	// as it amortizes transaction overhead. Given typical batch sizes (hundreds),
	// this is performant enough for Postgres.
	// A CASE statement would be faster but more complex to construct safely.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, amount := range updates {
			if err := tx.Model(&models.Payment{}).Where("id = ?", id).Update("interest_amount", amount).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ProjectRepository defines the interface for project data access
type ProjectRepository interface {
	FindByID(ctx context.Context, id uint) (*models.Project, error)
	Create(ctx context.Context, project *models.Project) error
	Update(ctx context.Context, project *models.Project) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, query *ListQuery) ([]models.Project, int64, error)
	FindAll(ctx context.Context) ([]models.Project, error)
}

type projectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) FindByID(ctx context.Context, id uint) (*models.Project, error) {
	var project models.Project
	err := r.db.WithContext(ctx).Preload("Lots").First(&project, id).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *projectRepository) Create(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *projectRepository) Update(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Save(project).Error
}

func (r *projectRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Project{}, id).Error
}

func (r *projectRepository) List(ctx context.Context, query *ListQuery) ([]models.Project, int64, error) {
	var projects []models.Project
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Project{})

	if query.Search != "" {
		search := "%" + query.Search + "%"
		db = db.Where("name ILIKE ? OR address ILIKE ? OR guid ILIKE ?", search, search, search)
	}

	if val, ok := query.Filters["guid"]; ok && val != "" {
		db = db.Where("guid = ?", val)
	}

	db.Count(&total)

	if query.SortBy != "" {
		order := query.SortBy
		if query.SortDir == "desc" {
			order += " DESC"
		}
		db = db.Order(order)
	} else {
		db = db.Order("created_at DESC")
	}

	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	err := db.Preload("Lots").Find(&projects).Error
	return projects, total, err
}

func (r *projectRepository) FindAll(ctx context.Context) ([]models.Project, error) {
	var projects []models.Project
	err := r.db.WithContext(ctx).Find(&projects).Error
	return projects, err
}

// LotRepository defines the interface for lot data access
type LotRepository interface {
	FindByID(ctx context.Context, id uint) (*models.Lot, error)
	FindByProject(ctx context.Context, projectID uint) ([]models.Lot, error)
	Create(ctx context.Context, lot *models.Lot) error
	Update(ctx context.Context, lot *models.Lot) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, projectID uint, query *ListQuery) ([]models.Lot, int64, error)
}

type lotRepository struct {
	db *gorm.DB
}

func NewLotRepository(db *gorm.DB) LotRepository {
	return &lotRepository{db: db}
}

func (r *lotRepository) FindByID(ctx context.Context, id uint) (*models.Lot, error) {
	var lot models.Lot
	err := r.db.WithContext(ctx).Preload("Project").First(&lot, id).Error
	if err != nil {
		return nil, err
	}
	return &lot, nil
}

func (r *lotRepository) FindByProject(ctx context.Context, projectID uint) ([]models.Lot, error) {
	var lots []models.Lot
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("name ASC").
		Find(&lots).Error
	return lots, err
}

func (r *lotRepository) Create(ctx context.Context, lot *models.Lot) error {
	return r.db.WithContext(ctx).Create(lot).Error
}

func (r *lotRepository) Update(ctx context.Context, lot *models.Lot) error {
	return r.db.WithContext(ctx).Save(lot).Error
}

func (r *lotRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Lot{}, id).Error
}

func (r *lotRepository) List(ctx context.Context, projectID uint, query *ListQuery) ([]models.Lot, int64, error) {
	var lots []models.Lot
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Lot{}).Where("project_id = ?", projectID)

	if query.Search != "" {
		search := "%" + query.Search + "%"
		db = db.Where("name ILIKE ? OR address ILIKE ?", search, search)
	}

	if query.Filters["status"] != "" {
		db = db.Where("lots.status = ?", query.Filters["status"])
	}

	db.Count(&total)

	if query.SortBy != "" {
		order := query.SortBy
		if query.SortDir == "desc" {
			order += " DESC"
		}
		db = db.Order(order)
	} else {
		db = db.Order("name ASC")
	}

	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	err := db.Preload("Project").
		Preload("Contracts", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Contracts.ApplicantUser").
		Preload("Contracts.Creator").
		Find(&lots).Error
	return lots, total, err
}

// NotificationRepository defines the interface for notification data access
type NotificationRepository interface {
	FindByID(ctx context.Context, id uint) (*models.Notification, error)
	FindByUser(ctx context.Context, userID uint, query *ListQuery) ([]models.Notification, int64, error)
	Create(ctx context.Context, notification *models.Notification) error
	Update(ctx context.Context, notification *models.Notification) error
	Delete(ctx context.Context, id uint) error
	MarkAllAsRead(ctx context.Context, userID uint) error
	CountUnread(ctx context.Context, userID uint) (int64, error)
}

type notificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) FindByID(ctx context.Context, id uint) (*models.Notification, error) {
	var notification models.Notification
	err := r.db.WithContext(ctx).First(&notification, id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (r *notificationRepository) FindByUser(ctx context.Context, userID uint, query *ListQuery) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Notification{}).Where("user_id = ?", userID)

	if status, ok := query.Filters["status"]; ok && status != "" {
		switch strings.ToLower(status) {
		case "unread":
			db = db.Where("read_at IS NULL")
		case "read":
			db = db.Where("read_at IS NOT NULL")
		}
	}

	db.Count(&total)
	db = db.Order("created_at DESC")

	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	err := db.Find(&notifications).Error
	return notifications, total, err
}

func (r *notificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

func (r *notificationRepository) Update(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Save(notification).Error
}

func (r *notificationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Notification{}, id).Error
}

func (r *notificationRepository) MarkAllAsRead(ctx context.Context, userID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", now).Error
}

func (r *notificationRepository) CountUnread(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

// RefreshTokenRepository defines the interface for refresh token data access
type RefreshTokenRepository interface {
	FindByToken(ctx context.Context, token string) (*models.RefreshToken, error)
	Create(ctx context.Context, rt *models.RefreshToken) error
	Delete(ctx context.Context, token string) error
	DeleteByUser(ctx context.Context, userID uint) error
}

type refreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

func (r *refreshTokenRepository) FindByToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&rt).Error
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *refreshTokenRepository) Create(ctx context.Context, rt *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(rt).Error
}

func (r *refreshTokenRepository) Delete(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Where("token = ?", token).Delete(&models.RefreshToken{}).Error
}

func (r *refreshTokenRepository) DeleteByUser(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.RefreshToken{}).Error
}

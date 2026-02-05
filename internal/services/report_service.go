package services

import (
	"bytes"
	"context"
	"embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

//go:embed templates/reports/*.html
var reportTemplates embed.FS

type CommissionReportItem struct {
	ContractID uint    `json:"contract_id"`
	ClientName string  `json:"client_name"`
	Project    string  `json:"project"`
	Lot        string  `json:"lot"`
	Amount     float64 `json:"amount"`
	Date       string  `json:"date"`
	Seller     string  `json:"seller"`
	Commission float64 `json:"commission"`
}

type ReportService struct {
	paymentRepo  repository.PaymentRepository
	contractRepo repository.ContractRepository
	userRepo     repository.UserRepository
}

func NewReportService(
	paymentRepo repository.PaymentRepository,
	contractRepo repository.ContractRepository,
	userRepo repository.UserRepository,
) *ReportService {
	return &ReportService{
		paymentRepo:  paymentRepo,
		contractRepo: contractRepo,
		userRepo:     userRepo,
	}
}

// GenerateCommissions returns a list of commission data. When userID > 0 and filterByCreator
// is true (e.g. for sellers), only contracts created by that user are returned.
func (s *ReportService) GenerateCommissions(ctx context.Context, startDate, endDate string, userID uint, filterByCreator bool) ([]CommissionReportItem, error) {
	listQuery := repository.NewListQuery()

	if startDate != "" && endDate != "" {
		listQuery.Filters["approved_from"] = startDate
		listQuery.Filters["approved_to"] = endDate
	}

	query := &repository.ContractQuery{
		ListQuery: listQuery,
		UserID:    userID,
		IsAdmin:   !filterByCreator,
		Status:    models.ContractStatusApproved,
	}

	contracts, _, err := s.contractRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}

	var filteredContracts []models.Contract
	if startDate != "" && endDate != "" {
		start, _ := time.Parse("2006-01-02", startDate)
		end, _ := time.Parse("2006-01-02", endDate)
		end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

		for _, c := range contracts {
			if c.ApprovedAt != nil && (c.ApprovedAt.After(start) || c.ApprovedAt.Equal(start)) && (c.ApprovedAt.Before(end) || c.ApprovedAt.Equal(end)) {
				filteredContracts = append(filteredContracts, c)
			}
		}
	} else {
		filteredContracts = contracts
	}

	var items []CommissionReportItem

	for _, c := range filteredContracts {
		clientName := "N/A"
		if c.ApplicantUser.ID != 0 {
			clientName = c.ApplicantUser.FullName
		}

		projectName := "N/A"
		lotNumber := "N/A"
		if c.Lot.ID != 0 {
			lotNumber = c.Lot.Name
			if c.Lot.Project.ID != 0 {
				projectName = c.Lot.Project.Name
			}
		}

		amount := 0.0
		if c.Amount != nil {
			amount = *c.Amount
		}

		dateStr := ""
		if c.ApprovedAt != nil {
			dateStr = c.ApprovedAt.Format("2006-01-02")
		}

		sellerName := "Unknown"
		if c.Creator != nil {
			sellerName = c.Creator.FullName
		}
		commission := amount * 0.02 // Example 2% commission logic

		items = append(items, CommissionReportItem{
			ContractID: c.ID,
			ClientName: clientName,
			Project:    projectName,
			Lot:        lotNumber,
			Amount:     amount,
			Date:       dateStr,
			Seller:     sellerName,
			Commission: commission,
		})
	}

	return items, nil
}

// GenerateCommissionsCSV generates a CSV report of sales and commissions
func (s *ReportService) GenerateCommissionsCSV(ctx context.Context, startDate, endDate string, userID uint, filterByCreator bool) (*bytes.Buffer, error) {
	items, err := s.GenerateCommissions(ctx, startDate, endDate, userID, filterByCreator)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	// Header
	header := []string{"Contrato ID", "Cliente", "Lote", "Proyecto", "Valor Contrato", "Fecha Venta", "Vendedor", "Comisión Est."}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, item := range items {
		record := []string{
			fmt.Sprintf("%d", item.ContractID),
			item.ClientName,
			item.Lot,
			item.Project,
			fmt.Sprintf("%.2f", item.Amount),
			item.Date,
			item.Seller,
			fmt.Sprintf("%.2f", item.Commission),
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRevenueCSV generates a CSV report of revenue
func (s *ReportService) GenerateRevenueCSV(ctx context.Context) (*bytes.Buffer, error) {
	// Get all payments that are PAID
	// Ideally we filter by date range, but for now dumping all paid payments
	query := repository.NewListQuery()
	query.Filters["status"] = "paid"

	payments, _, err := s.paymentRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	// Translation maps
	paymentTypeTranslations := map[string]string{
		models.PaymentTypeReservation: "Reservación",
		models.PaymentTypeDownPayment: "Prima",
		models.PaymentTypeInstallment: "Cuota",
		models.PaymentTypeFull:        "Pago Total",
		models.PaymentTypeAdvance:     "Abono a Capital",
	}

	financingTypeTranslations := map[string]string{
		models.FinancingTypeDirect: "Directo",
		models.FinancingTypeBank:   "Bancario",
		models.FinancingTypeCash:   "Contado",
	}

	// Header
	header := []string{
		"Pago ID", "Contrato", "Tipo", "Monto Pagado", "Fecha Pago",
		"Cliente", "Identidad", "Proyecto", "Lote",
		"Financiamiento", "Plazo",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, p := range payments {
		paidAmount := 0.0
		if p.PaidAmount != nil {
			paidAmount = *p.PaidAmount
		}

		payDate := ""
		if p.PaymentDate != nil {
			payDate = p.PaymentDate.Format("2006-01-02")
		}

		clientName := "N/A"
		clientIdentity := "N/A"
		projectName := "N/A"
		lotName := "N/A"
		financingType := "N/A"
		paymentTerm := "N/A"

		// Assuming Preload works in List
		if p.Contract.ID != 0 {
			if p.Contract.ApplicantUser.ID != 0 {
				clientName = p.Contract.ApplicantUser.FullName
				clientIdentity = p.Contract.ApplicantUser.Identity
			}

			if p.Contract.Lot.ID != 0 {
				lotName = p.Contract.Lot.Name
				if p.Contract.Lot.Project.ID != 0 {
					projectName = p.Contract.Lot.Project.Name
				}
			}

			if val, ok := financingTypeTranslations[p.Contract.FinancingType]; ok {
				financingType = val
			} else {
				financingType = p.Contract.FinancingType
			}

			paymentTerm = fmt.Sprintf("%d meses", p.Contract.PaymentTerm)
		}

		paymentType := p.PaymentType
		if val, ok := paymentTypeTranslations[paymentType]; ok {
			paymentType = val
		}

		record := []string{
			fmt.Sprintf("%d", p.ID),
			fmt.Sprintf("%d", p.ContractID),
			paymentType,
			fmt.Sprintf("%.2f", paidAmount),
			payDate,
			clientName,
			clientIdentity,
			projectName,
			lotName,
			financingType,
			paymentTerm,
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return b, nil
}

// GenerateOverduePaymentsCSV generates a CSV of overdue payments
func (s *ReportService) GenerateOverduePaymentsCSV(ctx context.Context) (*bytes.Buffer, error) {
	payments, err := s.paymentRepo.FindOverdue(ctx)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	header := []string{"ID", "Contrato", "Cliente", "Teléfono", "Fecha Vencimiento", "Días Mora", "Monto", "Interés Acum."}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, p := range payments {
		daysOverdue := int(time.Since(p.DueDate).Hours() / 24)

		clientName := "N/A"
		phone := "N/A"
		if p.Contract.ID != 0 && p.Contract.ApplicantUser.ID != 0 {
			clientName = p.Contract.ApplicantUser.FullName
			phone = p.Contract.ApplicantUser.Phone
		}

		interest := 0.0
		if p.InterestAmount != nil {
			interest = *p.InterestAmount
		}

		record := []string{
			fmt.Sprintf("%d", p.ID),
			fmt.Sprintf("%d", p.ContractID),
			clientName,
			phone,
			p.DueDate.Format("2006-01-02"),
			fmt.Sprintf("%d", daysOverdue),
			fmt.Sprintf("%.2f", p.Amount),
			fmt.Sprintf("%.2f", interest),
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return b, nil
}

// Helper to generate PDF from HTML template (templates are embedded in the binary)
func (s *ReportService) generatePDF(templateName string, data interface{}) (*bytes.Buffer, error) {
	// 1. Parse Template from embedded FS
	tmplPath := "templates/reports/" + templateName
	tmpl, err := template.ParseFS(reportTemplates, tmplPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s (path: %s): %w", templateName, tmplPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// 2. Convert to PDF using wkhtmltopdf
	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "wkhtmltopdf") && (strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "not installed")) {
			return nil, fmt.Errorf("wkhtmltopdf is required for PDF generation but is not installed. Install it: macOS: brew install --cask wkhtmltopdf (or download from https://wkhtmltopdf.org); Linux: apt-get install wkhtmltopdf. Original error: %w", err)
		}
		return nil, fmt.Errorf("failed to create pdf generator: %w", err)
	}

	// Set options
	pdfg.Dpi.Set(300)
	pdfg.Orientation.Set(wkhtmltopdf.OrientationPortrait)
	pdfg.Grayscale.Set(false)

	// Add page from buffer
	page := wkhtmltopdf.NewPageReader(bytes.NewReader(buf.Bytes()))
	page.EnableLocalFileAccess.Set(true)
	pdfg.AddPage(page)

	// Create PDF
	if err := pdfg.Create(); err != nil {
		return nil, fmt.Errorf("failed to create pdf: %w", err)
	}

	return pdfg.Buffer(), nil
}

// GenerateUserBalancePDF generates a PDF statement of account for a user
func (s *ReportService) GenerateUserBalancePDF(ctx context.Context, userID uint) (*bytes.Buffer, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	contracts, err := s.contractRepo.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Prepare data for template
	type PaymentData struct {
		PaymentType string
		DueDate     string
		Amount      string
		PaidAmount  string
		Status      string
	}

	type ContractData struct {
		ID       uint
		Payments []PaymentData
	}

	type ReportData struct {
		User      interface{}
		Date      string
		Contracts []ContractData
	}

	paymentTypeTranslations := map[string]string{
		models.PaymentTypeReservation:      "Reservación",
		models.PaymentTypeDownPayment:      "Prima",
		models.PaymentTypeInstallment:      "Cuota",
		models.PaymentTypeFull:             "Pago Total",
		models.PaymentTypeAdvance:          "Abono a Capital",
		models.PaymentTypeCapitalRepayment: "Amortización a Capital",
	}

	var contractDataList []ContractData
	for _, c := range contracts {
		cDetails, err := s.contractRepo.FindByIDWithDetails(ctx, c.ID)
		if err == nil {
			var payments []PaymentData
			for _, p := range cDetails.Payments {
				paid := 0.0
				if p.PaidAmount != nil {
					paid = *p.PaidAmount
				}
				paymentTypeLabel := p.PaymentType
				if translated, ok := paymentTypeTranslations[p.PaymentType]; ok {
					paymentTypeLabel = translated
				}
				payments = append(payments, PaymentData{
					PaymentType: paymentTypeLabel,
					DueDate:     p.DueDate.Format("02/01/2006"),
					Amount:      fmt.Sprintf("%.2f", p.Amount),
					PaidAmount:  fmt.Sprintf("%.2f", paid),
					Status:      p.Status,
				})
			}
			contractDataList = append(contractDataList, ContractData{
				ID:       c.ID,
				Payments: payments,
			})
		}
	}

	data := ReportData{
		User:      user,
		Date:      time.Now().Format("02/01/2006"),
		Contracts: contractDataList,
	}

	return s.generatePDF("user_balance.html", data)
}

// GenerateContractPDF generates a PDF for a specific contract
func (s *ReportService) GenerateContractPDF(ctx context.Context, contractID uint) (*bytes.Buffer, error) {
	contract, err := s.contractRepo.FindByIDWithDetails(ctx, contractID)
	if err != nil {
		return nil, err
	}

	// Helper for currency formatting
	toCurrency := func(amount float64) string {
		return fmt.Sprintf("%.2f", amount)
	}

	// Helper for date formatting
	formatDate := func(t time.Time) string {
		months := []string{"", "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}
		return fmt.Sprintf("%d de %s del %d", t.Day(), months[t.Month()], t.Year())
	}

	// Helper for basic number to words (simplified for this context)
	// Note: A full implementation would be much larger. This is a placeholder or requires a robust impl.
	// For now, we will use a simplified version or just return the formatted number if too complex.
	// Real-world usage requires a proper library or comprehensive function.
	amountToWords := func(amount float64) string {
		// Ideally use specific library. For now, returning formatted string with currency text.
		// User can replace with robust logic.
		return fmt.Sprintf("%.2f", amount)
	}

	// Prepare Applicant Data
	clientName := "EL CLIENTE"
	clientIdentity := "____________________"
	clientAddress := "____________________"
	if contract.ApplicantUser.ID != 0 {
		clientName = contract.ApplicantUser.FullName
		if contract.ApplicantUser.Identity != "" {
			clientIdentity = contract.ApplicantUser.Identity
		}
		if contract.ApplicantUser.Address != nil && *contract.ApplicantUser.Address != "" {
			clientAddress = *contract.ApplicantUser.Address
		}
	}

	// Prepare Project & Lot Data
	projectName := "EL PROYECTO"
	projectAddress := "________________"
	projectInterestRate := 0.0
	measurementUnit := "V2" // default

	lotName := "DEL LOTE"
	lotAddress := ""
	lotWidth := 0.0
	lotLength := 0.0
	lotArea := 0.0
	lotAreaUnit := 0.0

	if contract.Lot.ID != 0 {
		lotName = contract.Lot.Name
		lotWidth = contract.Lot.Width
		lotLength = contract.Lot.Length
		lotArea = contract.Lot.Area()

		if contract.Lot.Address != nil {
			lotAddress = *contract.Lot.Address
		}

		if contract.Lot.MeasurementUnit != nil {
			measurementUnit = *contract.Lot.MeasurementUnit
		}

		if contract.Lot.Project.ID != 0 {
			projectName = contract.Lot.Project.Name
			projectAddress = contract.Lot.Project.Address
			projectInterestRate = contract.Lot.Project.InterestRate
			if contract.Lot.MeasurementUnit == nil {
				measurementUnit = contract.Lot.Project.MeasurementUnit
			}
		}
		// Assuming AreaInProjectUnit might be different or needed. Go model doesn't show it explicitly calculated different from Area().
		lotAreaUnit = lotArea // Simplified
	}

	amount := 0.0
	if contract.Amount != nil {
		amount = *contract.Amount
	}

	reserveAmount := 0.0
	if contract.ReserveAmount != nil {
		reserveAmount = *contract.ReserveAmount
	}

	downPayment := 0.0
	if contract.DownPayment != nil {
		downPayment = *contract.DownPayment
	}

	financingAmount := amount - reserveAmount - downPayment

	// Payment details
	firstPaymentDate := "__________"
	lastPaymentDate := "__________"
	monthlyPayment := 0.0

	var installments []models.Payment
	for _, p := range contract.Payments {
		if p.PaymentType == models.PaymentTypeInstallment {
			installments = append(installments, p)
		}
	}

	if len(installments) > 0 {
		// Find first and last by due date
		first := installments[0]
		last := installments[0]
		for _, p := range installments {
			if p.DueDate.Before(first.DueDate) {
				first = p
			}
			if p.DueDate.After(last.DueDate) {
				last = p
			}
		}
		firstPaymentDate = formatDate(first.DueDate)
		lastPaymentDate = formatDate(last.DueDate)
		monthlyPayment = first.Amount // Assuming equal payments roughly
	}

	maxPaymentDate := ""
	if contract.MaxPaymentDate != nil {
		maxPaymentDate = formatDate(*contract.MaxPaymentDate)
	}

	data := map[string]interface{}{
		"ClientName":           clientName,
		"ClientIdentity":       clientIdentity,
		"ClientAddress":        clientAddress,
		"ProjectName":          projectName,
		"ProjectAddress":       projectAddress,
		"InterestRate":         fmt.Sprintf("%.2f", projectInterestRate),
		"LotName":              lotName,
		"LotAddress":           lotAddress,
		"LotWidth":             fmt.Sprintf("%.2f", lotWidth),
		"LotLength":            fmt.Sprintf("%.2f", lotLength),
		"LotAreaM2":            fmt.Sprintf("%.2f", lotArea),
		"LotAreaUnit":          fmt.Sprintf("%.2f", lotAreaUnit),
		"MeasurementUnit":      measurementUnit,
		"Amount":               toCurrency(amount),
		"AmountWords":          amountToWords(amount), // user can tackle the complexity of spanish words
		"ReserveAmount":        toCurrency(reserveAmount),
		"ReserveAmountWords":   amountToWords(reserveAmount),
		"DownPayment":          toCurrency(downPayment),
		"DownPaymentWords":     amountToWords(downPayment),
		"FinancingAmount":      toCurrency(financingAmount),
		"FinancingAmountWords": amountToWords(financingAmount),
		"PaymentTerm":          contract.PaymentTerm,
		"Currency":             contract.Currency,
		"FirstPaymentDate":     firstPaymentDate,
		"LastPaymentDate":      lastPaymentDate,
		"MonthlyPayment":       toCurrency(monthlyPayment),
		"MonthlyPaymentWords":  amountToWords(monthlyPayment),
		"Date":                 formatDate(time.Now()),
		"FinancingType":        contract.FinancingType,
		"MaxPaymentDate":       maxPaymentDate,
		"RawDownPayment":       downPayment,
	}

	return s.generatePDF("contract_promise.html", data)
}

// GenerateCustomerRecordPDF generates a PDF report for a customer record
func (s *ReportService) GenerateCustomerRecordPDF(ctx context.Context, contractID uint) (*bytes.Buffer, error) {
	contract, err := s.contractRepo.FindByIDWithDetails(ctx, contractID)
	if err != nil {
		return nil, err
	}

	// Helper for currency formatting
	toCurrency := func(amount float64) string {
		return fmt.Sprintf("L. %.2f", amount)
	}

	clientName := contract.ApplicantUser.FullName
	clientID := contract.ApplicantUser.Identity
	clientPhone := contract.ApplicantUser.Phone
	clientEmail := contract.ApplicantUser.Email
	clientAddress := "_______________________"
	if contract.ApplicantUser.Address != nil && *contract.ApplicantUser.Address != "" {
		clientAddress = *contract.ApplicantUser.Address
	}

	projectName := "N/A"
	lotName := "N/A"
	dimensions := "0m x 0m"
	measureUnit := "V2"
	areaStr := "0"

	if contract.Lot.ID != 0 {
		lotName = contract.Lot.Name
		dimensions = fmt.Sprintf("%.2fm x %.2fm", contract.Lot.Length, contract.Lot.Width)
		areaStr = fmt.Sprintf("%.2f", contract.Lot.Area())
		if contract.Lot.MeasurementUnit != nil {
			measureUnit = *contract.Lot.MeasurementUnit
		} else if contract.Lot.Project.ID != 0 {
			measureUnit = contract.Lot.Project.MeasurementUnit
		}

		if contract.Lot.Project.ID != 0 {
			projectName = contract.Lot.Project.Name
		}
	}

	effectivePrice := 0.0
	basePrice := 0.0
	overridePrice := 0.0
	hasOverride := false

	if contract.Lot.ID != 0 {
		effectivePrice = contract.Lot.EffectivePrice()
		basePrice = contract.Lot.Price
		if contract.Lot.OverridePrice != nil {
			overridePrice = *contract.Lot.OverridePrice
			hasOverride = true
		}
	}

	reserveAmount := 0.0
	if contract.ReserveAmount != nil {
		reserveAmount = *contract.ReserveAmount
	}

	downPayment := 0.0
	if contract.DownPayment != nil {
		downPayment = *contract.DownPayment
	}

	installmentAmount := 0.0
	endDate := "N/A"
	var maxDate time.Time
	for _, p := range contract.Payments {
		if p.PaymentType == models.PaymentTypeInstallment {
			if installmentAmount == 0 {
				installmentAmount = p.Amount
			}
			if p.DueDate.After(maxDate) {
				maxDate = p.DueDate
			}
		}
	}
	if !maxDate.IsZero() {
		endDate = maxDate.Format("02/01/2006")
	}

	startDate := contract.CreatedAt.Format("02/01/2006")

	financingType := contract.FinancingType
	financingTypeEs := map[string]string{
		models.FinancingTypeDirect: "Directo",
		models.FinancingTypeBank:   "Bancario",
		models.FinancingTypeCash:   "Contado",
	}
	if val, ok := financingTypeEs[contract.FinancingType]; ok {
		financingType = val
	}

	data := map[string]interface{}{
		"ClientName":        clientName,
		"ClientID":          clientID,
		"ClientPhone":       clientPhone,
		"ClientEmail":       clientEmail,
		"ClientAddress":     clientAddress,
		"ProjectName":       projectName,
		"LotName":           lotName,
		"Dimensions":        dimensions,
		"Area":              fmt.Sprintf("%s %s", areaStr, measureUnit),
		"ContractID":        contract.ID,
		"FinancingType":     financingType,
		"Price":             toCurrency(effectivePrice),
		"HasOverride":       hasOverride,
		"BasePrice":         toCurrency(basePrice),
		"OverridePrice":     toCurrency(overridePrice),
		"ReserveAmount":     toCurrency(reserveAmount),
		"DownPayment":       toCurrency(downPayment),
		"InstallmentAmount": toCurrency(installmentAmount),
		"Term":              fmt.Sprintf("%d meses", contract.PaymentTerm),
		"StartDate":         startDate,
		"EndDate":           endDate,
		"GeneratedDate":     time.Now().Format("02/01/2006"),
	}

	return s.generatePDF("customer_record.html", data)
}

// GenerateRescissionContractPDF generates a PDF for contract rescission
func (s *ReportService) GenerateRescissionContractPDF(ctx context.Context, contractID uint, refundAmount, penaltyAmount float64) (*bytes.Buffer, error) {
	contract, err := s.contractRepo.FindByIDWithDetails(ctx, contractID)
	if err != nil {
		return nil, err
	}

	// Prepare data
	clientName := "EL CLIENTE"
	clientIdentity := "____________________"
	clientAddress := ""
	if contract.ApplicantUser.ID != 0 {
		clientName = contract.ApplicantUser.FullName
		clientIdentity = contract.ApplicantUser.Identity
		if contract.ApplicantUser.Address != nil {
			clientAddress = *contract.ApplicantUser.Address
		}
	}

	projectName := "EL PROYECTO"
	projectAddress := "____________________"
	lotName := "DEL LOTE"
	lotLength := 0.0
	lotWidth := 0.0
	north := "__________"
	south := "__________"
	east := "__________"
	west := "__________"

	if contract.Lot.ID != 0 {
		lotName = contract.Lot.Name
		lotLength = contract.Lot.Length
		lotWidth = contract.Lot.Width

		if contract.Lot.North != nil {
			north = *contract.Lot.North
		}
		if contract.Lot.South != nil {
			south = *contract.Lot.South
		}
		if contract.Lot.East != nil {
			east = *contract.Lot.East
		}
		if contract.Lot.West != nil {
			west = *contract.Lot.West
		}

		if contract.Lot.Project.ID != 0 {
			projectName = contract.Lot.Project.Name
			projectAddress = contract.Lot.Project.Address
		}
	}

	// Date Formatting
	months := []string{"", "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}

	now := time.Now()
	dayStr := fmt.Sprintf("%d", now.Day())
	monthStr := months[now.Month()]
	yearStr := fmt.Sprintf("%d", now.Year())

	contractDate := ""
	if !contract.CreatedAt.IsZero() {
		cd := contract.CreatedAt
		contractDate = fmt.Sprintf("%d de %s de %d", cd.Day(), months[cd.Month()], cd.Year())
	}

	data := map[string]interface{}{
		"ApplicantName":      clientName,
		"ApplicantNameUpper": clientName, // In templates we can just use ToUpper but pre-calculating is fine
		"ApplicantIdentity":  clientIdentity,
		"ApplicantAddress":   clientAddress,
		"ContractDate":       contractDate,
		"LotName":            lotName,
		"ProjectName":        projectName,
		"ProjectAddress":     projectAddress,
		"LotLength":          fmt.Sprintf("%.2f", lotLength),
		"LotWidth":           fmt.Sprintf("%.2f", lotWidth),
		"North":              north,
		"South":              south,
		"East":               east,
		"West":               west,
		"RefundAmount":       fmt.Sprintf("%.2f", refundAmount),
		"PenaltyAmount":      fmt.Sprintf("%.2f", penaltyAmount),
		"Day":                dayStr,
		"Month":              monthStr,
		"Year":               yearStr,
	}

	return s.generatePDF("rescission_contract.html", data)
}

// SellerDashboardStats holds aggregated data for the seller dashboard
type SellerDashboardStats struct {
	TotalSalesValue   float64          `json:"total_sales_value"`
	ActiveLeads       int64            `json:"active_leads"`
	PendingCommission float64          `json:"pending_commission"`
	ConversionRate    float64          `json:"conversion_rate"`
	ChartData         DashboardChart   `json:"chart_data"`
	RecentCustomers   []RecentCustomer `json:"recent_customers"`
}

type DashboardChart struct {
	Labels []string  `json:"labels"`
	Data   []float64 `json:"data"`
}

type RecentCustomer struct {
	Name    string `json:"name"`
	Project string `json:"project"`
	Status  string `json:"status"`
	Amount  string `json:"amount"`
	Date    string `json:"date"`
}

// GetSellerDashboardStats generates statistics for the seller dashboard.
// All metrics (Volumen de Ventas, Prospectos Activos, Comisión Estimada, Tasa de Conversión)
// are computed over the last 6 months ending at the given month/year.
func (s *ReportService) GetSellerDashboardStats(ctx context.Context, userID uint, month, year int) (*SellerDashboardStats, error) {
	stats := &SellerDashboardStats{}

	// 1. Last 6 months date range (ending at end of target month)
	targetDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endOfRange := targetDate.AddDate(0, 1, 0).Add(-time.Second)
	startOfRange := targetDate.AddDate(0, -5, 0)

	// 2. Active Leads (Status = Pending or Submitted, created in last 6 months)
	activeLeadsQuery := &repository.ContractQuery{
		ListQuery: repository.NewListQuery(),
		UserID:    userID,
	}
	activeLeadsQuery.Filters["status_in"] = fmt.Sprintf("%s,%s", models.ContractStatusPending, models.ContractStatusSubmitted)
	activeLeadsQuery.Filters["start_date"] = startOfRange.Format("2006-01-02")
	activeLeadsQuery.Filters["end_date"] = endOfRange.Format("2006-01-02")
	_, count, err := s.contractRepo.List(ctx, activeLeadsQuery)
	if err != nil {
		return nil, err
	}
	stats.ActiveLeads = count

	// 3. Total Sales & Commission (last 6 months)
	monthSalesQuery := &repository.ContractQuery{
		ListQuery: repository.NewListQuery(),
		UserID:    userID,
		Status:    models.ContractStatusApproved,
	}
	monthSalesQuery.Filters["approved_from"] = startOfRange.Format("2006-01-02")
	monthSalesQuery.Filters["approved_to"] = endOfRange.Format("2006-01-02")

	approvedContracts, _, err := s.contractRepo.List(ctx, monthSalesQuery)
	if err != nil {
		return nil, err
	}

	var totalSales float64
	for _, c := range approvedContracts {
		if c.Amount != nil {
			totalSales += *c.Amount
		}
	}
	stats.TotalSalesValue = totalSales
	stats.PendingCommission = totalSales * 0.02 // 2% commission

	// 4. Conversion Rate (last 6 months: approved / (approved + active leads) * 100)
	approvedCount := float64(len(approvedContracts))
	totalActiveAndApproved := approvedCount + float64(stats.ActiveLeads)
	if totalActiveAndApproved > 0 {
		stats.ConversionRate = (approvedCount / totalActiveAndApproved) * 100
	}

	// 5. Recent Customers
	recentQuery := &repository.ContractQuery{
		ListQuery: &repository.ListQuery{
			Page:    1,
			PerPage: 5,
			SortBy:  "updated_at",
			SortDir: "desc",
			Filters: make(map[string]string),
		},
		UserID: userID,
	}
	recentContracts, _, err := s.contractRepo.List(ctx, recentQuery)
	if err != nil {
		return nil, err
	}

	for _, c := range recentContracts {
		clientName := "N/A"
		if c.ApplicantUser.ID != 0 {
			clientName = c.ApplicantUser.FullName
		}

		projectName := "N/A"
		if c.Lot.ID != 0 && c.Lot.Project.ID != 0 {
			projectName = c.Lot.Project.Name
		}

		amount := 0.0
		if c.Amount != nil {
			amount = *c.Amount
		}

		stats.RecentCustomers = append(stats.RecentCustomers, RecentCustomer{
			Name:    clientName,
			Project: projectName,
			Status:  c.Status,
			Amount:  fmt.Sprintf("L %.2f", amount),
			Date:    c.UpdatedAt.Format(time.RFC3339),
		})
	}

	// 6. Chart Data (Last 6 Months)
	iterDate := startOfRange

	for i := 0; i < 6; i++ {
		// Calculate sales for this month
		mStart := iterDate
		mEnd := mStart.AddDate(0, 1, 0).Add(-time.Second)

		// Query approved contracts for this specific month
		chartQuery := &repository.ContractQuery{
			ListQuery: repository.NewListQuery(),
			UserID:    userID,
			Status:    models.ContractStatusApproved,
		}
		chartQuery.Filters["approved_from"] = mStart.Format("2006-01-02")
		chartQuery.Filters["approved_to"] = mEnd.Format("2006-01-02")

		// Note: This is N+1 queries (6 queries). Acceptable for a dashboard with low concurrency.
		contracts, _, err := s.contractRepo.List(ctx, chartQuery)
		if err != nil {
			return nil, err
		}

		sales := 0.0
		for _, c := range contracts {
			if c.Amount != nil {
				sales += *c.Amount
			}
		}

		// Translate month
		monthName := getMonthNameSpanish(mStart.Month())
		stats.ChartData.Labels = append(stats.ChartData.Labels, monthName)
		stats.ChartData.Data = append(stats.ChartData.Data, sales)

		// Next month
		iterDate = iterDate.AddDate(0, 1, 0)
	}

	return stats, nil
}

func getMonthNameSpanish(m time.Month) string {
	months := []string{"", "Ene", "Feb", "Mar", "Abr", "May", "Jun", "Jul", "Ago", "Sep", "Oct", "Nov", "Dic"}
	if int(m) >= 1 && int(m) <= 12 {
		return months[int(m)]
	}
	return m.String()[:3]
}

package services

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"

	"github.com/resend/resend-go/v2"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

//go:embed templates/email/*.html
var emailTemplates embed.FS

type EmailService struct {
	config       *config.Config
	resendClient *resend.Client
}

func NewEmailService(cfg *config.Config) *EmailService {
	client := resend.NewClient(cfg.ResendAPIKey)
	return &EmailService{
		config:       cfg,
		resendClient: client,
	}
}

// Helper function to safely get string from pointer
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ensureEmailConfigured returns an error if Resend is not configured, so callers
// get a clear message instead of a Resend API error (e.g. 401).
func (s *EmailService) ensureEmailConfigured() error {
	if s.config.ResendAPIKey == "" {
		err := fmt.Errorf("email not configured: RESEND_API_KEY is not set (API loads .env)")
		logger.Warn(err.Error())
		return err
	}
	if s.config.FromEmail == "" {
		err := fmt.Errorf("email not configured: FROM_EMAIL is not set")
		logger.Warn(err.Error())
		return err
	}
	return nil
}

// validateEmail returns an error if the email address is invalid or empty
func (s *EmailService) validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email address is empty")
	}
	return nil
}

func (s *EmailService) SendRecoveryCode(ctx context.Context, user *models.User, code string) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send recovery code to %s: %v", user.Email, err))
		return err
	}
	if err := s.validateEmail(user.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping recovery email for user %s (ID: %d): %v", user.FullName, user.ID, err))
		return err
	}

	data := struct {
		Name    string
		Code    string
		Minutes int
		AppURL  string
	}{
		Name:    user.FullName,
		Code:    code,
		Minutes: 15,
		AppURL:  s.config.AppURL,
	}

	body, err := s.renderTemplate("reset_code.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render reset_code template: %v", err))
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{user.Email},
		Subject: "Código de reseteo",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", user.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Código de reseteo | Code: %s", user.Email, code))

	return nil
}

func (s *EmailService) SendAccountCreated(ctx context.Context, user *models.User) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send account created email to %s: %v", user.Email, err))
		return err
	}
	if err := s.validateEmail(user.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping account created email for user %s (ID: %d): %v", user.FullName, user.ID, err))
		return err
	}

	data := struct {
		Name   string
		AppURL string
	}{
		Name:   user.FullName,
		AppURL: s.config.AppURL,
	}

	body, err := s.renderTemplate("account_created.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render account_created template: %v", err))
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{user.Email},
		Subject: "¡Bienvenido a Fintera!",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", user.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: ¡Bienvenido a Fintera!", user.Email))
	return nil
}

func (s *EmailService) SendContractSubmitted(ctx context.Context, contract *models.Contract) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send contract submitted email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}
	if err := s.validateEmail(contract.ApplicantUser.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping contract submitted email for contract #%d: %v", contract.ID, err))
		return err
	}

	reserveAmount := 0.0
	if contract.ReserveAmount != nil {
		reserveAmount = *contract.ReserveAmount
	}
	downPayment := 0.0
	if contract.DownPayment != nil {
		downPayment = *contract.DownPayment
	}

	data := struct {
		Name          string
		ProjectName   string
		LotName       string
		LotAddress    string
		FinancingType string
		PaymentTerm   int
		ReserveAmount string
		DownPayment   string
		CreatedAt     string
		AppURL        string
	}{
		Name:          contract.ApplicantUser.FullName,
		ProjectName:   contract.Lot.Project.Name,
		LotName:       contract.Lot.Name,
		LotAddress:    getStringValue(contract.Lot.Address),
		FinancingType: contract.FinancingType,
		PaymentTerm:   contract.PaymentTerm,
		ReserveAmount: fmt.Sprintf("L%.2f", reserveAmount),
		DownPayment:   fmt.Sprintf("L%.2f", downPayment),
		CreatedAt:     contract.CreatedAt.Format("02/01/2006 15:04"),
		AppURL:        s.config.AppURL,
	}

	body, err := s.renderTemplate("contract_submitted.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render contract_submitted template: %v", err))
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{contract.ApplicantUser.Email},
		Subject: "Contrato Creado",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Contrato Creado", contract.ApplicantUser.Email))
	return nil
}

func (s *EmailService) SendContractApproved(ctx context.Context, contract *models.Contract, monthlyPayment float64, firstPaymentDate string) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send contract approved email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}
	if err := s.validateEmail(contract.ApplicantUser.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping contract approved email for contract #%d: %v", contract.ID, err))
		return err
	}

	downPayment := 0.0
	if contract.DownPayment != nil {
		downPayment = *contract.DownPayment
	}

	financingTypeEs := map[string]string{
		models.FinancingTypeDirect: "Directo",
		models.FinancingTypeBank:   "Bancario",
		models.FinancingTypeCash:   "Contado",
	}
	financingType := contract.FinancingType
	if val, ok := financingTypeEs[contract.FinancingType]; ok {
		financingType = val
	}

	data := struct {
		Name             string
		ProjectName      string
		LotName          string
		FinancingType    string
		PaymentTerm      int
		DownPayment      string
		MonthlyPayment   string
		FirstPaymentDate string
		ApprovedAt       string
		AppURL           string
	}{
		Name:             contract.ApplicantUser.FullName,
		ProjectName:      contract.Lot.Project.Name,
		LotName:          contract.Lot.Name,
		FinancingType:    financingType,
		PaymentTerm:      contract.PaymentTerm,
		DownPayment:      fmt.Sprintf("L%.2f", downPayment),
		MonthlyPayment:   fmt.Sprintf("L%.2f", monthlyPayment),
		FirstPaymentDate: firstPaymentDate,
		ApprovedAt:       contract.ApprovedAt.Format("02/01/2006 15:04"),
		AppURL:           s.config.AppURL,
	}

	body, err := s.renderTemplate("contract_approved.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render contract_approved template: %v", err))
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{contract.ApplicantUser.Email},
		Subject: "Contrato Aprobado",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Contrato Aprobado", contract.ApplicantUser.Email))
	return nil
}

// SendContractRejected sends an email to the contract owner with the rejection reason.
func (s *EmailService) SendContractRejected(ctx context.Context, contract *models.Contract, reason string) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send contract rejected email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}
	if err := s.validateEmail(contract.ApplicantUser.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping contract rejected email for contract #%d: %v", contract.ID, err))
		return err
	}

	projectName := ""
	lotName := ""
	if contract.Lot.Project.ID != 0 {
		projectName = contract.Lot.Project.Name
	}
	if contract.Lot.ID != 0 {
		lotName = contract.Lot.Name
	}
	reasonText := reason
	if reasonText == "" {
		reasonText = "No se proporcionó una razón específica."
	}
	data := struct {
		Name        string
		ProjectName string
		LotName     string
		Reason      string
		AppURL      string
	}{
		Name:        contract.ApplicantUser.FullName,
		ProjectName: projectName,
		LotName:     lotName,
		Reason:      reasonText,
		AppURL:      s.config.AppURL,
	}
	body, err := s.renderTemplate("contract_rejected.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render contract_rejected template: %v", err))
		return err
	}
	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{contract.ApplicantUser.Email},
		Subject: "Contrato rechazado",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}
	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Contrato rechazado", contract.ApplicantUser.Email))
	return nil
}

func (s *EmailService) SendPaymentApproved(ctx context.Context, payment *models.Payment) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send payment approved email to %s: %v", payment.Contract.ApplicantUser.Email, err))
		return err
	}
	if err := s.validateEmail(payment.Contract.ApplicantUser.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping payment approved email for payment #%d: %v", payment.ID, err))
		return err
	}

	interest := 0.0
	if payment.InterestAmount != nil {
		interest = *payment.InterestAmount
	}
	totalAmount := payment.Amount + interest
	data := struct {
		Name           string
		ProjectName    string
		LotName        string
		PaymentAmount  string
		InterestAmount string
		TotalAmount    string
		DueDate        string
		ApprovedAt     string
		AppURL         string
	}{
		Name:           payment.Contract.ApplicantUser.FullName,
		ProjectName:    payment.Contract.Lot.Project.Name,
		LotName:        payment.Contract.Lot.Name,
		PaymentAmount:  fmt.Sprintf("L%.2f", payment.Amount),
		InterestAmount: fmt.Sprintf("L%.2f", interest),
		TotalAmount:    fmt.Sprintf("L%.2f", totalAmount),
		DueDate:        payment.DueDate.Format("02/01/2006"),
		ApprovedAt:     payment.ApprovedAt.Format("02/01/2006"),
		AppURL:         s.config.AppURL,
	}

	body, err := s.renderTemplate("payment_approved.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render payment_approved template: %v", err))
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{payment.Contract.ApplicantUser.Email},
		Subject: "Pago Aprobado",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", payment.Contract.ApplicantUser.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Pago Aprobado", payment.Contract.ApplicantUser.Email))
	return nil
}

func (s *EmailService) SendPaymentRejected(ctx context.Context, contract *models.Contract, amount float64, dueDate string, reason string) error {
	if err := s.ensureEmailConfigured(); err != nil {
		return err
	}
	if err := s.validateEmail(contract.ApplicantUser.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping payment rejected email for contract #%d: %v", contract.ID, err))
		return err
	}

	data := struct {
		Name        string
		ProjectName string
		LotName     string
		Amount      string
		DueDate     string
		Reason      string
		AppURL      string
	}{
		Name:        contract.ApplicantUser.FullName,
		ProjectName: contract.Lot.Project.Name,
		LotName:     contract.Lot.Name,
		Amount:      fmt.Sprintf("L%.2f", amount),
		DueDate:     dueDate,
		Reason:      reason,
		AppURL:      s.config.AppURL,
	}

	body, err := s.renderTemplate("payment_rejected.html", data)
	if err != nil {
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{contract.ApplicantUser.Email},
		Subject: "Pago Rechazado",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Pago Rechazado", contract.ApplicantUser.Email))
	return nil
}

type OverduePaymentData struct {
	LotName string
	Amount  string
	DueDate string
}

func (s *EmailService) SendOverduePayments(ctx context.Context, user *models.User, payments []models.Payment) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send overdue payments email to %s: %v", user.Email, err))
		return err
	}
	if err := s.validateEmail(user.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping overdue payments email for user %s (ID: %d): %v", user.FullName, user.ID, err))
		return err
	}

	var paymentData []OverduePaymentData
	for _, p := range payments {
		paymentData = append(paymentData, OverduePaymentData{
			LotName: p.Contract.Lot.Name,
			Amount:  fmt.Sprintf("L%.2f", p.Amount),
			DueDate: p.DueDate.Format("02/01/2006"),
		})
	}

	data := struct {
		Name     string
		Payments []OverduePaymentData
		AppURL   string
	}{
		Name:     user.FullName,
		Payments: paymentData,
		AppURL:   s.config.AppURL,
	}

	body, err := s.renderTemplate("overdue_payment.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render overdue_payment template: %v", err))
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{user.Email},
		Subject: fmt.Sprintf("Pagos Vencidos (%d pagos)", len(payments)),
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", user.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Pagos Vencidos (%d pagos)", user.Email, len(payments)))
	return nil
}

func (s *EmailService) SendReservationApproved(ctx context.Context, contract *models.Contract) error {
	if err := s.ensureEmailConfigured(); err != nil {
		logger.Error(fmt.Sprintf("Failed to send reservation approved email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}
	if err := s.validateEmail(contract.ApplicantUser.Email); err != nil {
		logger.Warn(fmt.Sprintf("Skipping reservation approved email for contract #%d: %v", contract.ID, err))
		return err
	}

	reserveAmount := 0.0
	if contract.ReserveAmount != nil {
		reserveAmount = *contract.ReserveAmount
	}

	data := struct {
		Name          string
		ContractID    uint
		ProjectName   string
		LotName       string
		ReserveAmount string
		AppURL        string
	}{
		Name:          contract.ApplicantUser.FullName,
		ContractID:    contract.ID,
		ProjectName:   contract.Lot.Project.Name,
		LotName:       contract.Lot.Name,
		ReserveAmount: fmt.Sprintf("L%.2f", reserveAmount),
		AppURL:        s.config.AppURL,
	}

	body, err := s.renderTemplate("reservation_approved.html", data)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to render reservation_approved template: %v", err))
		return err
	}

	params := &resend.SendEmailRequest{
		From:    s.config.FromEmail,
		To:      []string{contract.ApplicantUser.Email},
		Subject: "Reserva Aprobada",
		Html:    body,
	}
	_, err = s.resendClient.Emails.Send(params)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", contract.ApplicantUser.Email, err))
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Email Sent] To: %s | Subject: Reserva Aprobada", contract.ApplicantUser.Email))
	return nil
}

func (s *EmailService) renderTemplate(name string, data interface{}) (string, error) {
	tmpl, err := template.ParseFS(emailTemplates, "templates/email/"+name)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

package services

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"

	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

//go:embed templates/email/*.html
var emailTemplates embed.FS

type EmailService struct {
	config *config.Config
	// resendClient *resend.Client // Use this when implementing actual sending
}

func NewEmailService(cfg *config.Config) *EmailService {
	// client := resend.NewClient(cfg.ResendAPIKey)
	return &EmailService{
		config: cfg,
	}
}

// Helper function to safely get string from pointer
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *EmailService) SendRecoveryCode(ctx context.Context, user *models.User, code string) error {
	data := struct {
		Name    string
		Code    string
		Minutes int
		AppURL  string
	}{
		Name:    user.FullName,
		Code:    code,
		Minutes: 15,
		AppURL:  "https://fintera.securexapp.com", // Should come from config
	}

	body, err := s.renderTemplate("reset_code.html", data)
	if err != nil {
		return err
	}

	// TODO: Replace with actual Resend call
	// params := &resend.SendEmailRequest{
	// 	From:    s.config.FromEmail,
	// 	To:      []string{user.Email},
	// 	Subject: "Código de reseteo",
	// 	Html:    body,
	// }
	// _, err = s.resendClient.Emails.Send(params)

	// For now, log it
	logger.Info(fmt.Sprintf("📧 [Mock Email] To: %s | Subject: Código de reseteo | Code: %s", user.Email, code))
	logger.Debug(fmt.Sprintf("📧 Body Preview: %s", body[:100])) // Print first 100 chars

	return nil
}

func (s *EmailService) SendAccountCreated(ctx context.Context, user *models.User) error {
	data := struct {
		Name   string
		AppURL string
	}{
		Name:   user.FullName,
		AppURL: "https://fintera.securexapp.com",
	}

	_, err := s.renderTemplate("account_created.html", data)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Mock Email] To: %s | Subject: ¡Bienvenido a Fintera!", user.Email))
	return nil
}

func (s *EmailService) SendContractSubmitted(ctx context.Context, contract *models.Contract) error {
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
		AppURL:        "https://fintera.securexapp.com",
	}

	_, err := s.renderTemplate("contract_submitted.html", data)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Mock Email] To: %s | Subject: Contrato Creado", contract.ApplicantUser.Email))
	return nil
}

func (s *EmailService) SendContractApproved(ctx context.Context, contract *models.Contract, monthlyPayment float64, firstPaymentDate string) error {
	downPayment := 0.0
	if contract.DownPayment != nil {
		downPayment = *contract.DownPayment
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
		FinancingType:    contract.FinancingType,
		PaymentTerm:      contract.PaymentTerm,
		DownPayment:      fmt.Sprintf("L%.2f", downPayment),
		MonthlyPayment:   fmt.Sprintf("L%.2f", monthlyPayment),
		FirstPaymentDate: firstPaymentDate,
		ApprovedAt:       contract.ApprovedAt.Format("02/01/2006 15:04"),
		AppURL:           "https://fintera.securexapp.com",
	}

	_, err := s.renderTemplate("contract_approved.html", data)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Mock Email] To: %s | Subject: Contrato Aprobado", contract.ApplicantUser.Email))
	return nil
}

func (s *EmailService) SendPaymentApproved(ctx context.Context, payment *models.Payment) error {
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
		AppURL:         "https://fintera.securexapp.com",
	}

	_, err := s.renderTemplate("payment_approved.html", data)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Mock Email] To: %s | Subject: Pago Aprobado", payment.Contract.ApplicantUser.Email))
	return nil
}

type OverduePaymentData struct {
	LotName string
	Amount  string
	DueDate string
}

func (s *EmailService) SendOverduePayments(ctx context.Context, user *models.User, payments []models.Payment) error {
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
		AppURL:   "https://fintera.securexapp.com",
	}

	_, err := s.renderTemplate("overdue_payment.html", data)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Mock Email] To: %s | Subject: Pagos Vencidos (%d pagos)", user.Email, len(payments)))
	return nil
}

func (s *EmailService) SendReservationApproved(ctx context.Context, contract *models.Contract) error {
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
		AppURL:        "https://fintera.securexapp.com",
	}

	_, err := s.renderTemplate("reservation_approved.html", data)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("📧 [Mock Email] To: %s | Subject: Reserva Aprobada", contract.ApplicantUser.Email))
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

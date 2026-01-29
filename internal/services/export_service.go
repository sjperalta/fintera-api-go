package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/xuri/excelize/v2"
)

type ExportService struct {
	analyticsSvc *AnalyticsService
}

func NewExportService(analyticsSvc *AnalyticsService) *ExportService {
	return &ExportService{analyticsSvc: analyticsSvc}
}

func (s *ExportService) ExportCSV(ctx context.Context, overview *models.AnalyticsOverview, dist *models.LotDistribution) ([]byte, string, error) {
	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)

	// Header
	_ = writer.Write([]string{"Reporte de Analíticas", time.Now().Format("2006-01-02 15:04")})
	_ = writer.Write([]string{""})

	// Overview Section
	_ = writer.Write([]string{"Resumen General"})
	_ = writer.Write([]string{"Métrica", "Valor"})
	_ = writer.Write([]string{"Ingresos Totales", fmt.Sprintf("%.2f", overview.TotalRevenue)})
	_ = writer.Write([]string{"Contratos Activos", fmt.Sprintf("%d", overview.ActiveContracts)})
	_ = writer.Write([]string{"Pago Promedio", fmt.Sprintf("%.2f", overview.AveragePayment)})
	_ = writer.Write([]string{"Tasa de Ocupación", fmt.Sprintf("%.2f%%", overview.OccupancyRate)})
	_ = writer.Write([]string{""})

	// Distribution Section
	_ = writer.Write([]string{"Distribución de Lotes"})
	_ = writer.Write([]string{"Estado", "Cantidad"})
	_ = writer.Write([]string{"Financiado", fmt.Sprintf("%d", dist.Financed)})
	_ = writer.Write([]string{"Pagado Totalmente", fmt.Sprintf("%d", dist.FullyPaid)})
	_ = writer.Write([]string{"Reservado", fmt.Sprintf("%d", dist.Reserved)})
	_ = writer.Write([]string{"Disponible", fmt.Sprintf("%d", dist.Available)})
	_ = writer.Write([]string{"Total", fmt.Sprintf("%d", dist.TotalLots)})

	writer.Flush()

	filename := fmt.Sprintf("analytics_report_%s.csv", time.Now().Format("2006-01-02"))
	return buf.Bytes(), filename, nil
}

func (s *ExportService) ExportXLSX(ctx context.Context, overview *models.AnalyticsOverview, dist *models.LotDistribution) ([]byte, string, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Analytics"
	_ = f.SetSheetName("Sheet1", sheet)

	// Summary Styles
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})

	// Write Overview
	_ = f.SetCellValue(sheet, "A1", "Reporte de Analíticas")
	_ = f.SetCellStyle(sheet, "A1", "A1", headerStyle)

	_ = f.SetCellValue(sheet, "A3", "Resumen General")
	_ = f.SetCellValue(sheet, "A4", "Métrica")
	_ = f.SetCellValue(sheet, "B4", "Valor")

	_ = f.SetCellValue(sheet, "A5", "Ingresos Totales")
	_ = f.SetCellValue(sheet, "B5", overview.TotalRevenue)
	_ = f.SetCellValue(sheet, "A6", "Contratos Activos")
	_ = f.SetCellValue(sheet, "B6", overview.ActiveContracts)
	_ = f.SetCellValue(sheet, "A7", "Pago Promedio")
	_ = f.SetCellValue(sheet, "B7", overview.AveragePayment)
	_ = f.SetCellValue(sheet, "A8", "Tasa de Ocupación")
	_ = f.SetCellValue(sheet, "B8", fmt.Sprintf("%.2f%%", overview.OccupancyRate))

	// Write Distribution
	_ = f.SetCellValue(sheet, "A10", "Distribución de Lotes")
	_ = f.SetCellValue(sheet, "A11", "Estado")
	_ = f.SetCellValue(sheet, "B11", "Cantidad")

	_ = f.SetCellValue(sheet, "A12", "Financiado")
	_ = f.SetCellValue(sheet, "B12", dist.Financed)
	_ = f.SetCellValue(sheet, "A13", "Pagado Totalmente")
	_ = f.SetCellValue(sheet, "B13", dist.FullyPaid)
	_ = f.SetCellValue(sheet, "A14", "Reservado")
	_ = f.SetCellValue(sheet, "B14", dist.Reserved)
	_ = f.SetCellValue(sheet, "A15", "Disponible")
	_ = f.SetCellValue(sheet, "B15", dist.Available)
	_ = f.SetCellValue(sheet, "A16", "Total")
	_ = f.SetCellValue(sheet, "B16", dist.TotalLots)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("analytics_report_%s.xlsx", time.Now().Format("2006-01-02"))
	return buf.Bytes(), filename, nil
}

func (s *ExportService) ExportPDF(ctx context.Context, overview *models.AnalyticsOverview, dist *models.LotDistribution) ([]byte, string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Reporte de Analiticas")
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Resumen General")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 10)
	pdf.Cell(60, 10, "Ingresos Totales:")
	pdf.Cell(40, 10, fmt.Sprintf("%.2f HNL", overview.TotalRevenue))
	pdf.Ln(6)

	pdf.Cell(60, 10, "Contratos Activos:")
	pdf.Cell(40, 10, fmt.Sprintf("%d", overview.ActiveContracts))
	pdf.Ln(6)

	pdf.Cell(60, 10, "Pago Promedio:")
	pdf.Cell(40, 10, fmt.Sprintf("%.2f HNL", overview.AveragePayment))
	pdf.Ln(6)

	pdf.Cell(60, 10, "Tasa de Ocupacion:")
	pdf.Cell(40, 10, fmt.Sprintf("%.2f%%", overview.OccupancyRate))
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Distribucion de Lotes")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 10)
	pdf.Cell(60, 10, "Financiado:")
	pdf.Cell(40, 10, fmt.Sprintf("%d", dist.Financed))
	pdf.Ln(6)

	pdf.Cell(60, 10, "Pagado Totalmente:")
	pdf.Cell(40, 10, fmt.Sprintf("%d", dist.FullyPaid))
	pdf.Ln(6)

	pdf.Cell(60, 10, "Reservado:")
	pdf.Cell(40, 10, fmt.Sprintf("%d", dist.Reserved))
	pdf.Ln(6)

	pdf.Cell(60, 10, "Disponible:")
	pdf.Cell(40, 10, fmt.Sprintf("%d", dist.Available))
	pdf.Ln(6)

	pdf.Cell(60, 10, "Total:")
	pdf.Cell(40, 10, fmt.Sprintf("%d", dist.TotalLots))
	pdf.Ln(6)

	buf := new(bytes.Buffer)
	err := pdf.Output(buf)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("analytics_report_%s.pdf", time.Now().Format("2006-01-02"))
	return buf.Bytes(), filename, nil
}

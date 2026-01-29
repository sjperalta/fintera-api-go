package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/services"
)

type AnalyticsHandler struct {
	analyticsSvc *services.AnalyticsService
	exportSvc    *services.ExportService
}

func NewAnalyticsHandler(analyticsSvc *services.AnalyticsService, exportSvc *services.ExportService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsSvc: analyticsSvc,
		exportSvc:    exportSvc,
	}
}

// @Summary Get Analytics Overview
// @Description Returns high-level statistics and trend data
// @Tags Analytics
// @Produce json
// @Param project_id query int false "Project ID"
// @Param start_date query string false "Start Date (ISO 8601)"
// @Param end_date query string false "End Date (ISO 8601)"
// @Param revenue_timeframe query string false "Revenue timeframe (6M or 12M)"
// @Security BearerAuth
// @Router /analytics/overview [get]
func (h *AnalyticsHandler) Overview(c *gin.Context) {
	filters := h.parseFilters(c)
	overview, err := h.analyticsSvc.GetOverview(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, overview)
}

// @Summary Get Lot Distribution
// @Description Returns availability statistics for lots
// @Tags Analytics
// @Produce json
// @Param project_id query int false "Project ID"
// @Security BearerAuth
// @Router /analytics/distribution [get]
func (h *AnalyticsHandler) Distribution(c *gin.Context) {
	var projectID *uint
	if pidStr := c.Query("project_id"); pidStr != "" {
		pid, _ := strconv.ParseUint(pidStr, 10, 64)
		uintPid := uint(pid)
		projectID = &uintPid
	}

	dist, err := h.analyticsSvc.GetDistribution(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dist)
}

// @Summary Get Project Performance
// @Description Returns performance metrics for multiple projects
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/performance [get]
func (h *AnalyticsHandler) Performance(c *gin.Context) {
	perf, err := h.analyticsSvc.GetPerformance(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, perf)
}

// @Summary Export Analytics Data
// @Description Generates and downloads analytics reports in various formats
// @Tags Analytics
// @Produce application/octet-stream
// @Param format query string true "Report format (csv, xlsx, pdf)"
// @Param project_id query int false "Project ID"
// @Param start_date query string false "Start Date (ISO 8601)"
// @Param end_date query string false "End Date (ISO 8601)"
// @Security BearerAuth
// @Router /analytics/export [get]
func (h *AnalyticsHandler) Export(c *gin.Context) {
	format := c.Query("format")
	filters := h.parseFilters(c)

	overview, err := h.analyticsSvc.GetOverview(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get overview data"})
		return
	}

	dist, err := h.analyticsSvc.GetDistribution(c.Request.Context(), filters.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get distribution data"})
		return
	}

	var data []byte
	var filename string

	switch format {
	case "csv":
		data, filename, err = h.exportSvc.ExportCSV(c.Request.Context(), overview, dist)
	case "xlsx":
		data, filename, err = h.exportSvc.ExportXLSX(c.Request.Context(), overview, dist)
	case "pdf":
		data, filename, err = h.exportSvc.ExportPDF(c.Request.Context(), overview, dist)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format (csv, xlsx, pdf)"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate %s: %v", format, err)})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (h *AnalyticsHandler) parseFilters(c *gin.Context) services.AnalyticsFilters {
	var filters services.AnalyticsFilters

	if pidStr := c.Query("project_id"); pidStr != "" {
		pid, _ := strconv.ParseUint(pidStr, 10, 64)
		uintPid := uint(pid)
		filters.ProjectID = &uintPid
	}

	if startStr := c.Query("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filters.StartDate = &t
		}
	}

	if endStr := c.Query("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filters.EndDate = &t
		}
	}

	if yearStr := c.Query("year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			filters.Year = &y
		}
	}

	filters.RevenueTimeframe = c.Query("timeframe")
	if filters.RevenueTimeframe == "" {
		filters.RevenueTimeframe = c.DefaultQuery("revenue_timeframe", "12M")
	}

	return filters
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/services"
)

type JobHandler struct {
	jobService *services.JobService
}

func NewJobHandler(jobSvc *services.JobService) *JobHandler {
	return &JobHandler{
		jobService: jobSvc,
	}
}

// Status returns the current worker status
// @Summary Get background job status
// @Description Get statistics about background jobs (active, completed, failed, queue length)
// @Tags Jobs
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /jobs/status [get]
func (h *JobHandler) Status(c *gin.Context) {
	status := h.jobService.GetStatus()
	c.JSON(http.StatusOK, status)
}

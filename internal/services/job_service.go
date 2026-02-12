package services

import (
	"github.com/sjperalta/fintera-api/internal/jobs"
)

type JobService struct {
	worker *jobs.Worker
}

func NewJobService(worker *jobs.Worker) *JobService {
	return &JobService{
		worker: worker,
	}
}

func (s *JobService) GetStatus() map[string]interface{} {
	stats := s.worker.GetStats()
	return map[string]interface{}{
		"active_jobs":    stats.ActiveJobs,
		"completed_jobs": stats.CompletedJobs,
		"failed_jobs":    stats.FailedJobs,
		"queue_length":   stats.QueueLength,
		"max_concurrent": stats.MaxConcurrent,
	}
}

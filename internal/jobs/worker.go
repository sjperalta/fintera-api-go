package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sjperalta/fintera-api/pkg/logger"
)

// Job represents a background task
type Job func(ctx context.Context) error

// Worker manages background jobs and scheduled tasks
type Worker struct {
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	queue         chan Job
	asyncSem      chan struct{}
	maxConcurrent int
	stats         WorkerStats
	statsMu       sync.RWMutex
}

// WorkerStats holds statistics about the worker
type WorkerStats struct {
	ActiveJobs    int   `json:"active_jobs"`
	CompletedJobs int64 `json:"completed_jobs"`
	FailedJobs    int64 `json:"failed_jobs"`
	QueueLength   int   `json:"queue_length"`
	MaxConcurrent int   `json:"max_concurrent"`
}

// NewWorker creates a worker with N concurrent processors
func NewWorker(numWorkers int) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	// Allow 2x workers for async jobs
	asyncLimit := numWorkers * 2
	if asyncLimit < 10 {
		asyncLimit = 10
	}

	w := &Worker{
		ctx:           ctx,
		cancel:        cancel,
		queue:         make(chan Job, 100),
		asyncSem:      make(chan struct{}, asyncLimit),
		maxConcurrent: asyncLimit,
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		w.wg.Add(1)
		go w.process(i)
	}

	return w
}

// Enqueue adds a job to be processed by the worker pool
func (w *Worker) Enqueue(job Job) {
	select {
	case w.queue <- job:
	default:
		logger.Warn("[Worker] Queue full, running job synchronously")
		if err := job(w.ctx); err != nil {
			logger.Error(fmt.Sprintf("[Worker] Job error: %v", err))
		}
	}
}

// EnqueueAsync runs a job in a new goroutine (fire-and-forget), bounded by semaphore
func (w *Worker) EnqueueAsync(job Job) {
	go func() {
		// Acquire semaphore to limit concurrency
		w.asyncSem <- struct{}{}
		defer func() { <-w.asyncSem }()

		// Track in waitgroup
		w.wg.Add(1)
		defer w.wg.Done()

		w.trackJobStart()
		defer w.trackJobEnd()

		// Recover from panics
		defer func() {
			if r := recover(); r != nil {
				logger.Error(fmt.Sprintf("[Worker] Async job panic: %v", r))
				w.trackJobFailure()
			}
		}()

		if err := job(w.ctx); err != nil {
			logger.Error(fmt.Sprintf("[Worker] Async job error: %v", err))
			w.trackJobFailure()
		}
	}()
}

// process handles jobs from the queue
func (w *Worker) process(workerID int) {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			return
		case job, ok := <-w.queue:
			if !ok {
				return
			}
			w.trackJobStart()
			start := time.Now()
			if err := job(w.ctx); err != nil {
				logger.Error(fmt.Sprintf("[Worker %d] Job error: %v", workerID, err))
				w.trackJobFailure()
			} else {
				logger.Info(fmt.Sprintf("[Worker %d] Job completed in %v", workerID, time.Since(start)))
			}
			w.trackJobEnd()
		}
	}
}

// ScheduleEvery runs a job at fixed intervals. The first run happens after the interval (not at startup).
func (w *Worker) ScheduleEvery(interval time.Duration, job Job) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				w.runScheduledJob(job)
			}
		}
	}()
}

// ScheduleEveryImmediate runs a job once at startup, then at fixed intervals. Use this when the process
// may restart (e.g. Railway deploys) so jobs run soon after start instead of waiting for the first interval.
func (w *Worker) ScheduleEveryImmediate(interval time.Duration, job Job) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.runScheduledJob(job)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				w.runScheduledJob(job)
			}
		}
	}()
}

func (w *Worker) runScheduledJob(job Job) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Sprintf("[Scheduler] Job panic: %v", r))
			w.trackJobFailure()
			w.trackJobEnd()
		}
	}()
	w.trackJobStart()
	start := time.Now()
	if err := job(w.ctx); err != nil {
		logger.Error(fmt.Sprintf("[Scheduler] Job error: %v", err))
		w.trackJobFailure()
	} else {
		logger.Info(fmt.Sprintf("[Scheduler] Job completed in %v", time.Since(start)))
	}
	w.trackJobEnd()
}

// ScheduleAt runs a job once at a specific time
func (w *Worker) ScheduleAt(at time.Time, job Job) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		timer := time.NewTimer(time.Until(at))
		defer timer.Stop()

		select {
		case <-w.ctx.Done():
			return
		case <-timer.C:
			w.trackJobStart()
			if err := job(w.ctx); err != nil {
				logger.Error(fmt.Sprintf("[Scheduler] Scheduled job error: %v", err))
				w.trackJobFailure()
			}
			w.trackJobEnd()
		}
	}()
}

// Shutdown gracefully stops all workers
func (w *Worker) Shutdown() {
	w.cancel()
	close(w.queue)
	w.wg.Wait()
}

// Context returns the worker's context for checking cancellation
func (w *Worker) Context() context.Context {
	return w.ctx
}

// GetStats returns the current worker statistics
func (w *Worker) GetStats() WorkerStats {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()
	stats := w.stats
	stats.QueueLength = len(w.queue)
	stats.MaxConcurrent = w.maxConcurrent
	return stats
}

func (w *Worker) trackJobStart() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.ActiveJobs++
}

func (w *Worker) trackJobEnd() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.ActiveJobs--
	w.stats.CompletedJobs++
}

func (w *Worker) trackJobFailure() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.FailedJobs++
	// CompletedJobs is incremented in trackJobEnd, which is always called.
	// So we don't decrement CompletedJobs here, total jobs = completed + (failed included in completed count? No, typically separate or failed is subset).
	// Let's enable CompletedJobs to be total finished. FailedJobs is a subset or separate counter?
	// Common pattern: Completed = Success, Failed = Failure.
	// But here trackJobEnd is always called.
	// Let's adjust: trackJobEnd increments Completed. If it failed, we also increment Failed.
	// So CompletedJobs effectively means "Finished Jobs" (success or fail).
	// If we want Success count, we can derive it: Completed - Failed.
}

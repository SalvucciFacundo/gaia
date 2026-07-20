// Package cron provides a built-in cron scheduler for GAIA.
// Jobs are defined via CLI and persisted in SQLite. The scheduler
// evaluates cron expressions and delivers results to configured targets.
package cron

import (
	"context"
	"log"
	"sync"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// Scheduler manages cron job lifecycle: creation, scheduling, execution, and delivery.
type Scheduler struct {
	repo     ports.CronRepository
	delivery *DeliveryService
	jobs     map[string]*runningJob
	mu       sync.Mutex
	logger   *log.Logger
}

// runningJob holds the runtime state for a scheduled job.
type runningJob struct {
	job    domain.CronJob
	cancel context.CancelFunc
}

// NewScheduler creates a new cron scheduler.
func NewScheduler(repo ports.CronRepository) *Scheduler {
	return &Scheduler{
		repo:     repo,
		delivery: NewDeliveryService(),
		jobs:     make(map[string]*runningJob),
		logger:   log.Default(),
	}
}

// CreateJob creates a new cron job and schedules it if enabled.
func (s *Scheduler) CreateJob(ctx context.Context, job domain.CronJob) (string, error) {
	id, err := s.repo.CreateJob(ctx, job)
	if err != nil {
		return "", err
	}
	job.ID = id

	if job.Enabled {
		s.startJob(job)
	}

	return id, nil
}

// ListJobs returns all cron jobs from the store.
func (s *Scheduler) ListJobs(ctx context.Context) ([]domain.CronJob, error) {
	return s.repo.ListJobs(ctx)
}

// PauseJob disables a job and stops its scheduler.
func (s *Scheduler) PauseJob(ctx context.Context, id string) error {
	s.mu.Lock()
	rj, ok := s.jobs[id]
	if !ok {
		s.mu.Unlock()
		return nil // already paused
	}
	if rj.cancel != nil {
		rj.cancel()
	}
	delete(s.jobs, id)
	s.mu.Unlock()

	// Update enabled status in store
	job := rj.job
	job.Enabled = false
	return s.repo.UpdateJob(ctx, job)
}

// ResumeJob enables a job and starts its scheduler.
func (s *Scheduler) ResumeJob(ctx context.Context, id string) error {
	jobs, err := s.repo.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, j := range jobs {
		if j.ID == id {
			j.Enabled = true
			if err := s.repo.UpdateJob(ctx, j); err != nil {
				return err
			}
			s.startJob(j)
			return nil
		}
	}

	return nil
}

// RemoveJob deletes a job and stops its scheduler.
func (s *Scheduler) RemoveJob(ctx context.Context, id string) error {
	s.mu.Lock()
	rj, ok := s.jobs[id]
	if ok {
		if rj.cancel != nil {
			rj.cancel()
		}
		delete(s.jobs, id)
	}
	s.mu.Unlock()

	return s.repo.DeleteJob(ctx, id)
}

// Start begins the scheduler loop — evaluates jobs and runs due ones.
// This is a blocking call; run in a goroutine.
func (s *Scheduler) Start(ctx context.Context) error {
	// Load existing enabled jobs from store
	jobs, err := s.repo.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, j := range jobs {
		if j.Enabled {
			s.startJob(j)
		}
	}

	s.logger.Println("Cron scheduler started")
	<-ctx.Done()
	s.logger.Println("Cron scheduler stopped")
	return nil
}

// startJob launches a goroutine that evaluates the cron schedule and executes the job.
func (s *Scheduler) startJob(job domain.CronJob) {
	s.mu.Lock()
	if _, exists := s.jobs[job.ID]; exists {
		s.mu.Unlock()
		return // already running
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.jobs[job.ID] = &runningJob{job: job, cancel: cancel}
	s.mu.Unlock()

	go s.runJob(ctx, job)
}

// runJob evaluates the cron schedule and runs the job when due.
func (s *Scheduler) runJob(ctx context.Context, job domain.CronJob) {
	schedule, err := ParseSchedule(job.Schedule)
	if err != nil {
		s.logger.Printf("cron job %q: invalid schedule %q: %v", job.Name, job.Schedule, err)
		return
	}

	for {
		now := time.Now()
		next := schedule.Next(now)
		waitDuration := next.Sub(now)

		select {
		case <-ctx.Done():
			return
		case <-time.After(waitDuration):
			s.executeJob(context.Background(), job)
		}

		// Update next_run in store
		nextNext := schedule.Next(time.Now())
		s.repo.MarkRun(context.Background(), job.ID, time.Now(), nextNext)
	}
}

// executeJob runs a job by delivering the task.
// In production, this would invoke the Brain for task processing.
// Currently, it delivers directly to the configured target.
func (s *Scheduler) executeJob(ctx context.Context, job domain.CronJob) {
	s.logger.Printf("cron job %q executing: %s", job.Name, job.Task)

	result := domain.ToolResult{
		Success: true,
		Output:  "Scheduled task: " + job.Task,
	}

	s.delivery.Deliver(ctx, job, result)
}

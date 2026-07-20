package cron

import (
	"context"
	"testing"
	"time"

	"gaia/internal/core/domain"
)

// mockCronRepo implements ports.CronRepository for testing.
type mockCronRepo struct {
	jobs []domain.CronJob
}

func (m *mockCronRepo) ListJobs(ctx context.Context) ([]domain.CronJob, error) {
	return m.jobs, nil
}

func (m *mockCronRepo) CreateJob(ctx context.Context, job domain.CronJob) (string, error) {
	job.ID = "test-job-1"
	m.jobs = append(m.jobs, job)
	return job.ID, nil
}

func (m *mockCronRepo) UpdateJob(ctx context.Context, job domain.CronJob) error {
	for i, j := range m.jobs {
		if j.ID == job.ID {
			m.jobs[i] = job
			return nil
		}
	}
	return nil
}

func (m *mockCronRepo) DeleteJob(ctx context.Context, id string) error {
	for i, j := range m.jobs {
		if j.ID == id {
			m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockCronRepo) GetDueJobs(ctx context.Context) ([]domain.CronJob, error) {
	var due []domain.CronJob
	now := time.Now()
	for _, j := range m.jobs {
		if j.Enabled && !j.NextRun.IsZero() && j.NextRun.Before(now) {
			due = append(due, j)
		}
	}
	return due, nil
}

func (m *mockCronRepo) MarkRun(ctx context.Context, id string, lastRun, nextRun time.Time) error {
	for i, j := range m.jobs {
		if j.ID == id {
			m.jobs[i].LastRun = lastRun
			m.jobs[i].NextRun = nextRun
			return nil
		}
	}
	return nil
}

func TestParseScheduleEveryMinute(t *testing.T) {
	s, err := ParseSchedule("* * * * *")
	if err != nil {
		t.Fatalf("ParseSchedule failed: %v", err)
	}

	now := time.Now().Truncate(time.Minute)
	next := s.Next(now)
	if !next.After(now) {
		t.Errorf("expected next time after now, got %v (now: %v)", next, now)
	}

	diff := next.Sub(now)
	if diff != time.Minute {
		t.Errorf("expected next run in ~1 minute, got %v", diff)
	}
}

func TestParseScheduleSpecificTime(t *testing.T) {
	s, err := ParseSchedule("30 14 * * *") // 14:30 every day
	if err != nil {
		t.Fatalf("ParseSchedule failed: %v", err)
	}

	// Test at 14:00 — next should be 14:30 today
	now := time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC)
	next := s.Next(now)

	expected := time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestParseScheduleStepValue(t *testing.T) {
	s, err := ParseSchedule("*/15 * * * *") // every 15 minutes
	if err != nil {
		t.Fatalf("ParseSchedule failed: %v", err)
	}

	now := time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC)
	next := s.Next(now)

	expected := time.Date(2026, 1, 15, 14, 15, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestParseScheduleRange(t *testing.T) {
	s, err := ParseSchedule("0 9-17 * * *") // every hour from 9 to 17
	if err != nil {
		t.Fatalf("ParseSchedule failed: %v", err)
	}

	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	next := s.Next(now)

	expected := time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestParseScheduleInvalid(t *testing.T) {
	tests := []struct {
		expr string
	}{
		{"* * *"},          // too few fields
		{"* * * * * *"},    // too many fields
		{"60 * * * *"},     // minute out of range
		{"* 24 * * *"},     // hour out of range
		{"* * * invalid *"}, // non-numeric
	}

	for _, tc := range tests {
		_, err := ParseSchedule(tc.expr)
		if err == nil {
			t.Errorf("expected error for %q, got nil", tc.expr)
		}
	}
}

func TestParseScheduleWeekday(t *testing.T) {
	// 0 9 * * 1 — 9:00 AM every Monday
	s, err := ParseSchedule("0 9 * * 1")
	if err != nil {
		t.Fatalf("ParseSchedule failed: %v", err)
	}

	// Monday Jan 19, 2026
	monday := time.Date(2026, 1, 19, 9, 0, 0, 0, time.UTC)
	if !s.matches(monday) {
		t.Error("expected Monday 9:00 to match")
	}

	// Tuesday Jan 20, 2026
	tuesday := time.Date(2026, 1, 20, 9, 0, 0, 0, time.UTC)
	if s.matches(tuesday) {
		t.Error("expected Tuesday 9:00 to NOT match")
	}
}

func TestSchedulerCreateAndList(t *testing.T) {
	repo := &mockCronRepo{}
	s := NewScheduler(repo)

	ctx := context.Background()
	job := domain.CronJob{
		Name:     "test-job",
		Schedule: "0 2 * * *",
		Task:     "run backup",
		Enabled:  true,
	}

	id, err := s.CreateJob(ctx, job)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty job ID")
	}

	jobs, err := s.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "test-job" {
		t.Errorf("expected job name 'test-job', got %q", jobs[0].Name)
	}
}

func TestSchedulerPauseResumeRemove(t *testing.T) {
	repo := &mockCronRepo{}
	s := NewScheduler(repo)

	ctx := context.Background()
	job := domain.CronJob{
		Name:     "test-job",
		Schedule: "0 2 * * *",
		Task:     "run backup",
		Enabled:  true,
	}

	id, _ := s.CreateJob(ctx, job)

	// Pause
	err := s.PauseJob(ctx, id)
	if err != nil {
		t.Fatalf("PauseJob failed: %v", err)
	}

	jobs, _ := s.ListJobs(ctx)
	if len(jobs) == 1 && jobs[0].Enabled {
		t.Error("expected job to be paused after PauseJob")
	}

	// Resume
	err = s.ResumeJob(ctx, id)
	if err != nil {
		t.Fatalf("ResumeJob failed: %v", err)
	}

	jobs, _ = s.ListJobs(ctx)
	if len(jobs) == 1 && !jobs[0].Enabled {
		t.Error("expected job to be enabled after ResumeJob")
	}

	// Remove
	err = s.RemoveJob(ctx, id)
	if err != nil {
		t.Fatalf("RemoveJob failed: %v", err)
	}

	jobs, _ = s.ListJobs(ctx)
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs after remove, got %d", len(jobs))
	}
}

func TestDeliveryTerminal(t *testing.T) {
	d := NewDeliveryService()
	job := domain.CronJob{
		Name:      "test-job",
		Schedule:  "0 2 * * *",
		Task:      "run backup",
		DeliverTo: "terminal",
	}

	result := domain.ToolResult{
		Success: true,
		Output:  "Backup completed",
	}

	// Should not panic
	d.Deliver(context.Background(), job, result)
}

func TestContains(t *testing.T) {
	if !contains([]int{1, 2, 3}, 2) {
		t.Error("expected contains to return true")
	}
	if contains([]int{1, 2, 3}, 4) {
		t.Error("expected contains to return false")
	}
	if contains([]int{}, 1) {
		t.Error("expected contains on empty slice to return false")
	}
}

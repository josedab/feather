package compute

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// JobStatus represents the status of a materialization job.
type JobStatus string

const (
	// JobStatusIdle means the job is waiting for its next run.
	JobStatusIdle JobStatus = "idle"
	// JobStatusRunning means the job is currently executing.
	JobStatusRunning JobStatus = "running"
	// JobStatusPaused means the job has been paused.
	JobStatusPaused JobStatus = "paused"
	// JobStatusFailed means the job failed on its last run.
	JobStatusFailed JobStatus = "failed"
)

// MaterializationJob represents a scheduled feature computation job.
type MaterializationJob struct {
	Name         string    `json:"name"`
	Feature      string    `json:"feature"`
	Schedule     string    `json:"schedule"`
	EntitySource string    `json:"entity_source"`
	Status       JobStatus `json:"status"`
	LastRun      time.Time `json:"last_run"`
	NextRun      time.Time `json:"next_run"`
	RunCount     int64     `json:"run_count"`
	ErrorCount   int64     `json:"error_count"`
	LastError    string    `json:"last_error,omitempty"`
}

// MaterializationScheduler manages scheduled feature computations.
type MaterializationScheduler struct {
	jobs   map[string]*MaterializationJob
	engine *ComputeEngine
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMaterializationScheduler creates a new scheduler.
func NewMaterializationScheduler(engine *ComputeEngine) *MaterializationScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &MaterializationScheduler{
		jobs:   make(map[string]*MaterializationJob),
		engine: engine,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Schedule registers a new materialization job.
func (s *MaterializationScheduler) Schedule(ctx context.Context, job *MaterializationJob) error {
	if job.Name == "" {
		return fmt.Errorf("scheduling job: name is required")
	}
	if job.Feature == "" {
		return fmt.Errorf("scheduling job %s: feature is required", job.Name)
	}
	if job.Schedule == "" {
		return fmt.Errorf("scheduling job %s: schedule is required", job.Name)
	}

	interval, err := parseSimpleSchedule(job.Schedule)
	if err != nil {
		return fmt.Errorf("scheduling job %s: %w", job.Name, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.Name]; exists {
		return fmt.Errorf("scheduling job %s: job already exists", job.Name)
	}

	job.Status = JobStatusIdle
	job.NextRun = time.Now().Add(interval)
	s.jobs[job.Name] = job

	// Start the job ticker
	s.wg.Add(1)
	go s.runJobLoop(job.Name, interval)

	return nil
}

// Unschedule removes a materialization job.
func (s *MaterializationScheduler) Unschedule(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[name]; !exists {
		return fmt.Errorf("unscheduling job %s: job not found", name)
	}

	delete(s.jobs, name)
	return nil
}

// Pause pauses a materialization job.
func (s *MaterializationScheduler) Pause(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("pausing job %s: job not found", name)
	}

	job.Status = JobStatusPaused
	return nil
}

// Resume resumes a paused materialization job.
func (s *MaterializationScheduler) Resume(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("resuming job %s: job not found", name)
	}

	if job.Status != JobStatusPaused {
		return fmt.Errorf("resuming job %s: job is not paused (status: %s)", name, job.Status)
	}

	job.Status = JobStatusIdle
	return nil
}

// ListJobs returns all registered jobs.
func (s *MaterializationScheduler) ListJobs(ctx context.Context) []*MaterializationJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MaterializationJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		result = append(result, job)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// GetJob retrieves a job by name.
func (s *MaterializationScheduler) GetJob(ctx context.Context, name string) (*MaterializationJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[name]
	if !exists {
		return nil, fmt.Errorf("getting job %s: job not found", name)
	}

	return job, nil
}

// TriggerNow immediately triggers a job execution.
func (s *MaterializationScheduler) TriggerNow(ctx context.Context, name string) error {
	s.mu.RLock()
	job, exists := s.jobs[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("triggering job %s: job not found", name)
	}

	s.executeJob(job)
	return nil
}

// Close shuts down the scheduler and waits for all jobs to complete.
func (s *MaterializationScheduler) Close() error {
	s.cancel()
	s.wg.Wait()
	return nil
}

func (s *MaterializationScheduler) runJobLoop(name string, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			job, exists := s.jobs[name]
			s.mu.RUnlock()

			if !exists {
				return
			}

			if job.Status == JobStatusPaused {
				continue
			}

			s.executeJob(job)
		}
	}
}

func (s *MaterializationScheduler) executeJob(job *MaterializationJob) {
	s.mu.Lock()
	job.Status = JobStatusRunning
	s.mu.Unlock()

	// Create empty inputs for the computation (in a real system these
	// would come from the entity source).
	inputs := make(map[string]interface{})

	_, err := s.engine.Compute(s.ctx, job.Feature, inputs)

	s.mu.Lock()
	defer s.mu.Unlock()

	job.LastRun = time.Now()
	job.RunCount++

	if err != nil {
		job.ErrorCount++
		job.LastError = err.Error()
		job.Status = JobStatusFailed
	} else {
		job.LastError = ""
		job.Status = JobStatusIdle
	}
}

// parseSimpleSchedule parses a simple schedule string into a duration.
// Supports formats like "@every 5m", "@every 1h", or raw durations "5m", "1h".
func parseSimpleSchedule(schedule string) (time.Duration, error) {
	s := schedule
	if len(s) > 7 && s[:7] == "@every " {
		s = s[7:]
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid schedule %q: %w", schedule, err)
	}

	if d <= 0 {
		return 0, fmt.Errorf("invalid schedule %q: duration must be positive", schedule)
	}

	return d, nil
}

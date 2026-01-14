package warehouse

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CronScheduler manages cron-based sync job scheduling.
type CronScheduler struct {
	mu       sync.RWMutex
	entries  map[string]*ScheduleEntry
	engine   *SyncEngine
	logger   *slog.Logger
	location *time.Location

	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// ScheduleEntry represents a scheduled sync job.
type ScheduleEntry struct {
	JobID         string
	ConnectorName string
	Schedule      *CronExpression
	NextRun       time.Time
	LastRun       time.Time
	Enabled       bool
	RetryCount    int
	MaxRetries    int
	LastError     string
}

// CronExpression represents a parsed cron expression.
type CronExpression struct {
	raw      string
	minute   []int         // 0-59
	hour     []int         // 0-23
	day      []int         // 1-31
	month    []int         // 1-12
	weekday  []int         // 0-6 (Sunday = 0)
	interval time.Duration // For @every expressions
}

// NewCronScheduler creates a new cron scheduler.
func NewCronScheduler(engine *SyncEngine, logger *slog.Logger) *CronScheduler {
	if logger == nil {
		logger = slog.Default()
	}

	return &CronScheduler{
		entries:  make(map[string]*ScheduleEntry),
		engine:   engine,
		logger:   logger,
		location: time.Local,
	}
}

// SetLocation sets the timezone for schedule evaluation.
func (s *CronScheduler) SetLocation(loc *time.Location) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.location = loc
}

// Schedule adds or updates a scheduled job.
func (s *CronScheduler) Schedule(jobID, connectorName, cronExpr string, maxRetries int) error {
	cron, err := ParseCronExpression(cronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().In(s.location)
	entry := &ScheduleEntry{
		JobID:         jobID,
		ConnectorName: connectorName,
		Schedule:      cron,
		NextRun:       cron.Next(now),
		Enabled:       true,
		MaxRetries:    maxRetries,
	}

	s.entries[jobID] = entry
	s.logger.Info("scheduled job",
		"job_id", jobID,
		"connector", connectorName,
		"schedule", cronExpr,
		"next_run", entry.NextRun,
	)

	return nil
}

// Unschedule removes a scheduled job.
func (s *CronScheduler) Unschedule(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[jobID]; !exists {
		return fmt.Errorf("job %s not scheduled", jobID)
	}

	delete(s.entries, jobID)
	s.logger.Info("unscheduled job", "job_id", jobID)

	return nil
}

// Enable enables a scheduled job.
func (s *CronScheduler) Enable(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[jobID]
	if !exists {
		return fmt.Errorf("job %s not scheduled", jobID)
	}

	entry.Enabled = true
	entry.NextRun = entry.Schedule.Next(time.Now().In(s.location))
	return nil
}

// Disable disables a scheduled job.
func (s *CronScheduler) Disable(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[jobID]
	if !exists {
		return fmt.Errorf("job %s not scheduled", jobID)
	}

	entry.Enabled = false
	return nil
}

// GetEntry returns a schedule entry.
func (s *CronScheduler) GetEntry(jobID string) (*ScheduleEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not scheduled", jobID)
	}

	// Return a copy
	entryCopy := *entry
	return &entryCopy, nil
}

// ListEntries returns all schedule entries.
func (s *CronScheduler) ListEntries() []*ScheduleEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*ScheduleEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		entryCopy := *entry
		entries = append(entries, &entryCopy)
	}

	// Sort by next run time
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].NextRun.Before(entries[j].NextRun)
	})

	return entries
}

// Start starts the scheduler.
func (s *CronScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run(s.ctx)

	s.logger.Info("cron scheduler started")
	return nil
}

// Stop stops the scheduler.
func (s *CronScheduler) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}

	s.cancel()
	s.running = false
	s.mu.Unlock()

	s.wg.Wait()
	s.logger.Info("cron scheduler stopped")
	return nil
}

// IsRunning returns whether the scheduler is running.
func (s *CronScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *CronScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.checkAndRun(ctx, now.In(s.location))
		}
	}
}

func (s *CronScheduler) checkAndRun(ctx context.Context, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for jobID, entry := range s.entries {
		if !entry.Enabled {
			continue
		}

		if now.After(entry.NextRun) || now.Equal(entry.NextRun) {
			// Schedule next run immediately to prevent duplicate runs
			entry.LastRun = now
			entry.NextRun = entry.Schedule.Next(now)

			// Run in background
			go s.executeJob(ctx, jobID, entry.ConnectorName)
		}
	}
}

func (s *CronScheduler) executeJob(ctx context.Context, jobID, connectorName string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	execution, err := s.engine.ExecuteJob(ctx, jobID, connectorName)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[jobID]
	if !exists {
		return
	}

	if err != nil {
		entry.RetryCount++
		entry.LastError = err.Error()

		s.logger.Error("scheduled job failed",
			"job_id", jobID,
			"error", err,
			"retry_count", entry.RetryCount,
		)

		// Disable if max retries exceeded
		if entry.MaxRetries > 0 && entry.RetryCount >= entry.MaxRetries {
			entry.Enabled = false
			s.logger.Warn("job disabled after max retries",
				"job_id", jobID,
				"retries", entry.RetryCount,
			)
		}
	} else {
		entry.RetryCount = 0
		entry.LastError = ""

		s.logger.Info("scheduled job completed",
			"job_id", jobID,
			"execution_id", execution.ID,
			"rows_synced", execution.RowsSynced,
		)
	}
}

// TriggerNow immediately executes a scheduled job.
func (s *CronScheduler) TriggerNow(ctx context.Context, jobID string) error {
	s.mu.RLock()
	entry, exists := s.entries[jobID]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("job %s not scheduled", jobID)
	}
	connectorName := entry.ConnectorName
	s.mu.RUnlock()

	go s.executeJob(ctx, jobID, connectorName)
	return nil
}

// ParseCronExpression parses a cron expression string.
// Supports:
// - Standard 5-field cron: "minute hour day month weekday"
// - Special expressions: @yearly, @monthly, @weekly, @daily, @hourly
// - Interval expressions: @every 1h30m
func ParseCronExpression(expr string) (*CronExpression, error) {
	expr = strings.TrimSpace(expr)

	// Handle special expressions
	if strings.HasPrefix(expr, "@") {
		return parseSpecialExpression(expr)
	}

	// Parse standard 5-field cron
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	cron := &CronExpression{raw: expr}
	var err error

	cron.minute, err = parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}

	cron.hour, err = parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}

	cron.day, err = parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day field: %w", err)
	}

	cron.month, err = parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}

	cron.weekday, err = parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid weekday field: %w", err)
	}

	return cron, nil
}

func parseSpecialExpression(expr string) (*CronExpression, error) {
	switch expr {
	case "@yearly", "@annually":
		return &CronExpression{
			raw:     expr,
			minute:  []int{0},
			hour:    []int{0},
			day:     []int{1},
			month:   []int{1},
			weekday: allValues(0, 6),
		}, nil
	case "@monthly":
		return &CronExpression{
			raw:     expr,
			minute:  []int{0},
			hour:    []int{0},
			day:     []int{1},
			month:   allValues(1, 12),
			weekday: allValues(0, 6),
		}, nil
	case "@weekly":
		return &CronExpression{
			raw:     expr,
			minute:  []int{0},
			hour:    []int{0},
			day:     allValues(1, 31),
			month:   allValues(1, 12),
			weekday: []int{0},
		}, nil
	case "@daily", "@midnight":
		return &CronExpression{
			raw:     expr,
			minute:  []int{0},
			hour:    []int{0},
			day:     allValues(1, 31),
			month:   allValues(1, 12),
			weekday: allValues(0, 6),
		}, nil
	case "@hourly":
		return &CronExpression{
			raw:     expr,
			minute:  []int{0},
			hour:    allValues(0, 23),
			day:     allValues(1, 31),
			month:   allValues(1, 12),
			weekday: allValues(0, 6),
		}, nil
	default:
		if strings.HasPrefix(expr, "@every ") {
			durStr := strings.TrimPrefix(expr, "@every ")
			dur, err := time.ParseDuration(durStr)
			if err != nil {
				return nil, fmt.Errorf("invalid duration: %w", err)
			}
			if dur < time.Minute {
				return nil, fmt.Errorf("interval must be at least 1 minute")
			}
			return &CronExpression{
				raw:      expr,
				interval: dur,
			}, nil
		}
		return nil, fmt.Errorf("unknown special expression: %s", expr)
	}
}

func parseField(field string, minValue, maxValue int) ([]int, error) {
	if field == "*" {
		return allValues(minValue, maxValue), nil
	}

	var values []int

	// Handle comma-separated values
	parts := strings.Split(field, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Handle step values (*/5 or 1-10/2)
		step := 1
		if idx := strings.Index(part, "/"); idx != -1 {
			stepStr := part[idx+1:]
			var err error
			step, err = strconv.Atoi(stepStr)
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step: %s", stepStr)
			}
			part = part[:idx]
		}

		// Handle ranges (1-10)
		if idx := strings.Index(part, "-"); idx != -1 {
			startStr := part[:idx]
			endStr := part[idx+1:]
			start, err := strconv.Atoi(startStr)
			if err != nil {
				return nil, fmt.Errorf("invalid start: %s", startStr)
			}
			end, err := strconv.Atoi(endStr)
			if err != nil {
				return nil, fmt.Errorf("invalid end: %s", endStr)
			}
			if start < minValue || end > maxValue || start > end {
				return nil, fmt.Errorf("range out of bounds: %d-%d", start, end)
			}
			for i := start; i <= end; i += step {
				values = append(values, i)
			}
		} else if part == "*" {
			for i := minValue; i <= maxValue; i += step {
				values = append(values, i)
			}
		} else {
			val, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			if val < minValue || val > maxValue {
				return nil, fmt.Errorf("value out of bounds: %d", val)
			}
			values = append(values, val)
		}
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no valid values")
	}

	// Sort and deduplicate
	sort.Ints(values)
	unique := values[:1]
	for i := 1; i < len(values); i++ {
		if values[i] != values[i-1] {
			unique = append(unique, values[i])
		}
	}

	return unique, nil
}

func allValues(minValue, maxValue int) []int {
	values := make([]int, maxValue-minValue+1)
	for i := range values {
		values[i] = minValue + i
	}
	return values
}

// Next returns the next execution time after the given time.
func (c *CronExpression) Next(after time.Time) time.Time {
	// Handle interval expressions
	if c.interval > 0 {
		// Round up to the next interval
		ns := after.UnixNano()
		intervalNs := c.interval.Nanoseconds()
		nextNs := ((ns / intervalNs) + 1) * intervalNs
		return time.Unix(0, nextNs)
	}

	// Start from the next minute
	t := after.Add(time.Minute)
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())

	// Search for the next matching time (max 4 years to prevent infinite loops)
	maxTime := after.Add(4 * 365 * 24 * time.Hour)

	for t.Before(maxTime) {
		// Check month
		if !contains(c.month, int(t.Month())) {
			// Advance to first day of next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		// Check day and weekday
		if !contains(c.day, t.Day()) || !contains(c.weekday, int(t.Weekday())) {
			// Advance to next day
			t = t.Add(24 * time.Hour)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			continue
		}

		// Check hour
		if !contains(c.hour, t.Hour()) {
			// Advance to next hour
			t = t.Add(time.Hour)
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
			continue
		}

		// Check minute
		if !contains(c.minute, t.Minute()) {
			// Advance to next minute
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	// Return far future if no match found
	return maxTime
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// String returns the original cron expression.
func (c *CronExpression) String() string {
	return c.raw
}

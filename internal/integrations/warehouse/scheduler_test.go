package warehouse

import (
	"context"
	"testing"
	"time"
)

func TestParseCronExpression_Standard(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"every minute", "* * * * *", false},
		{"every hour", "0 * * * *", false},
		{"specific time", "30 14 * * *", false},
		{"weekday only", "0 9 * * 1-5", false},
		{"monthly", "0 0 1 * *", false},
		{"with step", "*/15 * * * *", false},
		{"range with step", "0-30/10 * * * *", false},
		{"comma separated", "0,15,30,45 * * * *", false},
		{"complex", "0 */2 1,15 * 1-5", false},
		{"invalid fields", "* * *", true},
		{"invalid minute", "60 * * * *", true},
		{"invalid hour", "* 25 * * *", true},
		{"invalid day", "* * 32 * *", true},
		{"invalid month", "* * * 13 *", true},
		{"invalid weekday", "* * * * 8", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCronExpression(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCronExpression(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestParseCronExpression_Special(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"yearly", "@yearly", false},
		{"annually", "@annually", false},
		{"monthly", "@monthly", false},
		{"weekly", "@weekly", false},
		{"daily", "@daily", false},
		{"midnight", "@midnight", false},
		{"hourly", "@hourly", false},
		{"every 1h", "@every 1h", false},
		{"every 30m", "@every 30m", false},
		{"every 2h30m", "@every 2h30m", false},
		{"every 30s", "@every 30s", true}, // Too short
		{"invalid special", "@invalid", true},
		{"invalid duration", "@every abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCronExpression(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCronExpression(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestCronExpression_Next(t *testing.T) {
	// Test from a fixed time
	base := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expr     string
		expected time.Time
	}{
		{
			"next minute",
			"* * * * *",
			time.Date(2024, 6, 15, 10, 31, 0, 0, time.UTC),
		},
		{
			"next hour at :00",
			"0 * * * *",
			time.Date(2024, 6, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			"specific hour today",
			"0 14 * * *",
			time.Date(2024, 6, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			"specific hour tomorrow",
			"0 9 * * *",
			time.Date(2024, 6, 16, 9, 0, 0, 0, time.UTC),
		},
		{
			"every 15 minutes",
			"*/15 * * * *",
			time.Date(2024, 6, 15, 10, 45, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cron, err := ParseCronExpression(tt.expr)
			if err != nil {
				t.Fatalf("ParseCronExpression(%q) error = %v", tt.expr, err)
			}

			got := cron.Next(base)
			if !got.Equal(tt.expected) {
				t.Errorf("Next() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCronExpression_Next_Interval(t *testing.T) {
	cron, err := ParseCronExpression("@every 1h")
	if err != nil {
		t.Fatalf("ParseCronExpression error = %v", err)
	}

	base := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	next := cron.Next(base)

	// Should be at 11:00 (next full hour)
	expected := time.Date(2024, 6, 15, 11, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("Next() = %v, want %v", next, expected)
	}
}

func TestCronExpression_String(t *testing.T) {
	expr := "*/15 * * * *"
	cron, err := ParseCronExpression(expr)
	if err != nil {
		t.Fatalf("ParseCronExpression error = %v", err)
	}

	if cron.String() != expr {
		t.Errorf("String() = %q, want %q", cron.String(), expr)
	}
}

func TestParseField(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		min      int
		max      int
		expected []int
		wantErr  bool
	}{
		{"wildcard", "*", 0, 5, []int{0, 1, 2, 3, 4, 5}, false},
		{"single value", "3", 0, 5, []int{3}, false},
		{"comma list", "1,3,5", 0, 5, []int{1, 3, 5}, false},
		{"range", "1-3", 0, 5, []int{1, 2, 3}, false},
		{"step", "*/2", 0, 5, []int{0, 2, 4}, false},
		{"range with step", "0-4/2", 0, 5, []int{0, 2, 4}, false},
		{"out of range", "10", 0, 5, nil, true},
		{"invalid range", "5-2", 0, 5, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseField(tt.field, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseField(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !sliceEqual(got, tt.expected) {
				t.Errorf("parseField(%q) = %v, want %v", tt.field, got, tt.expected)
			}
		})
	}
}

func TestCronScheduler_ScheduleUnschedule(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)

	// Schedule a job
	err := scheduler.Schedule("job-1", "connector-1", "*/5 * * * *", 3)
	if err != nil {
		t.Fatalf("Schedule error = %v", err)
	}

	// Verify it was scheduled
	entries := scheduler.ListEntries()
	if len(entries) != 1 {
		t.Errorf("ListEntries() len = %d, want 1", len(entries))
	}

	// Get entry
	entry, err := scheduler.GetEntry("job-1")
	if err != nil {
		t.Fatalf("GetEntry error = %v", err)
	}
	if entry.JobID != "job-1" {
		t.Errorf("entry.JobID = %q, want %q", entry.JobID, "job-1")
	}
	if !entry.Enabled {
		t.Error("entry should be enabled")
	}

	// Unschedule
	err = scheduler.Unschedule("job-1")
	if err != nil {
		t.Fatalf("Unschedule error = %v", err)
	}

	// Verify it was removed
	entries = scheduler.ListEntries()
	if len(entries) != 0 {
		t.Errorf("ListEntries() len = %d, want 0", len(entries))
	}

	// Unschedule nonexistent should error
	err = scheduler.Unschedule("job-1")
	if err == nil {
		t.Error("expected error unscheduling nonexistent job")
	}
}

func TestCronScheduler_EnableDisable(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)

	err := scheduler.Schedule("job-1", "connector-1", "@hourly", 3)
	if err != nil {
		t.Fatalf("Schedule error = %v", err)
	}

	// Disable
	err = scheduler.Disable("job-1")
	if err != nil {
		t.Fatalf("Disable error = %v", err)
	}

	entry, _ := scheduler.GetEntry("job-1")
	if entry.Enabled {
		t.Error("entry should be disabled")
	}

	// Enable
	err = scheduler.Enable("job-1")
	if err != nil {
		t.Fatalf("Enable error = %v", err)
	}

	entry, _ = scheduler.GetEntry("job-1")
	if !entry.Enabled {
		t.Error("entry should be enabled")
	}
}

func TestCronScheduler_StartStop(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running initially")
	}

	err := scheduler.Start(context.Background())
	if err != nil {
		t.Fatalf("Start error = %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start")
	}

	// Starting again should error
	err = scheduler.Start(context.Background())
	if err == nil {
		t.Error("expected error starting already running scheduler")
	}

	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Stop error = %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after Stop")
	}
}

func TestCronScheduler_SetLocation(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)

	loc, _ := time.LoadLocation("America/New_York")
	scheduler.SetLocation(loc)

	err := scheduler.Schedule("job-1", "connector-1", "0 9 * * *", 0)
	if err != nil {
		t.Fatalf("Schedule error = %v", err)
	}

	entry, _ := scheduler.GetEntry("job-1")
	// NextRun should be in the NY timezone
	if entry.NextRun.Location() != loc {
		t.Errorf("NextRun location = %v, want %v", entry.NextRun.Location(), loc)
	}
}

func TestCronScheduler_InvalidSchedule(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)

	err := scheduler.Schedule("job-1", "connector-1", "invalid", 0)
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestCronScheduler_MultipleJobs(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)

	// Schedule multiple jobs
	for i := 1; i <= 5; i++ {
		err := scheduler.Schedule(
			"job-"+string(rune('0'+i)),
			"connector-1",
			"*/5 * * * *",
			3,
		)
		if err != nil {
			t.Fatalf("Schedule job-%d error = %v", i, err)
		}
	}

	entries := scheduler.ListEntries()
	if len(entries) != 5 {
		t.Errorf("ListEntries() len = %d, want 5", len(entries))
	}

	// Entries should be sorted by next run time
	for i := 1; i < len(entries); i++ {
		if entries[i].NextRun.Before(entries[i-1].NextRun) {
			t.Error("entries should be sorted by NextRun")
		}
	}
}

func sliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

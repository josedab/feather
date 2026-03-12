package backfillengine

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockWriter implements FeatureWriter for testing.
type mockWriter struct {
	written []writeRecord
	flushed int
}

type writeRecord struct {
	entityKey   string
	featureName string
	value       interface{}
	timestamp   time.Time
}

func (w *mockWriter) WriteFeature(_ context.Context, entityKey, featureName string, value interface{}, ts time.Time) error {
	w.written = append(w.written, writeRecord{entityKey, featureName, value, ts})
	return nil
}

func (w *mockWriter) Flush(_ context.Context) error {
	w.flushed++
	return nil
}

// mockSource implements Source for testing.
type mockSource struct {
	events    []Event
	connected bool
}

func (s *mockSource) Type() SourceType                                   { return SourceTypeFile }
func (s *mockSource) Connect(_ context.Context) error                    { s.connected = true; return nil }
func (s *mockSource) Close() error                                       { s.connected = false; return nil }
func (s *mockSource) SeekToTimestamp(_ context.Context, _ time.Time) (int64, error) { return 0, nil }
func (s *mockSource) LatestOffset(_ context.Context) (int64, error)      { return int64(len(s.events)), nil }

func (s *mockSource) ReadBatch(_ context.Context, fromOffset int64, batchSize int) ([]Event, error) {
	if fromOffset >= int64(len(s.events)) {
		return nil, nil
	}
	end := fromOffset + int64(batchSize)
	if end > int64(len(s.events)) {
		end = int64(len(s.events))
	}
	return s.events[fromOffset:end], nil
}

func TestCoordinatorCreateJob(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	src := &mockSource{}
	if err := coord.RegisterSource("test-src", src); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	job, err := coord.CreateJob(JobRequest{
		SourceName: "test-src",
		StartTime:  now.Add(-time.Hour),
		EndTime:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != JobStatusPending {
		t.Errorf("expected pending, got %s", job.Status)
	}
	if job.BatchSize != DefaultCoordinatorConfig().DefaultBatchSize {
		t.Errorf("expected default batch size")
	}
}

func TestCoordinatorInvalidTimeRange(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	src := &mockSource{}
	_ = coord.RegisterSource("test-src", src)

	now := time.Now()
	_, err := coord.CreateJob(JobRequest{
		SourceName: "test-src",
		StartTime:  now,
		EndTime:    now.Add(-time.Hour),
	})
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}
}

func TestCoordinatorSourceNotFound(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	_, err := coord.CreateJob(JobRequest{
		SourceName: "nonexistent",
		StartTime:  time.Now().Add(-time.Hour),
		EndTime:    time.Now(),
	})
	if err != ErrSourceNotFound {
		t.Errorf("expected ErrSourceNotFound, got %v", err)
	}
}

func TestCoordinatorDuplicateSource(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	src := &mockSource{}
	_ = coord.RegisterSource("test-src", src)
	err := coord.RegisterSource("test-src", src)
	if err != ErrSourceExists {
		t.Errorf("expected ErrSourceExists, got %v", err)
	}
}

func TestCoordinatorListSources(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	_ = coord.RegisterSource("kafka", NewKafkaSource(KafkaSourceConfig{}))
	_ = coord.RegisterSource("file", NewFileSource(FileSourceConfig{}))

	sources := coord.ListSources()
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
	if sources["kafka"] != SourceTypeKafka {
		t.Error("expected kafka source type")
	}
}

func TestMaterializerDeduplication(t *testing.T) {
	t.Parallel()
	writer := &mockWriter{}
	mat := NewMaterializer(writer)

	now := time.Now()
	events := []Event{
		{ID: "e1", Source: "test", EntityKey: "user:1", Features: map[string]interface{}{"clicks": 10}, Timestamp: now},
		{ID: "e1", Source: "test", EntityKey: "user:1", Features: map[string]interface{}{"clicks": 10}, Timestamp: now},
	}

	if err := mat.Materialize(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	stats := mat.Stats()
	if stats.EventsMaterialized != 1 {
		t.Errorf("expected 1 materialized, got %d", stats.EventsMaterialized)
	}
	if stats.DuplicatesSkipped != 1 {
		t.Errorf("expected 1 duplicate, got %d", stats.DuplicatesSkipped)
	}
}

func TestMaterializerReset(t *testing.T) {
	t.Parallel()
	writer := &mockWriter{}
	mat := NewMaterializer(writer)

	events := []Event{
		{ID: "e1", Source: "s", EntityKey: "u:1", Features: map[string]interface{}{"x": 1}, Timestamp: time.Now()},
	}
	_ = mat.Materialize(context.Background(), events)
	mat.Reset()

	stats := mat.Stats()
	if stats.EventsMaterialized != 0 {
		t.Errorf("expected 0 after reset, got %d", stats.EventsMaterialized)
	}
}

func TestCoordinatorJobLifecycle(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	now := time.Now()
	events := make([]Event, 5)
	for i := range events {
		events[i] = Event{
			ID:        fmt.Sprintf("e%d", i),
			Source:    "test",
			EntityKey: fmt.Sprintf("user:%d", i),
			Features:  map[string]interface{}{"count": i},
			Timestamp: now.Add(-time.Duration(5-i) * time.Minute),
			Offset:    int64(i),
		}
	}
	src := &mockSource{events: events}
	_ = coord.RegisterSource("test-src", src)

	job, _ := coord.CreateJob(JobRequest{
		SourceName: "test-src",
		StartTime:  now.Add(-10 * time.Minute),
		EndTime:    now,
		BatchSize:  10,
	})

	writer := &mockWriter{}
	if err := coord.StartJob(job.ID, writer); err != nil {
		t.Fatal(err)
	}

	// Wait for job to complete.
	deadline := time.After(5 * time.Second)
	for {
		j, _ := coord.GetJob(job.ID)
		if j.Status == JobStatusCompleted || j.Status == JobStatusFailed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("job did not complete in time")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	j, _ := coord.GetJob(job.ID)
	if j.Status != JobStatusCompleted {
		t.Errorf("expected completed, got %s (error: %s)", j.Status, j.LastError)
	}
	if j.EventsProcessed != 5 {
		t.Errorf("expected 5 events processed, got %d", j.EventsProcessed)
	}
}

func TestCoordinatorCancelJob(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	src := &mockSource{}
	_ = coord.RegisterSource("test-src", src)

	job, _ := coord.CreateJob(JobRequest{
		SourceName: "test-src",
		StartTime:  time.Now().Add(-time.Hour),
		EndTime:    time.Now(),
	})

	if err := coord.CancelJob(job.ID); err != nil {
		t.Fatal(err)
	}

	j, _ := coord.GetJob(job.ID)
	if j.Status != JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", j.Status)
	}
}

// failingSource simulates transient failures for retry testing.
type failingSource struct {
	events      []Event
	connected   bool
	failCount   int
	readCalls   int
}

func (s *failingSource) Type() SourceType { return SourceTypeFile }
func (s *failingSource) Connect(_ context.Context) error {
	s.connected = true
	return nil
}
func (s *failingSource) Close() error { s.connected = false; return nil }
func (s *failingSource) SeekToTimestamp(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *failingSource) LatestOffset(_ context.Context) (int64, error) {
	return int64(len(s.events)), nil
}
func (s *failingSource) ReadBatch(_ context.Context, fromOffset int64, batchSize int) ([]Event, error) {
	s.readCalls++
	if s.readCalls <= s.failCount {
		return nil, fmt.Errorf("transient error %d", s.readCalls)
	}
	if fromOffset >= int64(len(s.events)) {
		return nil, nil
	}
	end := fromOffset + int64(batchSize)
	if end > int64(len(s.events)) {
		end = int64(len(s.events))
	}
	return s.events[fromOffset:end], nil
}

func TestCoordinatorRetryOnTransientFailure(t *testing.T) {
	t.Parallel()
	cfg := DefaultCoordinatorConfig()
	cfg.RetryDelay = time.Millisecond // speed up test
	coord := NewCoordinator(cfg)

	now := time.Now()
	events := []Event{
		{ID: "e1", Source: "test", EntityKey: "u:1", Features: map[string]interface{}{"x": 1}, Timestamp: now.Add(-time.Minute), Offset: 0},
	}
	src := &failingSource{events: events, failCount: 2}
	_ = coord.RegisterSource("test-src", src)

	job, _ := coord.CreateJob(JobRequest{
		SourceName: "test-src",
		StartTime:  now.Add(-10 * time.Minute),
		EndTime:    now,
		BatchSize:  10,
	})

	writer := &mockWriter{}
	_ = coord.StartJob(job.ID, writer)

	deadline := time.After(5 * time.Second)
	for {
		j, _ := coord.GetJob(job.ID)
		if j.Status == JobStatusCompleted || j.Status == JobStatusFailed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("job did not complete in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	j, _ := coord.GetJob(job.ID)
	if j.Status != JobStatusCompleted {
		t.Errorf("expected completed after retries, got %s (error: %s)", j.Status, j.LastError)
	}
	if j.EventsProcessed != 1 {
		t.Errorf("expected 1 event, got %d", j.EventsProcessed)
	}
}

func TestJobProgress(t *testing.T) {
	t.Parallel()
	now := time.Now()
	job := &Job{
		StartTime: now.Add(-10 * time.Minute),
		EndTime:   now,
		Status:    JobStatusRunning,
	}

	if job.Progress() != 0 {
		t.Errorf("expected 0 progress without watermark, got %f", job.Progress())
	}

	mid := now.Add(-5 * time.Minute)
	job.Watermark = &mid
	pct := job.Progress()
	if pct < 49 || pct > 51 {
		t.Errorf("expected ~50%% progress at midpoint, got %f", pct)
	}

	job.Status = JobStatusCompleted
	if job.Progress() != 100 {
		t.Errorf("expected 100%% for completed job")
	}
}

func TestMaterializerDedupEviction(t *testing.T) {
	t.Parallel()
	writer := &mockWriter{}
	mat := NewMaterializer(writer)
	mat.maxDedupSize = 10 // small for testing

	now := time.Now()
	for i := 0; i < 20; i++ {
		events := []Event{
			{ID: fmt.Sprintf("e%d", i), Source: "s", EntityKey: "u:1", Features: map[string]interface{}{"x": i}, Timestamp: now, Offset: int64(i)},
		}
		if err := mat.Materialize(context.Background(), events); err != nil {
			t.Fatal(err)
		}
	}

	stats := mat.Stats()
	if stats.DedupEvictions == 0 {
		t.Error("expected dedup evictions to occur")
	}
	if stats.EventsMaterialized != 20 {
		t.Errorf("expected 20 materialized, got %d", stats.EventsMaterialized)
	}
}

func TestCoordinatorUnregisterSource(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	src := &mockSource{}
	_ = coord.RegisterSource("test-src", src)

	if err := coord.UnregisterSource("test-src"); err != nil {
		t.Fatalf("UnregisterSource failed: %v", err)
	}
	sources := coord.ListSources()
	if len(sources) != 0 {
		t.Errorf("expected 0 sources after unregister, got %d", len(sources))
	}

	// Unregistering a nonexistent source should fail.
	if err := coord.UnregisterSource("nonexistent"); err != ErrSourceNotFound {
		t.Errorf("expected ErrSourceNotFound, got %v", err)
	}
}

func TestCoordinatorListJobs(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	src := &mockSource{}
	_ = coord.RegisterSource("test-src", src)

	now := time.Now()
	_, _ = coord.CreateJob(JobRequest{SourceName: "test-src", StartTime: now.Add(-2 * time.Hour), EndTime: now})
	_, _ = coord.CreateJob(JobRequest{SourceName: "test-src", StartTime: now.Add(-time.Hour), EndTime: now})

	jobs := coord.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestCoordinatorPauseJob(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())

	// Use a source with many events so the job doesn't finish immediately.
	now := time.Now()
	events := make([]Event, 100)
	for i := range events {
		events[i] = Event{
			ID: fmt.Sprintf("e%d", i), Source: "s", EntityKey: fmt.Sprintf("u:%d", i),
			Features: map[string]interface{}{"x": i}, Timestamp: now.Add(-time.Duration(100-i) * time.Minute), Offset: int64(i),
		}
	}
	src := &mockSource{events: events}
	_ = coord.RegisterSource("test-src", src)

	job, _ := coord.CreateJob(JobRequest{
		SourceName: "test-src",
		StartTime:  now.Add(-200 * time.Minute),
		EndTime:    now,
		BatchSize:  1, // process one at a time
	})

	// Can't pause a non-running job.
	err := coord.PauseJob(job.ID)
	if err != ErrJobNotRunning {
		t.Errorf("expected ErrJobNotRunning, got %v", err)
	}

	// Start and then pause.
	writer := &mockWriter{}
	_ = coord.StartJob(job.ID, writer)
	time.Sleep(50 * time.Millisecond)

	if err := coord.PauseJob(job.ID); err != nil {
		// Job may have already completed; check status.
		j, _ := coord.GetJob(job.ID)
		if j.Status != JobStatusCompleted {
			t.Fatalf("PauseJob failed: %v (status=%s)", err, j.Status)
		}
		return
	}
	j, _ := coord.GetJob(job.ID)
	if j.Status != JobStatusPaused {
		t.Errorf("expected paused, got %s", j.Status)
	}

	// Pause nonexistent job.
	if err := coord.PauseJob("nonexistent"); err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestCoordinatorGetCheckpoint(t *testing.T) {
	t.Parallel()
	cfg := DefaultCoordinatorConfig()
	cfg.CheckpointEvery = 1
	coord := NewCoordinator(cfg)

	now := time.Now()
	events := []Event{
		{ID: "e1", Source: "s", EntityKey: "u:1", Features: map[string]interface{}{"x": 1}, Timestamp: now.Add(-time.Minute), Offset: 0},
	}
	src := &mockSource{events: events}
	_ = coord.RegisterSource("test-src", src)

	job, _ := coord.CreateJob(JobRequest{
		SourceName: "test-src",
		StartTime:  now.Add(-10 * time.Minute),
		EndTime:    now,
		BatchSize:  10,
	})

	writer := &mockWriter{}
	_ = coord.StartJob(job.ID, writer)

	deadline := time.After(5 * time.Second)
	for {
		j, _ := coord.GetJob(job.ID)
		if j.Status == JobStatusCompleted || j.Status == JobStatusFailed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("job did not complete in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cp, err := coord.GetCheckpoint(job.ID)
	if err != nil {
		t.Fatalf("GetCheckpoint failed: %v", err)
	}
	if cp.EventsProcessed != 1 {
		t.Errorf("expected 1 event processed in checkpoint, got %d", cp.EventsProcessed)
	}

	// Nonexistent checkpoint.
	_, err = coord.GetCheckpoint("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestCoordinatorStats(t *testing.T) {
	t.Parallel()
	coord := NewCoordinator(DefaultCoordinatorConfig())
	src := &mockSource{}
	_ = coord.RegisterSource("test-src", src)

	now := time.Now()
	_, _ = coord.CreateJob(JobRequest{SourceName: "test-src", StartTime: now.Add(-time.Hour), EndTime: now})

	stats := coord.Stats()
	if stats.TotalJobs != 1 {
		t.Errorf("expected TotalJobs=1, got %d", stats.TotalJobs)
	}
	if stats.RunningJobs != 0 {
		t.Errorf("expected RunningJobs=0, got %d", stats.RunningJobs)
	}
}

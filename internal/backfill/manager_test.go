package backfill

import (
	"context"
	"testing"
	"time"
)

// mockWriter implements FeatureWriter for testing
type mockWriter struct {
	writes     []FeatureRecord
	batchCount int
	writeErr   error
}

func (w *mockWriter) WriteFeature(ctx context.Context, entityID string, feature string, value interface{}, timestamp time.Time) error {
	w.writes = append(w.writes, FeatureRecord{
		EntityID:  entityID,
		Feature:   feature,
		Value:     value,
		Timestamp: timestamp,
	})
	return w.writeErr
}

func (w *mockWriter) WriteBatch(ctx context.Context, records []FeatureRecord) error {
	w.batchCount++
	w.writes = append(w.writes, records...)
	return w.writeErr
}

func TestManager_CreateJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	tests := []struct {
		name    string
		job     *Job
		wantErr error
	}{
		{
			name: "valid job",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Features: []string{"feature1", "feature2"},
			},
			wantErr: nil,
		},
		{
			name: "missing ID",
			job: &Job{
				Name:     "Test Job",
				Features: []string{"feature1"},
			},
			wantErr: ErrJobIDRequired,
		},
		{
			name: "missing features",
			job: &Job{
				ID:   "job-2",
				Name: "Test Job",
			},
			wantErr: ErrFeaturesRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.CreateJob(tt.job)
			if err != tt.wantErr {
				t.Errorf("CreateJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_CreateJob_Duplicate(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
	}

	err := m.CreateJob(job)
	if err != nil {
		t.Fatalf("First CreateJob() failed: %v", err)
	}

	err = m.CreateJob(job)
	if err != ErrJobExists {
		t.Errorf("CreateJob() duplicate error = %v, want %v", err, ErrJobExists)
	}
}

func TestManager_CreateJob_DefaultConfig(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
	}

	err := m.CreateJob(job)
	if err != nil {
		t.Fatalf("CreateJob() failed: %v", err)
	}

	got := m.GetJob("job-1")
	if got.Config.BatchSize != 1000 {
		t.Errorf("CreateJob() default BatchSize = %v, want %v", got.Config.BatchSize, 1000)
	}
	if got.Status != StatusPending {
		t.Errorf("CreateJob() Status = %v, want %v", got.Status, StatusPending)
	}
}

func TestManager_GetJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Name:     "Test Job",
		Features: []string{"feature1"},
	}

	err := m.CreateJob(job)
	if err != nil {
		t.Fatalf("CreateJob() failed: %v", err)
	}

	// Get existing job
	got := m.GetJob("job-1")
	if got == nil {
		t.Error("GetJob() returned nil for existing job")
	}
	if got.Name != "Test Job" {
		t.Errorf("GetJob() Name = %v, want %v", got.Name, "Test Job")
	}

	// Get non-existing job
	got = m.GetJob("non-existing")
	if got != nil {
		t.Error("GetJob() should return nil for non-existing job")
	}
}

func TestManager_ListJobs(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	jobs := []*Job{
		{ID: "job-1", Features: []string{"f1"}},
		{ID: "job-2", Features: []string{"f2"}},
		{ID: "job-3", Features: []string{"f3"}},
	}

	for _, j := range jobs {
		m.CreateJob(j)
	}

	// List all
	all := m.ListJobs("")
	if len(all) != 3 {
		t.Errorf("ListJobs() count = %v, want %v", len(all), 3)
	}

	// Filter by status
	pending := m.ListJobs(StatusPending)
	if len(pending) != 3 {
		t.Errorf("ListJobs(pending) count = %v, want %v", len(pending), 3)
	}

	running := m.ListJobs(StatusRunning)
	if len(running) != 0 {
		t.Errorf("ListJobs(running) count = %v, want %v", len(running), 0)
	}
}

func TestManager_DeleteJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
	}

	m.CreateJob(job)

	err := m.DeleteJob("job-1")
	if err != nil {
		t.Fatalf("DeleteJob() failed: %v", err)
	}

	if m.GetJob("job-1") != nil {
		t.Error("GetJob() should return nil after deletion")
	}
}

func TestManager_DeleteJob_NotFound(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	err := m.DeleteJob("non-existing")
	if err != ErrJobNotFound {
		t.Errorf("DeleteJob() error = %v, want %v", err, ErrJobNotFound)
	}
}

func TestManager_DeleteJob_Running(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
		Source: DataSource{
			Type: "mock",
		},
	}

	m.CreateJob(job)
	m.StartJob(context.Background(), "job-1")

	err := m.DeleteJob("job-1")
	if err != ErrCannotDeleteRunning {
		t.Errorf("DeleteJob() error = %v, want %v", err, ErrCannotDeleteRunning)
	}

	// Cleanup
	m.CancelJob("job-1")
}

func TestManager_StartJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
		Source: DataSource{
			Type: "mock",
			Mapping: FieldMapping{
				EntityIDField:  "entity_id",
				TimestampField: "timestamp",
				FeatureFields:  map[string]string{"feature1": "value"},
			},
		},
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now(),
	}

	m.CreateJob(job)

	err := m.StartJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("StartJob() failed: %v", err)
	}

	got := m.GetJob("job-1")
	if got.Status != StatusRunning {
		t.Errorf("StartJob() Status = %v, want %v", got.Status, StatusRunning)
	}
	if got.StartedAt == nil {
		t.Error("StartJob() StartedAt should be set")
	}

	// Cleanup
	m.CancelJob("job-1")
}

func TestManager_StartJob_NotFound(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	err := m.StartJob(context.Background(), "non-existing")
	if err != ErrJobNotFound {
		t.Errorf("StartJob() error = %v, want %v", err, ErrJobNotFound)
	}
}

func TestManager_StartJob_AlreadyRunning(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
		Source:   DataSource{Type: "mock"},
	}

	m.CreateJob(job)
	m.StartJob(context.Background(), "job-1")

	err := m.StartJob(context.Background(), "job-1")
	if err != ErrJobAlreadyRunning {
		t.Errorf("StartJob() error = %v, want %v", err, ErrJobAlreadyRunning)
	}

	// Cleanup
	m.CancelJob("job-1")
}

func TestManager_PauseJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
		Source:   DataSource{Type: "mock"},
	}

	m.CreateJob(job)
	m.StartJob(context.Background(), "job-1")

	err := m.PauseJob("job-1")
	if err != nil {
		t.Fatalf("PauseJob() failed: %v", err)
	}

	got := m.GetJob("job-1")
	if got.Status != StatusPaused {
		t.Errorf("PauseJob() Status = %v, want %v", got.Status, StatusPaused)
	}
}

func TestManager_PauseJob_NotRunning(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
	}

	m.CreateJob(job)

	err := m.PauseJob("job-1")
	if err != ErrJobNotRunning {
		t.Errorf("PauseJob() error = %v, want %v", err, ErrJobNotRunning)
	}
}

func TestManager_ResumeJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
		Source:   DataSource{Type: "mock"},
	}

	m.CreateJob(job)
	m.StartJob(context.Background(), "job-1")
	m.PauseJob("job-1")

	err := m.ResumeJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ResumeJob() failed: %v", err)
	}

	got := m.GetJob("job-1")
	if got.Status != StatusRunning {
		t.Errorf("ResumeJob() Status = %v, want %v", got.Status, StatusRunning)
	}

	// Cleanup
	m.CancelJob("job-1")
}

func TestManager_ResumeJob_NotPaused(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
	}

	m.CreateJob(job)

	err := m.ResumeJob(context.Background(), "job-1")
	if err != ErrJobNotPaused {
		t.Errorf("ResumeJob() error = %v, want %v", err, ErrJobNotPaused)
	}
}

func TestManager_CancelJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Features: []string{"feature1"},
		Source:   DataSource{Type: "mock"},
	}

	m.CreateJob(job)
	m.StartJob(context.Background(), "job-1")

	err := m.CancelJob("job-1")
	if err != nil {
		t.Fatalf("CancelJob() failed: %v", err)
	}

	got := m.GetJob("job-1")
	if got.Status != StatusCancelled {
		t.Errorf("CancelJob() Status = %v, want %v", got.Status, StatusCancelled)
	}
}

func TestManager_CancelJob_NotFound(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	err := m.CancelJob("non-existing")
	if err != ErrJobNotFound {
		t.Errorf("CancelJob() error = %v, want %v", err, ErrJobNotFound)
	}
}

func TestManager_GetCheckpoint(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	// No checkpoint initially
	cp := m.GetCheckpoint("job-1")
	if cp != nil {
		t.Error("GetCheckpoint() should return nil for non-existing job")
	}
}

func TestManager_GetStats(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	jobs := []*Job{
		{ID: "job-1", Features: []string{"f1"}},
		{ID: "job-2", Features: []string{"f2"}},
		{ID: "job-3", Features: []string{"f3"}, Source: DataSource{Type: "mock"}},
	}

	for _, j := range jobs {
		m.CreateJob(j)
	}

	// Start one job
	m.StartJob(context.Background(), "job-3")

	stats := m.GetStats()
	if stats.TotalJobs != 3 {
		t.Errorf("GetStats() TotalJobs = %v, want %v", stats.TotalJobs, 3)
	}
	if stats.ByStatus[StatusPending] != 2 {
		t.Errorf("GetStats() ByStatus[pending] = %v, want %v", stats.ByStatus[StatusPending], 2)
	}
	if stats.ByStatus[StatusRunning] != 1 {
		t.Errorf("GetStats() ByStatus[running] = %v, want %v", stats.ByStatus[StatusRunning], 1)
	}

	// Cleanup
	m.CancelJob("job-3")
}

func TestManager_ExportJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	job := &Job{
		ID:       "job-1",
		Name:     "Test Job",
		Features: []string{"feature1"},
	}

	m.CreateJob(job)

	data, err := m.ExportJob("job-1")
	if err != nil {
		t.Fatalf("ExportJob() failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("ExportJob() returned empty data")
	}
}

func TestManager_ExportJob_NotFound(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	_, err := m.ExportJob("non-existing")
	if err != ErrJobNotFound {
		t.Errorf("ExportJob() error = %v, want %v", err, ErrJobNotFound)
	}
}

func TestManager_ImportJob(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	jsonData := []byte(`{"id":"imported-job","name":"Imported Job","features":["f1","f2"]}`)

	err := m.ImportJob(jsonData)
	if err != nil {
		t.Fatalf("ImportJob() failed: %v", err)
	}

	got := m.GetJob("imported-job")
	if got == nil {
		t.Error("ImportJob() job not found after import")
	}
	if got.Name != "Imported Job" {
		t.Errorf("ImportJob() Name = %v, want %v", got.Name, "Imported Job")
	}
}

func TestManager_ImportJob_Invalid(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	err := m.ImportJob([]byte("invalid json"))
	if err == nil {
		t.Error("ImportJob() should fail for invalid JSON")
	}
}

func TestDefaultJobConfig(t *testing.T) {
	cfg := DefaultJobConfig()

	if cfg.BatchSize != 1000 {
		t.Errorf("DefaultJobConfig() BatchSize = %v, want %v", cfg.BatchSize, 1000)
	}
	if cfg.Parallelism != 4 {
		t.Errorf("DefaultJobConfig() Parallelism = %v, want %v", cfg.Parallelism, 4)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("DefaultJobConfig() RetryAttempts = %v, want %v", cfg.RetryAttempts, 3)
	}
	if cfg.OnConflict != "overwrite" {
		t.Errorf("DefaultJobConfig() OnConflict = %v, want %v", cfg.OnConflict, "overwrite")
	}
}

func TestMockReader(t *testing.T) {
	reader := &mockReader{}

	for i := 0; i < 100; i++ {
		record, err := reader.Read(context.Background(), time.Now(), time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("Read() at iteration %d failed: %v", i, err)
		}
		if record == nil {
			t.Fatalf("Read() at iteration %d returned nil", i)
		}
	}

	// Should return ErrEndOfData after 100 reads
	_, err := reader.Read(context.Background(), time.Now(), time.Now().Add(time.Hour))
	if err != ErrEndOfData {
		t.Errorf("Read() after 100 iterations error = %v, want %v", err, ErrEndOfData)
	}

	err = reader.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestManager_Concurrency(t *testing.T) {
	writer := &mockWriter{}
	m := NewManager(writer)

	// Create jobs
	for i := 0; i < 10; i++ {
		job := &Job{
			ID:       "job-" + string(rune('0'+i)),
			Features: []string{"f1"},
		}
		m.CreateJob(job)
	}

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.ListJobs("")
				m.GetStats()
			}
			done <- true
		}()
	}

	// Wait
	for i := 0; i < 10; i++ {
		<-done
	}
}

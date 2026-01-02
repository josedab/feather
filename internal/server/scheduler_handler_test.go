package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/warehouse"
)

func TestSchedulerHandler_ListJobs_Empty(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/scheduler/jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	jobs := resp["jobs"].([]interface{})
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestSchedulerHandler_CreateJob(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{"job_id":"test-job","connector_name":"snowflake-1","schedule":"*/5 * * * *","max_retries":3}`
	req := httptest.NewRequest("POST", "/v1/scheduler/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["success"] != true {
		t.Error("expected success=true")
	}

	job := resp["job"].(map[string]interface{})
	if job["job_id"] != "test-job" {
		t.Errorf("expected job_id 'test-job', got %v", job["job_id"])
	}
	if job["enabled"] != true {
		t.Error("expected job to be enabled by default")
	}
}

func TestSchedulerHandler_CreateJob_InvalidSchedule(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{"job_id":"test-job","connector_name":"snowflake-1","schedule":"invalid"}`
	req := httptest.NewRequest("POST", "/v1/scheduler/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSchedulerHandler_CreateJob_MissingFields(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	tests := []struct {
		name string
		body string
	}{
		{"missing job_id", `{"connector_name":"c1","schedule":"@hourly"}`},
		{"missing connector_name", `{"job_id":"j1","schedule":"@hourly"}`},
		{"missing schedule", `{"job_id":"j1","connector_name":"c1"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/scheduler/jobs", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestSchedulerHandler_GetJob(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	scheduler.Schedule("test-job", "connector-1", "@hourly", 3)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/scheduler/jobs/test-job", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var job ScheduleJobJSON
	json.NewDecoder(w.Body).Decode(&job)

	if job.JobID != "test-job" {
		t.Errorf("expected job_id 'test-job', got %s", job.JobID)
	}
	if job.Schedule != "@hourly" {
		t.Errorf("expected schedule '@hourly', got %s", job.Schedule)
	}
}

func TestSchedulerHandler_GetJob_NotFound(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/scheduler/jobs/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSchedulerHandler_DeleteJob(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	scheduler.Schedule("test-job", "connector-1", "@hourly", 3)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/scheduler/jobs/test-job", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify job is deleted
	_, err := scheduler.GetEntry("test-job")
	if err == nil {
		t.Error("expected job to be deleted")
	}
}

func TestSchedulerHandler_DeleteJob_NotFound(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/scheduler/jobs/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSchedulerHandler_EnableDisable(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	scheduler.Schedule("test-job", "connector-1", "@hourly", 3)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Disable the job
	req := httptest.NewRequest("POST", "/v1/scheduler/jobs/test-job/disable", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	entry, _ := scheduler.GetEntry("test-job")
	if entry.Enabled {
		t.Error("expected job to be disabled")
	}

	// Enable the job
	req = httptest.NewRequest("POST", "/v1/scheduler/jobs/test-job/enable", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	entry, _ = scheduler.GetEntry("test-job")
	if !entry.Enabled {
		t.Error("expected job to be enabled")
	}
}

func TestSchedulerHandler_TriggerJob(t *testing.T) {
	// This test only verifies that the handler returns the expected response.
	// We skip the actual trigger because the sync engine would be nil.
	// The TriggerNow functionality is tested via scheduler package tests.
	t.Skip("Skipping TriggerJob test - requires real sync engine")
}

func TestSchedulerHandler_TriggerJob_NotFound(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/scheduler/jobs/nonexistent/trigger", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSchedulerHandler_GetStatus(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	scheduler.Schedule("job-1", "connector-1", "@hourly", 3)
	scheduler.Schedule("job-2", "connector-2", "@daily", 3)
	scheduler.Disable("job-2")
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/scheduler/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["total_jobs"].(float64) != 2 {
		t.Errorf("expected 2 total jobs, got %v", resp["total_jobs"])
	}
	if resp["enabled_jobs"].(float64) != 1 {
		t.Errorf("expected 1 enabled job, got %v", resp["enabled_jobs"])
	}
	if resp["disabled_jobs"].(float64) != 1 {
		t.Errorf("expected 1 disabled job, got %v", resp["disabled_jobs"])
	}
}

func TestSchedulerHandler_StartStop(t *testing.T) {
	// Note: Starting the scheduler will run a background goroutine that checks jobs
	// even with nil sync engine, but it only panics when a job is actually executed.
	// Since we don't add any jobs here, start/stop should work fine.
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Start scheduler
	req := httptest.NewRequest("POST", "/v1/scheduler/start", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !scheduler.IsRunning() {
		t.Error("expected scheduler to be running")
	}

	// Stop scheduler immediately before any jobs could run
	req = httptest.NewRequest("POST", "/v1/scheduler/stop", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if scheduler.IsRunning() {
		t.Error("expected scheduler to be stopped")
	}
}

func TestSchedulerHandler_StartTwice(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Start scheduler first time
	req := httptest.NewRequest("POST", "/v1/scheduler/start", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Start scheduler second time (should fail)
	req = httptest.NewRequest("POST", "/v1/scheduler/start", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}

	// Clean up
	scheduler.Stop()
}

func TestSchedulerHandler_NilScheduler(t *testing.T) {
	handler := NewSchedulerHandler(nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/scheduler/jobs"},
		{"GET", "/v1/scheduler/jobs/test"},
		{"POST", "/v1/scheduler/jobs"},
		{"DELETE", "/v1/scheduler/jobs/test"},
		{"POST", "/v1/scheduler/jobs/test/enable"},
		{"POST", "/v1/scheduler/jobs/test/disable"},
		{"POST", "/v1/scheduler/jobs/test/trigger"},
		{"GET", "/v1/scheduler/status"},
		{"POST", "/v1/scheduler/start"},
		{"POST", "/v1/scheduler/stop"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.method == "POST" && ep.path == "/v1/scheduler/jobs" {
				req = httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{}`))
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
			}
		})
	}
}

func TestSchedulerHandler_ListJobs_WithJobs(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	scheduler.Schedule("job-1", "connector-1", "*/5 * * * *", 3)
	scheduler.Schedule("job-2", "connector-2", "@hourly", 2)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/scheduler/jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	jobs := resp["jobs"].([]interface{})
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
	if resp["count"].(float64) != 2 {
		t.Errorf("expected count 2, got %v", resp["count"])
	}
}

func TestSchedulerHandler_SpecialSchedules(t *testing.T) {
	scheduler := warehouse.NewCronScheduler(nil, nil)
	handler := NewSchedulerHandler(scheduler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	schedules := []string{
		"@yearly",
		"@monthly",
		"@weekly",
		"@daily",
		"@hourly",
		"@every 1h",
		"@every 30m",
	}

	for i, sched := range schedules {
		t.Run(sched, func(t *testing.T) {
			body := map[string]interface{}{
				"job_id":         "job-" + string(rune('A'+i)),
				"connector_name": "connector-1",
				"schedule":       sched,
			}
			bodyBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/v1/scheduler/jobs", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status %d for schedule %s, got %d: %s", http.StatusCreated, sched, w.Code, w.Body.String())
			}
		})
	}
}

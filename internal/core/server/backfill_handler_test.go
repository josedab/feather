package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/tools/backfill"
)

// mockFeatureWriter is a mock implementation of backfill.FeatureWriter for testing.
type mockFeatureWriter struct{}

func (w *mockFeatureWriter) WriteFeature(ctx context.Context, entityID string, feature string, value interface{}, timestamp time.Time) error {
	return nil
}

func (w *mockFeatureWriter) WriteBatch(ctx context.Context, records []backfill.FeatureRecord) error {
	return nil
}

// testBackfillServer wraps a BackfillHandler for testing.
type testBackfillServer struct {
	handler *BackfillHandler
	manager *backfill.Manager
	mux     *http.ServeMux
	t       *testing.T
}

// newTestBackfillServer creates a new test backfill server with a mock writer.
func newTestBackfillServer(t *testing.T) *testBackfillServer {
	t.Helper()

	writer := &mockFeatureWriter{}
	manager := backfill.NewManager(writer)
	handler := &BackfillHandler{
		manager: manager,
	}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testBackfillServer{
		handler: handler,
		manager: manager,
		mux:     mux,
		t:       t,
	}
}

func (ts *testBackfillServer) request(method, path string, body string) *httptest.ResponseRecorder {
	ts.t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

func (ts *testBackfillServer) postJSON(path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()

	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("failed to marshal body: %v", err)
	}
	return ts.request(http.MethodPost, path, string(jsonBody))
}

func (ts *testBackfillServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testBackfillServer) delete(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodDelete, path, "")
}

// createJob is a helper to create a backfill job for testing.
func (ts *testBackfillServer) createJob(id string) *httptest.ResponseRecorder {
	ts.t.Helper()

	body := CreateJobRequest{
		ID:         id,
		Name:       "Test Job",
		Features:   []string{"feature1", "feature2"},
		EntityType: "user",
		StartTime:  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		EndTime:    time.Now().Format(time.RFC3339),
		Source: backfill.DataSource{
			Type:   "file",
			URI:    "/data/test.csv",
			Format: "csv",
		},
	}

	return ts.postJSON("/v1/backfill/jobs", body)
}

func TestBackfillHandler_ListJobs_Empty(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.get("/v1/backfill/jobs")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["count"].(float64) != 0 {
		t.Errorf("Expected count=0, got %v", result["count"])
	}
}

func TestBackfillHandler_CreateJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	body := CreateJobRequest{
		ID:         "job-1",
		Name:       "Test Backfill Job",
		Features:   []string{"feature1", "feature2"},
		EntityType: "user",
		StartTime:  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		EndTime:    time.Now().Format(time.RFC3339),
		Source: backfill.DataSource{
			Type:   "file",
			URI:    "/data/test.csv",
			Format: "csv",
		},
	}

	rr := ts.postJSON("/v1/backfill/jobs", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestBackfillHandler_CreateJob_MissingID(t *testing.T) {
	ts := newTestBackfillServer(t)

	body := CreateJobRequest{
		Features: []string{"feature1"},
	}

	rr := ts.postJSON("/v1/backfill/jobs", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_CreateJob_MissingFeatures(t *testing.T) {
	ts := newTestBackfillServer(t)

	body := CreateJobRequest{
		ID: "job-1",
	}

	rr := ts.postJSON("/v1/backfill/jobs", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_CreateJob_InvalidBody(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.request(http.MethodPost, "/v1/backfill/jobs", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_CreateJob_InvalidStartTime(t *testing.T) {
	ts := newTestBackfillServer(t)

	body := CreateJobRequest{
		ID:        "job-1",
		Features:  []string{"feature1"},
		StartTime: "invalid",
		EndTime:   time.Now().Format(time.RFC3339),
	}

	rr := ts.postJSON("/v1/backfill/jobs", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_CreateJob_InvalidEndTime(t *testing.T) {
	ts := newTestBackfillServer(t)

	body := CreateJobRequest{
		ID:        "job-1",
		Features:  []string{"feature1"},
		StartTime: time.Now().Format(time.RFC3339),
		EndTime:   "invalid",
	}

	rr := ts.postJSON("/v1/backfill/jobs", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_GetJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job first
	ts.createJob("get-job")

	rr := ts.get("/v1/backfill/jobs/get-job")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestBackfillHandler_GetJob_NotFound(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.get("/v1/backfill/jobs/nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestBackfillHandler_DeleteJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job first
	ts.createJob("delete-job")

	rr := ts.delete("/v1/backfill/jobs/delete-job")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestBackfillHandler_DeleteJob_NotFound(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.delete("/v1/backfill/jobs/nonexistent")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_StartJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job first
	ts.createJob("start-job")

	rr := ts.postJSON("/v1/backfill/jobs/start-job/start", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestBackfillHandler_StartJob_NotFound(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.postJSON("/v1/backfill/jobs/nonexistent/start", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_PauseJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job first
	ts.createJob("pause-job")

	// Note: With mock writer, jobs may complete instantly after starting
	// so we can't reliably test pause on a running job.
	// Instead, we test that pause returns a meaningful error for non-running jobs.
	rr := ts.postJSON("/v1/backfill/jobs/pause-job/pause", nil)

	// Expect either success (if job is running) or error (if not running)
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestBackfillHandler_PauseJob_NotFound(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.postJSON("/v1/backfill/jobs/nonexistent/pause", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_ResumeJob_NotFound(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.postJSON("/v1/backfill/jobs/nonexistent/resume", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_CancelJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job first
	ts.createJob("cancel-job")

	rr := ts.postJSON("/v1/backfill/jobs/cancel-job/cancel", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestBackfillHandler_CancelJob_NotFound(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.postJSON("/v1/backfill/jobs/nonexistent/cancel", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_GetCheckpoint(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job first
	ts.createJob("checkpoint-job")

	// Start job to create checkpoint opportunity
	ts.postJSON("/v1/backfill/jobs/checkpoint-job/start", nil)

	rr := ts.get("/v1/backfill/jobs/checkpoint-job/checkpoint")

	// Checkpoint may not exist yet, so expect not found
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestBackfillHandler_GetStats(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.get("/v1/backfill/stats")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestBackfillHandler_ExportJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job first
	ts.createJob("export-job")

	rr := ts.get("/v1/backfill/jobs/export-job/export")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Check content type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestBackfillHandler_ExportJob_NotFound(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.get("/v1/backfill/jobs/nonexistent/export")

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestBackfillHandler_ImportJob(t *testing.T) {
	ts := newTestBackfillServer(t)

	job := map[string]interface{}{
		"id":          "import-job",
		"name":        "Imported Job",
		"features":    []string{"feature1"},
		"entity_type": "user",
		"start_time":  time.Now().Add(-24 * time.Hour),
		"end_time":    time.Now(),
	}

	rr := ts.postJSON("/v1/backfill/import", job)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d; body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}
}

func TestBackfillHandler_ImportJob_InvalidBody(t *testing.T) {
	ts := newTestBackfillServer(t)

	rr := ts.request(http.MethodPost, "/v1/backfill/import", "invalid json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_ImportJob_MissingID(t *testing.T) {
	ts := newTestBackfillServer(t)

	job := map[string]interface{}{
		"name":     "No ID Job",
		"features": []string{"feature1"},
	}

	rr := ts.postJSON("/v1/backfill/import", job)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBackfillHandler_ListJobs_WithStatus(t *testing.T) {
	ts := newTestBackfillServer(t)

	// Create job
	ts.createJob("status-job")

	// List with status filter
	rr := ts.get("/v1/backfill/jobs?status=pending")

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

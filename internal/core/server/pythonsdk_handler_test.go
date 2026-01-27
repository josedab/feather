package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/pythonsdk"
)

func newTestPythonSDKHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	registry := pythonsdk.NewRegistry(pythonsdk.DefaultRegistryConfig())
	handler := NewPythonSDKHandler(registry)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestPythonSDKHandler_Register(t *testing.T) {
	mux := newTestPythonSDKHandler(t)

	body := `{"id":"t1","name":"Age Bucket","source_code":"def transform(age): return age//10*10","entry_point":"transform","inputs":[{"name":"age","dtype":"int64"}],"outputs":[{"name":"age_bucket","dtype":"int64"}]}`
	req := httptest.NewRequest("POST", "/v1/transforms", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("POST transform = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestPythonSDKHandler_List(t *testing.T) {
	mux := newTestPythonSDKHandler(t)

	req := httptest.NewRequest("GET", "/v1/transforms", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET transforms = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPythonSDKHandler_NotFound(t *testing.T) {
	mux := newTestPythonSDKHandler(t)

	req := httptest.NewRequest("GET", "/v1/transforms/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET nonexistent = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestPythonSDKHandler_Execute(t *testing.T) {
	mux := newTestPythonSDKHandler(t)

	// Register first
	body := `{"id":"t1","name":"T1","source_code":"pass","outputs":[{"name":"result","dtype":"float64"}]}`
	req := httptest.NewRequest("POST", "/v1/transforms", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Execute
	body = `{"input": 42}`
	req = httptest.NewRequest("POST", "/v1/transforms/t1/execute", strings.NewReader(body))
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST execute = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestPythonSDKHandler_Validate(t *testing.T) {
	mux := newTestPythonSDKHandler(t)

	body := `{"id":"t1","name":"T1","source_code":"pass"}`
	req := httptest.NewRequest("POST", "/v1/transforms/validate", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("POST validate = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPythonSDKHandler_Stats(t *testing.T) {
	mux := newTestPythonSDKHandler(t)

	req := httptest.NewRequest("GET", "/v1/transforms/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET stats = %d, want %d", rr.Code, http.StatusOK)
	}
}

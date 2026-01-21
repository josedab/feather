package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/skewdetect"
)

func setupSkewDetectHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewSkewDetectHandler(skewdetect.NewDetector(skewdetect.DefaultDetectorConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestSkewDetect_GetStats(t *testing.T) {
	mux := setupSkewDetectHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/skew/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSkewDetect_PostRegisterFeature(t *testing.T) {
	mux := setupSkewDetectHandler(t)
	body := `{"name":"feature1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/skew/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSkewDetect_PostInvalidJSON(t *testing.T) {
	mux := setupSkewDetectHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/skew/features", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

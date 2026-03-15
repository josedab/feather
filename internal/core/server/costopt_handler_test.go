package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/costopt"
)

func setupCostOptHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	analyzer := costopt.NewAnalyzer(costopt.DefaultAnalyzerConfig())
	recommender := costopt.NewRecommender(analyzer, costopt.DefaultRecommenderConfig())
	forecaster := costopt.NewForecaster(costopt.DefaultForecastConfig())
	handler := NewCostOptHandler(analyzer, recommender, forecaster)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestCostOpt_GetStats(t *testing.T) {
	mux := setupCostOptHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/costopt/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCostOpt_PostRecordAccess(t *testing.T) {
	mux := setupCostOptHandler(t)
	body := `{"feature_group":"user_features","entity":"click_count","is_write":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/costopt/access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCostOpt_PostInvalidJSON(t *testing.T) {
	mux := setupCostOptHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/costopt/access", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

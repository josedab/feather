package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/qualityscore"
)

func setupQualityScoreHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	handler := NewQualityScoreHandler(qualityscore.NewScorer(qualityscore.DefaultScoringConfig()))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestQualityScore_GetStats(t *testing.T) {
	mux := setupQualityScoreHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/quality-score/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestQualityScore_PostRecordSignal(t *testing.T) {
	mux := setupQualityScoreHandler(t)
	body := `{"feature_id":"feat1","type":"freshness","value":0.9,"weight":1.0}`
	req := httptest.NewRequest(http.MethodPost, "/v1/quality-score/signal", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestQualityScore_PostInvalidJSON(t *testing.T) {
	mux := setupQualityScoreHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/quality-score/signal", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
